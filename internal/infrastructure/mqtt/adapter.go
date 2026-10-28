package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/application"
)

// Ingestor es el caso de uso al que el adaptador delega cada lectura.
type Ingestor interface {
	Handle(ctx context.Context, cmd application.IngestReadingCommand) (application.IngestResult, error)
}

type Config struct {
	BrokerURL string
	ClientID  string
	Username  string
	Password  string
	// Topic al que se suscribe el servicio.
	//
	// TODO(faltante): el formato del topic y del payload no está definido por
	// el equipo de Edge/Embedded. Se asume un topic con wildcards y un payload
	// JSON {"deviceId","metricType","value","unit"}.
	Topic string
}

// IoTBrokerAdapter recibe las lecturas publicadas por la Edge Application en
// el broker MQTT y las transforma en comandos para la capa de aplicación.
type IoTBrokerAdapter struct {
	cfg      Config
	ingestor Ingestor
	log      *slog.Logger
	client   paho.Client
}

func NewAdapter(cfg Config, ingestor Ingestor, log *slog.Logger) *IoTBrokerAdapter {
	return &IoTBrokerAdapter{cfg: cfg, ingestor: ingestor, log: log}
}

type payload struct {
	DeviceID   string   `json:"deviceId"`
	MetricType string   `json:"metricType"`
	Value      *float64 `json:"value"`
	Unit       string   `json:"unit"`
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
	return application.IngestReadingCommand{
		DeviceID: p.DeviceID, MetricType: p.MetricType, Value: *p.Value, Unit: p.Unit,
	}, nil
}

// Start conecta al broker y se suscribe al topic. Se reconecta
// automáticamente y re-suscribe en cada conexión.
func (a *IoTBrokerAdapter) Start() error {
	opts := paho.NewClientOptions().
		AddBroker(a.cfg.BrokerURL).
		SetClientID(a.cfg.ClientID).
		SetUsername(a.cfg.Username).
		SetPassword(a.cfg.Password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetOnConnectHandler(func(c paho.Client) {
			tok := c.Subscribe(a.cfg.Topic, 1, a.onMessage)
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
