package application

import (
	"time"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

type ReadingDto struct {
	ID         string
	DeviceID   string
	MetricType string
	Value      float64
	Unit       string
	ReceivedAt time.Time
	Status     string
	MeasuredAt *time.Time
	LocalState string
}

type AlertDto struct {
	ID             string
	ReadingID      string
	DeviceID       string
	SpaceID        string
	MetricType     string
	Message        string
	Severity       string
	Status         string
	CreatedAt      time.Time
	AcknowledgedAt *time.Time
	ClosedAt       *time.Time
	Reason         string
	EventID        string
	EvidenceRef    string
}

type DeviceDto struct {
	ID         string
	SpaceID    string
	Name       string
	Active     bool
	Thresholds []ThresholdInput
	CreatedAt  time.Time
}

// MonitoringSummaryDto agrupa el estado general del monitoreo de un espacio.
type MonitoringSummaryDto struct {
	SpaceID        string
	RecentReadings int
	ActiveAlerts   int
	Status         string // "normal" o "attention"
}

// EdgeCommandDto confirma que la orden se publicó hacia el Edge.
type EdgeCommandDto struct {
	CommandID string
	Target    string
	Action    string
}

func ToReadingDto(r *domain.Reading) ReadingDto {
	return ReadingDto{
		ID: r.ID, DeviceID: r.DeviceID, MetricType: string(r.Metric), Value: r.Value,
		Unit: r.Unit, ReceivedAt: r.ReceivedAt, Status: string(r.Status),
		MeasuredAt: r.MeasuredAt, LocalState: string(r.LocalState),
	}
}

func ToAlertDto(a *domain.Alert) AlertDto {
	return AlertDto{
		ID: a.ID, ReadingID: a.ReadingID, DeviceID: a.DeviceID, SpaceID: a.SpaceID,
		MetricType: string(a.Metric), Message: a.Message, Severity: string(a.Severity), Status: string(a.Status),
		CreatedAt: a.CreatedAt, AcknowledgedAt: a.AcknowledgedAt, ClosedAt: a.ClosedAt,
		Reason: a.Reason, EventID: a.EventID, EvidenceRef: a.EvidenceRef,
	}
}

func ToDeviceDto(d *domain.IoTDevice) DeviceDto {
	ths := make([]ThresholdInput, 0, len(d.Thresholds))
	for m, t := range d.Thresholds {
		ths = append(ths, ThresholdInput{MetricType: string(m), MinValue: t.MinValue, MaxValue: t.MaxValue, Unit: t.Unit})
	}
	return DeviceDto{ID: d.ID, SpaceID: d.SpaceID, Name: d.Name, Active: d.Active, Thresholds: ths, CreatedAt: d.CreatedAt}
}
