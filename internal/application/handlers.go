package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

type Clock func() time.Time

// IngestResult es la salida de IngestReadingCommandHandler; Alert es nil si
// la lectura estaba dentro del rango.
type IngestResult struct {
	Reading ReadingDto
	Alert   *AlertDto
}

type IngestReadingCommandHandler struct {
	Devices    domain.IIoTDeviceRepository
	Readings   domain.IReadingRepository
	Alerts     domain.IAlertRepository
	Monitoring domain.MonitoringDomainService
	Policy     domain.AlertPolicyService
	Notifier   Notifier
	Normalizer ReadingNormalizer
	Now        Clock
	Log        *slog.Logger
}

// Handle valida que el dispositivo exista y esté activo, registra la lectura,
// evalúa si está fuera de rango y, si corresponde, genera y notifica una alerta.
func (h *IngestReadingCommandHandler) Handle(ctx context.Context, cmd IngestReadingCommand) (IngestResult, error) {
	cmd, err := h.Normalizer.Normalize(cmd)
	if err != nil {
		return IngestResult{}, err
	}
	metric, err := domain.ParseMetricType(cmd.MetricType)
	if err != nil {
		return IngestResult{}, err
	}
	device, err := h.Devices.FindByID(ctx, cmd.DeviceID)
	if err != nil {
		return IngestResult{}, err
	}
	now := h.Now()
	reading, err := domain.NewReading(cmd.DeviceID, metric, cmd.Value, cmd.Unit, now)
	if err != nil {
		return IngestResult{}, err
	}
	if err := h.Monitoring.ValidateReading(device, reading); err != nil {
		return IngestResult{}, err
	}

	threshold, outOfRange := h.Monitoring.DetectOutOfRange(device, reading)
	if outOfRange {
		reading.MarkOutOfRange()
	} else {
		reading.MarkValidated()
	}
	if err := h.Readings.Save(ctx, reading); err != nil {
		return IngestResult{}, fmt.Errorf("save reading: %w", err)
	}
	result := IngestResult{Reading: ToReadingDto(reading)}
	if !outOfRange {
		return result, nil
	}

	alert := h.Monitoring.CreateAlert(device, reading, threshold, now)
	if err := h.Alerts.Save(ctx, alert); err != nil {
		return IngestResult{}, fmt.Errorf("save alert: %w", err)
	}
	h.dispatch(ctx, alert, device)
	dto := ToAlertDto(alert)
	result.Alert = &dto
	return result, nil
}

// dispatch no falla la ingesta: si el envío falla, la alerta queda en estado
// "created" y el error se registra.
func (h *IngestReadingCommandHandler) dispatch(ctx context.Context, alert *domain.Alert, device *domain.IoTDevice) {
	if !h.Policy.CanDispatchAlert(alert) {
		return
	}
	if err := h.Notifier.Notify(ctx, alert, device); err != nil {
		h.Log.Error("alert notification failed", "alertId", alert.ID, "error", err)
		return
	}
	if err := alert.MarkSent(); err != nil {
		return
	}
	if err := h.Alerts.Save(ctx, alert); err != nil {
		h.Log.Error("update alert status failed", "alertId", alert.ID, "error", err)
	}
}

type AcknowledgeAlertCommandHandler struct {
	Alerts domain.IAlertRepository
	Now    Clock
}

func (h *AcknowledgeAlertCommandHandler) Handle(ctx context.Context, cmd AcknowledgeAlertCommand) (AlertDto, error) {
	alert, err := h.Alerts.FindByID(ctx, cmd.AlertID)
	if err != nil {
		return AlertDto{}, err
	}
	if err := alert.Acknowledge(h.Now()); err != nil {
		return AlertDto{}, err
	}
	if err := h.Alerts.Save(ctx, alert); err != nil {
		return AlertDto{}, fmt.Errorf("save alert: %w", err)
	}
	return ToAlertDto(alert), nil
}

type CloseAlertCommandHandler struct {
	Alerts domain.IAlertRepository
	Now    Clock
}

func (h *CloseAlertCommandHandler) Handle(ctx context.Context, cmd CloseAlertCommand) (AlertDto, error) {
	alert, err := h.Alerts.FindByID(ctx, cmd.AlertID)
	if err != nil {
		return AlertDto{}, err
	}
	if err := alert.Close(h.Now()); err != nil {
		return AlertDto{}, err
	}
	if err := h.Alerts.Save(ctx, alert); err != nil {
		return AlertDto{}, fmt.Errorf("save alert: %w", err)
	}
	return ToAlertDto(alert), nil
}

type RegisterDeviceCommandHandler struct {
	Devices domain.IIoTDeviceRepository
	Now     Clock
}

func (h *RegisterDeviceCommandHandler) Handle(ctx context.Context, cmd RegisterDeviceCommand) (DeviceDto, error) {
	thresholds := make(map[domain.MetricType]domain.ThresholdRange, len(cmd.Thresholds))
	for _, in := range cmd.Thresholds {
		metric, err := domain.ParseMetricType(in.MetricType)
		if err != nil {
			return DeviceDto{}, err
		}
		tr, err := domain.NewThresholdRange(in.MinValue, in.MaxValue, in.Unit)
		if err != nil {
			return DeviceDto{}, err
		}
		thresholds[metric] = tr
	}
	device, err := domain.NewIoTDevice(cmd.SpaceID, cmd.Name, thresholds, h.Now())
	if err != nil {
		return DeviceDto{}, err
	}
	if err := h.Devices.Save(ctx, device); err != nil {
		return DeviceDto{}, fmt.Errorf("save device: %w", err)
	}
	return ToDeviceDto(device), nil
}

type SetDeviceActiveCommandHandler struct {
	Devices domain.IIoTDeviceRepository
}

func (h *SetDeviceActiveCommandHandler) Activate(ctx context.Context, cmd ActivateDeviceCommand) (DeviceDto, error) {
	return h.set(ctx, cmd.DeviceID, (*domain.IoTDevice).Activate)
}

func (h *SetDeviceActiveCommandHandler) Deactivate(ctx context.Context, cmd DeactivateDeviceCommand) (DeviceDto, error) {
	return h.set(ctx, cmd.DeviceID, (*domain.IoTDevice).Deactivate)
}

func (h *SetDeviceActiveCommandHandler) set(ctx context.Context, id string, change func(*domain.IoTDevice)) (DeviceDto, error) {
	device, err := h.Devices.FindByID(ctx, id)
	if err != nil {
		return DeviceDto{}, err
	}
	change(device)
	if err := h.Devices.Save(ctx, device); err != nil {
		return DeviceDto{}, fmt.Errorf("save device: %w", err)
	}
	return ToDeviceDto(device), nil
}

const (
	defaultReadingsLimit = 100
	maxReadingsLimit     = 1000
)

type GetReadingsBySpaceQueryHandler struct {
	Readings domain.IReadingRepository
}

func (h *GetReadingsBySpaceQueryHandler) Handle(ctx context.Context, q GetReadingsBySpaceQuery) ([]ReadingDto, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultReadingsLimit
	}
	if limit > maxReadingsLimit {
		limit = maxReadingsLimit
	}
	readings, err := h.Readings.FindBySpace(ctx, q.SpaceID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ReadingDto, 0, len(readings))
	for _, r := range readings {
		out = append(out, ToReadingDto(r))
	}
	return out, nil
}

type GetAlertsBySpaceQueryHandler struct {
	Alerts domain.IAlertRepository
}

func (h *GetAlertsBySpaceQueryHandler) Handle(ctx context.Context, q GetAlertsBySpaceQuery) ([]AlertDto, error) {
	var status *domain.AlertStatus
	if q.Status != "" {
		s, err := domain.ParseAlertStatus(q.Status)
		if err != nil {
			return nil, err
		}
		status = &s
	}
	alerts, err := h.Alerts.FindBySpace(ctx, q.SpaceID, status)
	if err != nil {
		return nil, err
	}
	out := make([]AlertDto, 0, len(alerts))
	for _, a := range alerts {
		out = append(out, ToAlertDto(a))
	}
	return out, nil
}

type GetDevicesBySpaceQueryHandler struct {
	Devices domain.IIoTDeviceRepository
}

func (h *GetDevicesBySpaceQueryHandler) Handle(ctx context.Context, q GetDevicesBySpaceQuery) ([]DeviceDto, error) {
	devices, err := h.Devices.FindBySpace(ctx, q.SpaceID)
	if err != nil {
		return nil, err
	}
	out := make([]DeviceDto, 0, len(devices))
	for _, d := range devices {
		out = append(out, ToDeviceDto(d))
	}
	return out, nil
}

type GetMonitoringSummaryQueryHandler struct {
	Readings domain.IReadingRepository
	Alerts   domain.IAlertRepository
}

func (h *GetMonitoringSummaryQueryHandler) Handle(ctx context.Context, q GetMonitoringSummaryQuery) (MonitoringSummaryDto, error) {
	readings, err := h.Readings.FindBySpace(ctx, q.SpaceID, defaultReadingsLimit)
	if err != nil {
		return MonitoringSummaryDto{}, err
	}
	alerts, err := h.Alerts.FindBySpace(ctx, q.SpaceID, nil)
	if err != nil {
		return MonitoringSummaryDto{}, err
	}
	active := 0
	for _, a := range alerts {
		if a.Status != domain.AlertClosed {
			active++
		}
	}
	status := "normal"
	if active > 0 {
		status = "attention"
	}
	return MonitoringSummaryDto{SpaceID: q.SpaceID, RecentReadings: len(readings), ActiveAlerts: active, Status: status}, nil
}
