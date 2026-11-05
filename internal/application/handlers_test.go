package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/infrastructure/normalizer"
)

type memDevices struct{ m map[string]*domain.IoTDevice }

func (r *memDevices) Save(_ context.Context, d *domain.IoTDevice) error { r.m[d.ID] = d; return nil }
func (r *memDevices) FindByID(_ context.Context, id string) (*domain.IoTDevice, error) {
	if d, ok := r.m[id]; ok {
		return d, nil
	}
	return nil, domain.ErrDeviceNotFound
}
func (r *memDevices) FindBySpace(_ context.Context, s string) ([]*domain.IoTDevice, error) {
	var out []*domain.IoTDevice
	for _, d := range r.m {
		if d.SpaceID == s {
			out = append(out, d)
		}
	}
	return out, nil
}

type memReadings struct{ list []*domain.Reading }

func (r *memReadings) Save(_ context.Context, rd *domain.Reading) error {
	r.list = append(r.list, rd)
	return nil
}
func (r *memReadings) FindBySpace(_ context.Context, _ string, limit int) ([]*domain.Reading, error) {
	if len(r.list) > limit {
		return r.list[:limit], nil
	}
	return r.list, nil
}

type memAlerts struct{ m map[string]*domain.Alert }

func (r *memAlerts) Save(_ context.Context, a *domain.Alert) error { r.m[a.ID] = a; return nil }
func (r *memAlerts) FindByID(_ context.Context, id string) (*domain.Alert, error) {
	if a, ok := r.m[id]; ok {
		return a, nil
	}
	return nil, domain.ErrAlertNotFound
}
func (r *memAlerts) FindBySpace(_ context.Context, space string, st *domain.AlertStatus) ([]*domain.Alert, error) {
	var out []*domain.Alert
	for _, a := range r.m {
		if a.SpaceID == space && (st == nil || a.Status == *st) {
			out = append(out, a)
		}
	}
	return out, nil
}

type fakeNotifier struct {
	calls int
	err   error
}

func (n *fakeNotifier) Notify(context.Context, *domain.Alert, *domain.IoTDevice) error {
	n.calls++
	return n.err
}

type env struct {
	devices  *memDevices
	readings *memReadings
	alerts   *memAlerts
	notifier *fakeNotifier
	ingest   *application.IngestReadingCommandHandler
	deviceID string
}

func newEnv(t *testing.T, active bool) *env {
	t.Helper()
	e := &env{
		devices:  &memDevices{m: map[string]*domain.IoTDevice{}},
		readings: &memReadings{},
		alerts:   &memAlerts{m: map[string]*domain.Alert{}},
		notifier: &fakeNotifier{},
	}
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	d, _ := domain.NewIoTDevice("space-1", "Kitchen sensor", nil, now)
	if active {
		d.Activate()
	}
	e.devices.m[d.ID] = d
	e.deviceID = d.ID

	policy := domain.NewAlertPolicyService()
	e.ingest = &application.IngestReadingCommandHandler{
		Devices: e.devices, Readings: e.readings, Alerts: e.alerts,
		Monitoring: domain.NewMonitoringDomainService(policy), Policy: policy,
		Notifier: e.notifier, Normalizer: normalizer.New(),
		Now: func() time.Time { return now }, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return e
}

func TestIngestInRangeCreatesNoAlert(t *testing.T) {
	e := newEnv(t, true)
	res, err := e.ingest.Handle(context.Background(), application.IngestReadingCommand{DeviceID: e.deviceID, MetricType: "temperature", Value: 22, Unit: "C"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Alert != nil || res.Reading.Status != "validated" || len(e.alerts.m) != 0 || e.notifier.calls != 0 {
		t.Fatalf("unexpected result %+v", res)
	}
}

func TestIngestOutOfRangeCreatesAndSendsAlert(t *testing.T) {
	e := newEnv(t, true)
	// 100°F = 37.78°C, fuera del rango por defecto 15-35.
	res, err := e.ingest.Handle(context.Background(), application.IngestReadingCommand{DeviceID: e.deviceID, MetricType: "Temperature", Value: 100, Unit: "F"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Reading.Status != "out_of_range" || res.Reading.Unit != "°C" || res.Alert == nil {
		t.Fatalf("unexpected result %+v", res)
	}
	if res.Alert.Status != "created" && e.alerts.m[res.Alert.ID].Status != domain.AlertSent {
		t.Fatalf("alert not sent: %+v", e.alerts.m[res.Alert.ID])
	}
	if e.notifier.calls != 1 || e.alerts.m[res.Alert.ID].Status != domain.AlertSent {
		t.Fatalf("notifier calls=%d status=%s", e.notifier.calls, e.alerts.m[res.Alert.ID].Status)
	}
}

func TestIngestNotifierFailureKeepsAlertCreated(t *testing.T) {
	e := newEnv(t, true)
	e.notifier.err = errors.New("smtp down")
	res, err := e.ingest.Handle(context.Background(), application.IngestReadingCommand{DeviceID: e.deviceID, MetricType: "voltage", Value: 300, Unit: "V"})
	if err != nil || res.Alert == nil {
		t.Fatalf("ingest must not fail: %v %+v", err, res)
	}
	if e.alerts.m[res.Alert.ID].Status != domain.AlertCreated {
		t.Fatalf("status = %s", e.alerts.m[res.Alert.ID].Status)
	}
}

func TestIngestRejections(t *testing.T) {
	cases := []struct {
		name   string
		active bool
		cmd    func(id string) application.IngestReadingCommand
		want   error
	}{
		{"inactive device", false, func(id string) application.IngestReadingCommand {
			return application.IngestReadingCommand{DeviceID: id, MetricType: "temperature", Value: 20, Unit: "C"}
		}, domain.ErrDeviceInactive},
		{"unknown device", true, func(string) application.IngestReadingCommand {
			return application.IngestReadingCommand{DeviceID: "nope", MetricType: "temperature", Value: 20, Unit: "C"}
		}, domain.ErrDeviceNotFound},
		{"unknown metric", true, func(id string) application.IngestReadingCommand {
			return application.IngestReadingCommand{DeviceID: id, MetricType: "humidity", Value: 20, Unit: "%"}
		}, domain.ErrInvalid},
		{"bad unit", true, func(id string) application.IngestReadingCommand {
			return application.IngestReadingCommand{DeviceID: id, MetricType: "voltage", Value: 220, Unit: "kg"}
		}, domain.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, c.active)
			_, err := e.ingest.Handle(context.Background(), c.cmd(e.deviceID))
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v want %v", err, c.want)
			}
			if len(e.readings.list) != 0 {
				t.Fatal("rejected reading must not be stored")
			}
		})
	}
}

func TestAcknowledgeAndCloseAlert(t *testing.T) {
	e := newEnv(t, true)
	res, _ := e.ingest.Handle(context.Background(), application.IngestReadingCommand{DeviceID: e.deviceID, MetricType: "voltage", Value: 300, Unit: "V"})
	now := func() time.Time { return time.Now() }
	ack := &application.AcknowledgeAlertCommandHandler{Alerts: e.alerts, Now: now}
	cl := &application.CloseAlertCommandHandler{Alerts: e.alerts, Now: now}

	dto, err := ack.Handle(context.Background(), application.AcknowledgeAlertCommand{AlertID: res.Alert.ID})
	if err != nil || dto.Status != "acknowledged" {
		t.Fatalf("ack: %v %+v", err, dto)
	}
	if _, err := cl.Handle(context.Background(), application.CloseAlertCommand{AlertID: res.Alert.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := ack.Handle(context.Background(), application.AcknowledgeAlertCommand{AlertID: res.Alert.ID}); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("ack closed alert: %v", err)
	}
	if _, err := cl.Handle(context.Background(), application.CloseAlertCommand{AlertID: "missing"}); !errors.Is(err, domain.ErrAlertNotFound) {
		t.Fatalf("missing alert: %v", err)
	}
}

func TestQueriesAndSummary(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	_, _ = e.ingest.Handle(ctx, application.IngestReadingCommand{DeviceID: e.deviceID, MetricType: "temperature", Value: 20, Unit: "C"})
	_, _ = e.ingest.Handle(ctx, application.IngestReadingCommand{DeviceID: e.deviceID, MetricType: "voltage", Value: 300, Unit: "V"})

	readings, _ := (&application.GetReadingsBySpaceQueryHandler{Readings: e.readings}).Handle(ctx, application.GetReadingsBySpaceQuery{SpaceID: "space-1"})
	if len(readings) != 2 {
		t.Fatalf("readings = %d", len(readings))
	}
	alerts, err := (&application.GetAlertsBySpaceQueryHandler{Alerts: e.alerts}).Handle(ctx, application.GetAlertsBySpaceQuery{SpaceID: "space-1", Status: "sent"})
	if err != nil || len(alerts) != 1 {
		t.Fatalf("alerts = %d err=%v", len(alerts), err)
	}
	if _, err := (&application.GetAlertsBySpaceQueryHandler{Alerts: e.alerts}).Handle(ctx, application.GetAlertsBySpaceQuery{SpaceID: "space-1", Status: "bogus"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bogus status: %v", err)
	}
	sum, _ := (&application.GetMonitoringSummaryQueryHandler{Readings: e.readings, Alerts: e.alerts}).Handle(ctx, application.GetMonitoringSummaryQuery{SpaceID: "space-1"})
	if sum.ActiveAlerts != 1 || sum.RecentReadings != 2 || sum.Status != "attention" {
		t.Fatalf("summary %+v", sum)
	}
}

func TestRegisterAndActivateDevice(t *testing.T) {
	devices := &memDevices{m: map[string]*domain.IoTDevice{}}
	reg := &application.RegisterDeviceCommandHandler{Devices: devices, Now: time.Now}
	dto, err := reg.Handle(context.Background(), application.RegisterDeviceCommand{
		SpaceID: "space-1", Name: "Sensor",
		Thresholds: []application.ThresholdInput{{MetricType: "temperature", MinValue: 10, MaxValue: 30, Unit: "°C"}},
	})
	if err != nil || dto.Active || len(dto.Thresholds) != 1 {
		t.Fatalf("register: %v %+v", err, dto)
	}
	set := &application.SetDeviceActiveCommandHandler{Devices: devices}
	if d, err := set.Activate(context.Background(), application.ActivateDeviceCommand{DeviceID: dto.ID}); err != nil || !d.Active {
		t.Fatalf("activate: %v %+v", err, d)
	}
	if _, err := reg.Handle(context.Background(), application.RegisterDeviceCommand{SpaceID: "s", Name: "x",
		Thresholds: []application.ThresholdInput{{MetricType: "temperature", MinValue: 30, MaxValue: 10}}}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad threshold: %v", err)
	}
}
