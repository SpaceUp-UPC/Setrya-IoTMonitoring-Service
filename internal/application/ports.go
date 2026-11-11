package application

import (
	"context"
	"encoding/json"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

// Notifier envía una alerta al usuario responsable (implementado en
// infraestructura por NotificationDispatcherService).
type Notifier interface {
	Notify(ctx context.Context, alert *domain.Alert, device *domain.IoTDevice) error
}

// ReadingNormalizer limpia y estandariza una lectura antes de llegar al
// dominio (implementado por ReadingNormalizerService).
type ReadingNormalizer interface {
	Normalize(cmd IngestReadingCommand) (IngestReadingCommand, error)
}

// EdgeCommand es la orden que recibe el Edge en el tópico commands.
type EdgeCommand struct {
	CommandID string         `json:"commandId"`
	Action    string         `json:"action"`
	Params    map[string]any `json:"params"`
}

// EdgePublisher publica mensajes de la nube hacia el Edge (implementado por
// IoTBrokerAdapter). Devuelve domain.ErrBrokerUnavailable si no hay broker.
type EdgePublisher interface {
	PublishCommand(ctx context.Context, spaceID, target string, cmd EdgeCommand) error
	PublishValidation(ctx context.Context, spaceID, eventID, decision string) error
	PublishConfig(ctx context.Context, spaceID string, config json.RawMessage) error
}
