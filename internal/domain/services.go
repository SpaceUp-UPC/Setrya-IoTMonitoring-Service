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
	CreateAlert(device *IoTDevice, reading *Reading, threshold ThresholdRange, now time.Time) *Alert
}

// AlertPolicyService define los criterios de severidad y despacho de alertas.
type AlertPolicyService interface {
	EvaluateSeverity(metric MetricType, value float64, threshold ThresholdRange) SeverityLevel
	CanDispatchAlert(alert *Alert) bool
}

// DefaultThresholds se usa cuando el dispositivo no tiene un umbral propio
// para la métrica.
//
// TODO(faltante): el informe (US06/US07) habla de umbrales de temperatura
// configurables pero no define dónde se configuran ni los valores por
// defecto. Estos valores son provisionales.
var DefaultThresholds = map[MetricType]ThresholdRange{
	MetricTemperature: {MinValue: 15, MaxValue: 35, Unit: "°C"},
	MetricVoltage:     {MinValue: 200, MaxValue: 240, Unit: "V"},
	MetricSound:       {MinValue: 0, MaxValue: 70, Unit: "dB"},
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

// DetectOutOfRange evalúa si el valor está fuera del rango permitido y
// devuelve el umbral aplicado.
func (s *monitoringDomainService) DetectOutOfRange(device *IoTDevice, reading *Reading) (ThresholdRange, bool) {
	threshold, ok := device.ThresholdFor(reading.Metric)
	if !ok {
		threshold = DefaultThresholds[reading.Metric]
	}
	return threshold, !threshold.Contains(reading.Value)
}

func (s *monitoringDomainService) CreateAlert(device *IoTDevice, reading *Reading, threshold ThresholdRange, now time.Time) *Alert {
	severity := s.policy.EvaluateSeverity(reading.Metric, reading.Value, threshold)
	msg := fmt.Sprintf("%s fuera de rango en %q: %.2f %s (permitido %.2f a %.2f)",
		reading.Metric, device.Name, reading.Value, reading.Unit, threshold.MinValue, threshold.MaxValue)
	return NewAlert(reading, device, severity, msg, now)
}

type alertPolicyService struct{}

func NewAlertPolicyService() AlertPolicyService { return alertPolicyService{} }

// EvaluateSeverity clasifica la alerta según cuánto se aleja el valor del
// rango permitido.
//
// TODO(faltante): el informe no define los criterios de severidad por
// métrica (solo los cuatro niveles). Se usa la desviación relativa al ancho
// del rango: <10% baja, <25% media, <50% alta y desde 50% crítica.
func (alertPolicyService) EvaluateSeverity(_ MetricType, value float64, threshold ThresholdRange) SeverityLevel {
	switch d := threshold.Deviation(value); {
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

// CanDispatchAlert indica si la alerta puede enviarse al usuario responsable:
// solo las recién creadas.
func (alertPolicyService) CanDispatchAlert(alert *Alert) bool {
	return alert.Status == AlertCreated
}
