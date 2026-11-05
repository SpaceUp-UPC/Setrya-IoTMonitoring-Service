package domain

import (
	"fmt"
	"strings"
	"time"
)

// IoTDevice es el aggregate root: un dispositivo asociado a un espacio que
// emite lecturas para el monitoreo.
type IoTDevice struct {
	ID         string
	SpaceID    string
	Name       string
	Active     bool
	Thresholds map[MetricType]ThresholdRange
	CreatedAt  time.Time
}

// NewIoTDevice registra un dispositivo. Nace inactivo: el informe indica que
// la activación del servicio depende de Payment Management.
//
// TODO(faltante): definir quién dispara la activación (Payments o el
// implementador) y si el dispositivo debe validar que SpaceID exista en
// Space Management.
func NewIoTDevice(spaceID, name string, thresholds map[MetricType]ThresholdRange, now time.Time) (*IoTDevice, error) {
	if strings.TrimSpace(spaceID) == "" {
		return nil, fmt.Errorf("%w: spaceId is required", ErrInvalid)
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if thresholds == nil {
		thresholds = map[MetricType]ThresholdRange{}
	}
	return &IoTDevice{
		ID:         NewID(),
		SpaceID:    spaceID,
		Name:       name,
		Thresholds: thresholds,
		CreatedAt:  now,
	}, nil
}

func (d *IoTDevice) Activate()      { d.Active = true }
func (d *IoTDevice) Deactivate()    { d.Active = false }
func (d *IoTDevice) IsActive() bool { return d.Active }

// ThresholdFor devuelve el umbral configurado para la métrica, si existe.
func (d *IoTDevice) ThresholdFor(m MetricType) (ThresholdRange, bool) {
	t, ok := d.Thresholds[m]
	return t, ok
}
