package domain

import "context"

// IIoTDeviceRepository persiste y recupera dispositivos IoT registrados.
type IIoTDeviceRepository interface {
	Save(ctx context.Context, device *IoTDevice) error
	FindByID(ctx context.Context, id string) (*IoTDevice, error)
	FindBySpace(ctx context.Context, spaceID string) ([]*IoTDevice, error)
}

// IReadingRepository almacena y consulta lecturas recibidas.
type IReadingRepository interface {
	Save(ctx context.Context, reading *Reading) error
	// FindByID devuelve ErrReadingNotFound si la lectura no existe.
	FindByID(ctx context.Context, id string) (*Reading, error)
	// FindBySpace devuelve las lecturas más recientes primero.
	FindBySpace(ctx context.Context, spaceID string, limit int) ([]*Reading, error)
}

// IAlertRepository gestiona el almacenamiento y consulta de alertas.
type IAlertRepository interface {
	Save(ctx context.Context, alert *Alert) error
	FindByID(ctx context.Context, id string) (*Alert, error)
	// FindBySpace devuelve las alertas más recientes primero; status nil no filtra.
	FindBySpace(ctx context.Context, spaceID string, status *AlertStatus) ([]*Alert, error)
	// FindOpenByDevice devuelve las alertas no cerradas del dispositivo y la métrica.
	FindOpenByDevice(ctx context.Context, deviceID string, metric MetricType) ([]*Alert, error)
	// FindByReading devuelve la alerta que originó la lectura, o nil si no hubo.
	FindByReading(ctx context.Context, readingID string) (*Alert, error)
}
