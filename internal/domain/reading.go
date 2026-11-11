package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Reading es una lectura enviada por un dispositivo IoT.
type Reading struct {
	ID         string
	DeviceID   string
	Metric     MetricType
	Value      float64
	Unit       string
	ReceivedAt time.Time
	Status     ReadingStatus

	// Datos opcionales que agrega el Edge.
	MeasuredAt *time.Time
	LocalState LocalState
}

// ReadingDetails agrupa los campos opcionales del contrato del Edge.
type ReadingDetails struct {
	// ID es el readingId del Edge; permite descartar reenvíos (QoS 1). Vacío genera uno.
	ID         string
	MeasuredAt *time.Time
	LocalState LocalState
}

func NewReading(deviceID string, metric MetricType, value float64, unit string, details ReadingDetails, now time.Time) (*Reading, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("%w: deviceId is required", ErrInvalid)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("%w: value must be a finite number", ErrInvalid)
	}
	if strings.TrimSpace(unit) == "" {
		return nil, fmt.Errorf("%w: unit is required", ErrInvalid)
	}
	id := details.ID
	if id == "" {
		id = NewID()
	} else if err := ValidateID("readingId", id); err != nil {
		return nil, err
	}
	return &Reading{
		ID:         id,
		DeviceID:   deviceID,
		Metric:     metric,
		Value:      value,
		Unit:       unit,
		ReceivedAt: now,
		Status:     ReadingReceived,
		MeasuredAt: details.MeasuredAt,
		LocalState: details.LocalState,
	}, nil
}

func (r *Reading) MarkValidated()  { r.Status = ReadingValidated }
func (r *Reading) MarkOutOfRange() { r.Status = ReadingOutOfRange }
