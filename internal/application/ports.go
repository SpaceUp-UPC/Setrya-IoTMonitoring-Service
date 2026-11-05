package application

import (
	"context"

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
