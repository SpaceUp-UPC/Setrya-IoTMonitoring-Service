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
}

func NewReading(deviceID string, metric MetricType, value float64, unit string, now time.Time) (*Reading, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("%w: deviceId is required", ErrInvalid)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("%w: value must be a finite number", ErrInvalid)
	}
	if strings.TrimSpace(unit) == "" {
		return nil, fmt.Errorf("%w: unit is required", ErrInvalid)
	}
	return &Reading{
		ID:         NewID(),
		DeviceID:   deviceID,
		Metric:     metric,
		Value:      value,
		Unit:       unit,
		ReceivedAt: now,
		Status:     ReadingReceived,
	}, nil
}

func (r *Reading) MarkValidated()  { r.Status = ReadingValidated }
func (r *Reading) MarkOutOfRange() { r.Status = ReadingOutOfRange }
