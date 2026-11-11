package domain

import (
	"fmt"
	"time"
)

// MonitoringDomainService define las reglas para validar lecturas, detectar
// valores fuera de rango y generar alertas.
type MonitoringDomainService interface {
	ValidateReading(device *IoTDevice, reading *Reading) error
	DetectOutOfRange(device *IoTDevice, reading *Reading) (ThresholdRange, bool)
	CreateAlert(device *IoTDevice, reading *Reading, threshold ThresholdRange, event EventDetails, now time.Time) *Alert
}

// AlertPolicyService define los criterios de severidad y despacho de alertas.
type AlertPolicyService interface {
	EvaluateSeverity(reading *Reading, threshold ThresholdRange) SeverityLevel
	// ShouldRaise decide si una lectura anómala amerita una alerta nueva, dadas
	// las alertas abiertas del mismo dispositivo y métrica.
	ShouldRaise(metric MetricType, severity SeverityLevel, open []*Alert) bool
	CanDispatchAlert(alert *Alert) bool
}

// DefaultThresholds se usa cuando el dispositivo no tiene un umbral propio
// para la métrica. Temperatura y voltaje toman el nivel Warning del informe
// (5.6); humedad toma el perfil del Edge. Lo normal es registrar cada
// dispositivo con los umbrales del perfil activo del Edge (demo o report).
var DefaultThresholds = map[MetricType]ThresholdRange{
	MetricTemperature: {MinValue: -40, MaxValue: 45, Unit: "°C"},
	MetricHumidity:    {MinValue: 40, MaxValue: 65, Unit: "%"},
	MetricVoltage:     {MinValue: 198, MaxValue: 242, Unit: "V"},
	MetricSound:       {MinValue: 0, MaxValue: 70, Unit: "dB"},
	// Lecturas evento: umbral máximo 0, el valor 1 genera alerta.
	MetricIntrusion:   {Unit: EventUnit},
	MetricEscalation:  {Unit: EventUnit},
	MetricDeviceError: {Unit: EventUnit},
}

type monitoringDomainService struct {
	policy AlertPolicyService
}

func NewMonitoringDomainService(policy AlertPolicyService) MonitoringDomainService {
	return &monitoringDomainService{policy: policy}
}

// ValidateReading verifica que la lectura provenga de un dispositivo
// registrado y activo.
func (s *monitoringDomainService) ValidateReading(device *IoTDevice, reading *Reading) error {
	if device == nil {
		return ErrDeviceNotFound
	}
	if !device.IsActive() {
		return ErrDeviceInactive
	}
	if reading.DeviceID != device.ID {
		return fmt.Errorf("%w: reading does not belong to device", ErrInvalid)
	}
	return nil
}

// DetectOutOfRange evalúa si la lectura es anómala y devuelve el umbral
// aplicado. Lo es si el valor sale del rango o si el Edge, que decide con sus
// propias reglas (histéresis, lecturas consecutivas), ya la marcó WARNING o
// CRITICAL.
func (s *monitoringDomainService) DetectOutOfRange(device *IoTDevice, reading *Reading) (ThresholdRange, bool) {
	threshold, ok := device.ThresholdFor(reading.Metric)
	if !ok {
		threshold = DefaultThresholds[reading.Metric]
	}
	return threshold, !threshold.Contains(reading.Value) || reading.LocalState.IsAbnormal()
}

func (s *monitoringDomainService) CreateAlert(device *IoTDevice, reading *Reading, threshold ThresholdRange, event EventDetails, now time.Time) *Alert {
	severity := s.policy.EvaluateSeverity(reading, threshold)
	return NewAlert(reading, device, severity, alertMessage(device, reading, threshold, event), event, now)
}

func alertMessage(device *IoTDevice, reading *Reading, threshold ThresholdRange, event EventDetails) string {
	var msg string
	switch reading.Metric {
	case MetricIntrusion:
		msg = fmt.Sprintf("Intrusión detectada en %q", device.Name)
	case MetricEscalation:
		msg = fmt.Sprintf("Escalamiento a autoridades en %q", device.Name)
	case MetricDeviceError:
		msg = fmt.Sprintf("Sensor sin respuesta en %q", device.Name)
	default:
		msg = fmt.Sprintf("%s fuera de rango en %q: %.2f %s (permitido %.2f a %.2f)",
			reading.Metric, device.Name, reading.Value, reading.Unit, threshold.MinValue, threshold.MaxValue)
		if reading.LocalState.IsAbnormal() {
			msg = fmt.Sprintf("%s en estado %s en %q: %.2f %s",
				reading.Metric, reading.LocalState, device.Name, reading.Value, reading.Unit)
		}
	}
	if event.Reason != "" {
		msg += " (motivo: " + event.Reason + ")"
	}
	// La columna admite 255 caracteres; el motivo llega de fuera.
	if r := []rune(msg); len(r) > 255 {
		msg = string(r[:255])
	}
	return msg
}

type alertPolicyService struct{}

func NewAlertPolicyService() AlertPolicyService { return alertPolicyService{} }

// EvaluateSeverity clasifica la alerta. Los eventos tienen severidad fija; si
// el Edge envió su estado local, manda ese estado; en otro caso se usa la
// desviación relativa al ancho del rango (<10 % baja, <25 % media, <50 % alta
// y desde 50 % crítica).
func (alertPolicyService) EvaluateSeverity(reading *Reading, threshold ThresholdRange) SeverityLevel {
	switch reading.Metric {
	case MetricIntrusion:
		return SeverityHigh
	case MetricEscalation:
		return SeverityCritical
	case MetricDeviceError:
		return SeverityMedium
	}
	switch reading.LocalState {
	case LocalStateCritical:
		return SeverityCritical
	case LocalStateWarning:
		return SeverityMedium
	}
	switch d := threshold.Deviation(reading.Value); {
	case d < 0.10:
		return SeverityLow
	case d < 0.25:
		return SeverityMedium
	case d < 0.50:
		return SeverityHigh
	default:
		return SeverityCritical
	}
}

// ShouldRaise evita una alerta por cada lectura mientras la condición dura
// (el Edge muestrea cada 1 a 5 s en WARNING/CRITICAL): solo se crea una
// alerta nueva si no hay otra abierta de igual o mayor severidad. Intrusión
// y escalamiento son hechos puntuales y siempre generan alerta.
func (alertPolicyService) ShouldRaise(metric MetricType, severity SeverityLevel, open []*Alert) bool {
	if metric == MetricIntrusion || metric == MetricEscalation {
		return true
	}
	for _, a := range open {
		if a.IsOpen() && a.Severity.Rank() >= severity.Rank() {
			return false
		}
	}
	return true
}

// CanDispatchAlert indica si la alerta puede enviarse al usuario responsable:
// solo las recién creadas.
func (alertPolicyService) CanDispatchAlert(alert *Alert) bool {
	return alert.Status == AlertCreated
}
