package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

// Ingestor es el caso de uso al que el adaptador delega cada lectura.
type Ingestor interface {
	Handle(ctx context.Context, cmd application.IngestReadingCommand) (application.IngestResult, error)
}

type Config struct {
	// BrokerURL admite tcp://host:1883 y, con TLS, ssl://host:8883.
	BrokerURL string
	ClientID  string
	Username  string
	Password  string
	// Topic al que se suscribe el servicio. El Edge publica en
	// sentrya/{spaceId}/{deviceId}/readings.
	Topic string
}

const (
	qos            = 1
	publishTimeout = 5 * time.Second
)

// IoTBrokerAdapter conecta el servicio con el broker MQTT: recibe las
// lecturas que publica la Edge Application y publica hacia ella las órdenes,
// validaciones y configuraciones.
type IoTBrokerAdapter struct {
	cfg      Config
	ingestor Ingestor
	log      *slog.Logger
	client   paho.Client
}

func NewAdapter(cfg Config, ingestor Ingestor, log *slog.Logger) *IoTBrokerAdapter {
	return &IoTBrokerAdapter{cfg: cfg, ingestor: ingestor, log: log}
}

// payload es el mensaje del tópico readings (sentrya-edge-service, AGENTS.md §9).
type payload struct {
	DeviceID   string   `json:"deviceId"`
	MetricType string   `json:"metricType"`
	Value      *float64 `json:"value"`
	Unit       string   `json:"unit"`

	ReadingID   string `json:"readingId"`
	MeasuredAt  string `json:"measuredAt"`
	LocalState  string `json:"localState"`
	Reason      string `json:"reason"`
	EventID     string `json:"eventId"`
	EvidenceRef string `json:"evidenceRef"`
}

// ParsePayload convierte el mensaje del broker en un comando de ingesta.
func ParsePayload(raw []byte) (application.IngestReadingCommand, error) {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return application.IngestReadingCommand{}, fmt.Errorf("invalid reading payload: %w", err)
	}
	if p.Value == nil {
		return application.IngestReadingCommand{}, fmt.Errorf("invalid reading payload: value is required")
	}
	cmd := application.IngestReadingCommand{
		DeviceID: p.DeviceID, MetricType: p.MetricType, Value: *p.Value, Unit: p.Unit,
		ReadingID: p.ReadingID, LocalState: p.LocalState,
		Reason: p.Reason, EventID: p.EventID, EvidenceRef: p.EvidenceRef,
	}
	// Una fecha mal formada no descarta la lectura: queda sin measuredAt.
	if t, err := time.Parse(time.RFC3339, p.MeasuredAt); err == nil {
		t = t.UTC()
		cmd.MeasuredAt = &t
	}
	return cmd, nil
}

// Start conecta al broker y se suscribe al topic. Se reconecta
// automáticamente y re-suscribe en cada conexión.
//
// La sesión es persistente (CleanSession false con ClientID fijo): si el
// servicio se reinicia o se suspende, el broker conserva las lecturas QoS 1 y
// las entrega al reconectar.
func (a *IoTBrokerAdapter) Start() error {
	opts := paho.NewClientOptions().
		AddBroker(a.cfg.BrokerURL).
		SetClientID(a.cfg.ClientID).
		SetUsername(a.cfg.Username).
		SetPassword(a.cfg.Password).
		SetCleanSession(false).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		// Los mensajes retenidos por la sesión llegan antes de re-suscribirse.
		SetDefaultPublishHandler(a.onMessage).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			a.log.Warn("mqtt connection lost", "error", err)
		}).
		SetOnConnectHandler(func(c paho.Client) {
			tok := c.Subscribe(a.cfg.Topic, qos, a.onMessage)
			tok.Wait()
			if err := tok.Error(); err != nil {
				a.log.Error("mqtt subscribe failed", "topic", a.cfg.Topic, "error", err)
				return
			}
			a.log.Info("mqtt subscribed", "topic", a.cfg.Topic)
		})
	a.client = paho.NewClient(opts)
	// Con ConnectRetry el token no se completa hasta conectar; no bloquea el arranque.
	a.client.Connect()
	return nil
}

func (a *IoTBrokerAdapter) Stop() {
	if a.client != nil {
		a.client.Disconnect(250)
	}
}

func (a *IoTBrokerAdapter) onMessage(_ paho.Client, msg paho.Message) {
	cmd, err := ParsePayload(msg.Payload())
	if err != nil {
		a.log.Warn("mqtt message discarded", "topic", msg.Topic(), "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := a.ingestor.Handle(ctx, cmd); err != nil {
		a.log.Warn("mqtt reading rejected", "topic", msg.Topic(), "deviceId", cmd.DeviceID, "error", err)
	}
}

// Tópicos nube -> Edge (sentrya-edge-service, AGENTS.md §9).

func CommandTopic(spaceID, target string) string {
	return fmt.Sprintf("sentrya/%s/%s/commands", spaceID, target)
}

func ValidationTopic(spaceID string) string {
	return fmt.Sprintf("sentrya/%s/security/validation", spaceID)
}

func ConfigTopic(spaceID string) string { return fmt.Sprintf("sentrya/%s/config", spaceID) }

func (a *IoTBrokerAdapter) PublishCommand(ctx context.Context, spaceID, target string, cmd application.EdgeCommand) error {
	return a.publishJSON(ctx, CommandTopic(spaceID, target), cmd)
}

func (a *IoTBrokerAdapter) PublishValidation(ctx context.Context, spaceID, eventID, decision string) error {
	return a.publishJSON(ctx, ValidationTopic(spaceID), map[string]string{"eventId": eventID, "decision": decision})
}

// PublishConfig no usa el flag retain: el Edge ignora la config retenida
// porque su YAML manda en cada arranque.
func (a *IoTBrokerAdapter) PublishConfig(ctx context.Context, spaceID string, config json.RawMessage) error {
	return a.publish(ctx, ConfigTopic(spaceID), config)
}

func (a *IoTBrokerAdapter) publishJSON(ctx context.Context, topic string, body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return a.publish(ctx, topic, raw)
}

// publish espera el acuse del broker; sin conexión falla de inmediato en vez
// de encolar una orden que llegaría tarde.
func (a *IoTBrokerAdapter) publish(ctx context.Context, topic string, raw []byte) error {
	if a.client == nil || !a.client.IsConnectionOpen() {
		return domain.ErrBrokerUnavailable
	}
	tok := a.client.Publish(topic, qos, false, raw)
	select {
	case <-tok.Done():
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", domain.ErrBrokerUnavailable, ctx.Err())
	case <-time.After(publishTimeout):
		return fmt.Errorf("%w: publish timed out", domain.ErrBrokerUnavailable)
	}
	if err := tok.Error(); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrBrokerUnavailable, err)
	}
	return nil
}

// DisabledPublisher se usa cuando el servicio corre sin broker MQTT.
type DisabledPublisher struct{}

func (DisabledPublisher) PublishCommand(context.Context, string, string, application.EdgeCommand) error {
	return domain.ErrBrokerUnavailable
}

func (DisabledPublisher) PublishValidation(context.Context, string, string, string) error {
	return domain.ErrBrokerUnavailable
}

func (DisabledPublisher) PublishConfig(context.Context, string, json.RawMessage) error {
	return domain.ErrBrokerUnavailable
}
