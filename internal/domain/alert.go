package domain

import (
	"fmt"
	"time"
)

// Alert se genera cuando una lectura está fuera de los límites permitidos.
type Alert struct {
	ID             string
	ReadingID      string
	DeviceID       string
	SpaceID        string
	Severity       SeverityLevel
	Status         AlertStatus
	Message        string
	CreatedAt      time.Time
	AcknowledgedAt *time.Time
	ClosedAt       *time.Time
}

func NewAlert(reading *Reading, device *IoTDevice, severity SeverityLevel, message string, now time.Time) *Alert {
	return &Alert{
		ID:        NewID(),
		ReadingID: reading.ID,
		DeviceID:  device.ID,
		SpaceID:   device.SpaceID,
		Severity:  severity,
		Status:    AlertCreated,
		Message:   message,
		CreatedAt: now,
	}
}

// MarkSent registra que la notificación fue entregada al usuario responsable.
func (a *Alert) MarkSent() error {
	if a.Status != AlertCreated {
		return fmt.Errorf("%w: cannot mark %s alert as sent", ErrInvalidTransition, a.Status)
	}
	a.Status = AlertSent
	return nil
}

// Acknowledge permite reconocer una alerta pendiente (creada o enviada).
func (a *Alert) Acknowledge(now time.Time) error {
	if a.Status != AlertCreated && a.Status != AlertSent {
		return fmt.Errorf("%w: cannot acknowledge %s alert", ErrInvalidTransition, a.Status)
	}
	a.Status = AlertAcknowledged
	a.AcknowledgedAt = &now
	return nil
}

// Close cierra una alerta que aún no está cerrada.
//
// TODO(faltante): el informe no aclara si una alerta puede cerrarse sin
// haber sido reconocida; por ahora se permite desde cualquier estado abierto.
func (a *Alert) Close(now time.Time) error {
	if a.Status == AlertClosed {
		return fmt.Errorf("%w: alert is already closed", ErrInvalidTransition)
	}
	a.Status = AlertClosed
	a.ClosedAt = &now
	return nil
}
