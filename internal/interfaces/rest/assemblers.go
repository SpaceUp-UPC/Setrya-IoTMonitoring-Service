package rest

import (
	"fmt"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

func IngestReadingCommandFromResourceAssembler(r IngestReadingResource) (application.IngestReadingCommand, error) {
	if r.Value == nil {
		return application.IngestReadingCommand{}, fmt.Errorf("%w: value is required", domain.ErrInvalid)
	}
	return application.IngestReadingCommand{
		DeviceID: r.DeviceID, MetricType: r.MetricType, Value: *r.Value, Unit: r.Unit,
		ReadingID: r.ReadingID, MeasuredAt: r.MeasuredAt, LocalState: r.LocalState,
		Reason: r.Reason, EventID: r.EventID, EvidenceRef: r.EvidenceRef,
	}, nil
}

func AcknowledgeAlertCommandFromResourceAssembler(r AcknowledgeAlertResource) application.AcknowledgeAlertCommand {
	return application.AcknowledgeAlertCommand{AlertID: r.AlertID}
}

func CloseAlertCommandFromResourceAssembler(r CloseAlertResource) application.CloseAlertCommand {
	return application.CloseAlertCommand{AlertID: r.AlertID}
}

func RegisterDeviceCommandFromResourceAssembler(r RegisterDeviceResource) application.RegisterDeviceCommand {
	ths := make([]application.ThresholdInput, 0, len(r.Thresholds))
	for _, t := range r.Thresholds {
		ths = append(ths, application.ThresholdInput{MetricType: t.MetricType, MinValue: t.MinValue, MaxValue: t.MaxValue, Unit: t.Unit})
	}
	return application.RegisterDeviceCommand{ID: r.ID, SpaceID: r.SpaceID, Name: r.Name, Active: r.Active, Thresholds: ths}
}

func ReadingResourceFromEntityAssembler(d application.ReadingDto) ReadingResource {
	return ReadingResource{
		ID: d.ID, DeviceID: d.DeviceID, MetricType: d.MetricType, Value: d.Value, Unit: d.Unit,
		ReceivedAt: d.ReceivedAt, Status: d.Status, MeasuredAt: d.MeasuredAt, LocalState: d.LocalState,
	}
}

func AlertResourceFromEntityAssembler(d application.AlertDto) AlertResource {
	return AlertResource{
		ID: d.ID, ReadingID: d.ReadingID, DeviceID: d.DeviceID, SpaceID: d.SpaceID, MetricType: d.MetricType, Message: d.Message,
		Severity: d.Severity, Status: d.Status, CreatedAt: d.CreatedAt, AcknowledgedAt: d.AcknowledgedAt, ClosedAt: d.ClosedAt,
		Reason: d.Reason, EventID: d.EventID, EvidenceRef: d.EvidenceRef,
	}
}

func DeviceResourceFromEntityAssembler(d application.DeviceDto) DeviceResource {
	ths := make([]ThresholdResource, 0, len(d.Thresholds))
	for _, t := range d.Thresholds {
		ths = append(ths, ThresholdResource{MetricType: t.MetricType, MinValue: t.MinValue, MaxValue: t.MaxValue, Unit: t.Unit})
	}
	return DeviceResource{ID: d.ID, SpaceID: d.SpaceID, Name: d.Name, Active: d.Active, Thresholds: ths, CreatedAt: d.CreatedAt}
}

func IngestResultResourceAssembler(r application.IngestResult) IngestResultResource {
	out := IngestResultResource{Reading: ReadingResourceFromEntityAssembler(r.Reading), Duplicate: r.Duplicate}
	if r.Alert != nil {
		a := AlertResourceFromEntityAssembler(*r.Alert)
		out.Alert = &a
	}
	return out
}

func MonitoringSummaryResourceAssembler(d application.MonitoringSummaryDto) MonitoringSummaryResource {
	return MonitoringSummaryResource{SpaceID: d.SpaceID, RecentReadings: d.RecentReadings, ActiveAlerts: d.ActiveAlerts, Status: d.Status}
}

func ValidateSecurityAlertCommandFromResourceAssembler(alertID string, r ValidateAlertResource) application.ValidateSecurityAlertCommand {
	return application.ValidateSecurityAlertCommand{AlertID: alertID, Decision: r.Decision}
}

func SendEdgeCommandCommandFromResourceAssembler(spaceID, target string, r EdgeCommandResource) application.SendEdgeCommandCommand {
	return application.SendEdgeCommandCommand{SpaceID: spaceID, Target: target, Action: r.Action, Params: r.Params}
}

func EdgeCommandAcceptedResourceAssembler(d application.EdgeCommandDto) EdgeCommandAcceptedResource {
	return EdgeCommandAcceptedResource{CommandID: d.CommandID, Target: d.Target, Action: d.Action}
}
