package domain

import (
	"fmt"
	"math"
	"regexp"
)

// MetricType identifica la magnitud o el evento que reporta un dispositivo.
// Los tipos siguen el contrato MQTT del Edge (sentrya-edge-service, AGENTS.md §9).
type MetricType string

const (
	MetricTemperature MetricType = "temperature"
	MetricHumidity    MetricType = "humidity"
	MetricVoltage     MetricType = "voltage"
	MetricSound       MetricType = "sound"

	// "Lecturas evento": llegan con value 1 y unit "event".
	MetricIntrusion   MetricType = "intrusion"
	MetricEscalation  MetricType = "escalation"
	MetricDeviceError MetricType = "device_error"
)

func ParseMetricType(s string) (MetricType, error) {
	switch m := MetricType(s); m {
	case MetricTemperature, MetricHumidity, MetricVoltage, MetricSound,
		MetricIntrusion, MetricEscalation, MetricDeviceError:
		return m, nil
	}
	return "", fmt.Errorf("%w: unknown metric type %q", ErrInvalid, s)
}

// IsEvent indica si el tipo es una "lectura evento" y no una magnitud medida.
func (m MetricType) IsEvent() bool {
	return m == MetricIntrusion || m == MetricEscalation || m == MetricDeviceError
}

// EventUnit es la unidad con la que el Edge publica las lecturas evento.
const EventUnit = "event"

// LocalState es el estado que el Edge calculó localmente para la lectura.
type LocalState string

const (
	LocalStateNone     LocalState = ""
	LocalStateNormal   LocalState = "NORMAL"
	LocalStateWarning  LocalState = "WARNING"
	LocalStateCritical LocalState = "CRITICAL"
)

// IsAbnormal indica si el Edge ya considera la lectura fuera de lo normal.
func (s LocalState) IsAbnormal() bool {
	return s == LocalStateWarning || s == LocalStateCritical
}

// Los ids viajan como segmento de un tópico MQTT: sin "/", "+" ni "#".
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// ValidateID verifica que un id externo (dispositivo, lectura, espacio) sea
// seguro para usarse como clave y como segmento de tópico.
func ValidateID(field, id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("%w: %s must be 1-64 characters (letters, digits, '-' or '_')", ErrInvalid, field)
	}
	return nil
}

// ThresholdRange define el rango permitido para una métrica monitoreada.
type ThresholdRange struct {
	MinValue float64
	MaxValue float64
	Unit     string
}

// NewThresholdRange acepta min == max: las lecturas evento se registran con
// umbral máximo 0 para que el valor 1 genere una alerta.
func NewThresholdRange(min, max float64, unit string) (ThresholdRange, error) {
	if math.IsNaN(min) || math.IsNaN(max) || math.IsInf(min, 0) || math.IsInf(max, 0) {
		return ThresholdRange{}, fmt.Errorf("%w: threshold must be finite", ErrInvalid)
	}
	if min > max {
		return ThresholdRange{}, fmt.Errorf("%w: min must not be greater than max", ErrInvalid)
	}
	return ThresholdRange{MinValue: min, MaxValue: max, Unit: unit}, nil
}

func (t ThresholdRange) Contains(v float64) bool {
	return v >= t.MinValue && v <= t.MaxValue
}

// Deviation devuelve cuánto se aleja v del rango, como fracción de su amplitud.
// Es 0 cuando v está dentro del rango y 1 si el rango no tiene amplitud.
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
	width := t.MaxValue - t.MinValue
	if width == 0 {
		return 1
	}
	return distance / width
}

// SeverityLevel representa el nivel de gravedad de una alerta.
type SeverityLevel string

const (
	SeverityLow      SeverityLevel = "low"
	SeverityMedium   SeverityLevel = "medium"
	SeverityHigh     SeverityLevel = "high"
	SeverityCritical SeverityLevel = "critical"
)

// Rank ordena las severidades de menor (1) a mayor (4).
func (s SeverityLevel) Rank() int {
	switch s {
	case SeverityLow:
		return 1
	case SeverityMedium:
		return 2
	case SeverityHigh:
		return 3
	case SeverityCritical:
		return 4
	}
	return 0
}

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
