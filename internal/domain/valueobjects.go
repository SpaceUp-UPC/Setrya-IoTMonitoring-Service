package domain

import (
	"fmt"
	"math"
)

// MetricType identifica la magnitud que mide un dispositivo.
//
// TODO(faltante): el informe no lista los tipos de métrica soportados; se
// asumen los tres que aparecen en las épicas (térmico, eléctrico, seguridad
// acústica). Confirmar con el equipo de Edge/Embedded.
type MetricType string

const (
	MetricTemperature MetricType = "temperature"
	MetricVoltage     MetricType = "voltage"
	MetricSound       MetricType = "sound"
)

func ParseMetricType(s string) (MetricType, error) {
	switch m := MetricType(s); m {
	case MetricTemperature, MetricVoltage, MetricSound:
		return m, nil
	}
	return "", fmt.Errorf("%w: unknown metric type %q", ErrInvalid, s)
}

// ThresholdRange define el rango permitido para una métrica monitoreada.
type ThresholdRange struct {
	MinValue float64
	MaxValue float64
	Unit     string
}

func NewThresholdRange(min, max float64, unit string) (ThresholdRange, error) {
	if math.IsNaN(min) || math.IsNaN(max) || math.IsInf(min, 0) || math.IsInf(max, 0) {
		return ThresholdRange{}, fmt.Errorf("%w: threshold must be finite", ErrInvalid)
	}
	if min >= max {
		return ThresholdRange{}, fmt.Errorf("%w: min must be lower than max", ErrInvalid)
	}
	return ThresholdRange{MinValue: min, MaxValue: max, Unit: unit}, nil
}

func (t ThresholdRange) Contains(v float64) bool {
	return v >= t.MinValue && v <= t.MaxValue
}

// Deviation devuelve cuánto se aleja v del rango, como fracción de su amplitud.
// Es 0 cuando v está dentro del rango.
func (t ThresholdRange) Deviation(v float64) float64 {
	var distance float64
	switch {
	case v < t.MinValue:
		distance = t.MinValue - v
	case v > t.MaxValue:
		distance = v - t.MaxValue
	default:
		return 0
	}
	return distance / (t.MaxValue - t.MinValue)
}

// SeverityLevel representa el nivel de gravedad de una alerta.
type SeverityLevel string

const (
	SeverityLow      SeverityLevel = "low"
	SeverityMedium   SeverityLevel = "medium"
	SeverityHigh     SeverityLevel = "high"
	SeverityCritical SeverityLevel = "critical"
)

// ReadingStatus indica el estado de una lectura.
type ReadingStatus string

const (
	ReadingReceived   ReadingStatus = "received"
	ReadingValidated  ReadingStatus = "validated"
	ReadingOutOfRange ReadingStatus = "out_of_range"
)

// AlertStatus indica el estado de una alerta.
type AlertStatus string

const (
	AlertCreated      AlertStatus = "created"
	AlertSent         AlertStatus = "sent"
	AlertAcknowledged AlertStatus = "acknowledged"
	AlertClosed       AlertStatus = "closed"
)

func ParseAlertStatus(s string) (AlertStatus, error) {
	switch a := AlertStatus(s); a {
	case AlertCreated, AlertSent, AlertAcknowledged, AlertClosed:
		return a, nil
	}
	return "", fmt.Errorf("%w: unknown alert status %q", ErrInvalid, s)
}
