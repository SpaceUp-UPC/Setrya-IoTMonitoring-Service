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

// NewIoTDevice registra un dispositivo, inactivo hasta que se active.
//
// id es opcional: el Edge publica en nombre de las ESP32 y necesita conocer
// el deviceId de antemano, así que se aceptan ids acordados (por ejemplo
// "space1-temperature"). Con id vacío se genera un UUID.
//
// TODO(faltante): falta validar que SpaceID exista en Space Management, cuyo
// servicio todavía no está publicado.
func NewIoTDevice(id, spaceID, name string, thresholds map[MetricType]ThresholdRange, now time.Time) (*IoTDevice, error) {
	if id == "" {
		id = NewID()
	} else if err := ValidateID("id", id); err != nil {
		return nil, err
	}
	if err := ValidateID("spaceId", spaceID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if thresholds == nil {
		thresholds = map[MetricType]ThresholdRange{}
	}
	return &IoTDevice{
		ID:         id,
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
