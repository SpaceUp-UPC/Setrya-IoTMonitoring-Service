package application_test

import (
	"context"
	"encoding/json"
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
func (r *memReadings) FindByID(_ context.Context, id string) (*domain.Reading, error) {
	for _, rd := range r.list {
		if rd.ID == id {
			return rd, nil
		}
	}
	return nil, domain.ErrReadingNotFound
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
func (r *memAlerts) FindOpenByDevice(_ context.Context, deviceID string, metric domain.MetricType) ([]*domain.Alert, error) {
	var out []*domain.Alert
	for _, a := range r.m {
		if a.DeviceID == deviceID && a.Metric == metric && a.IsOpen() {
			out = append(out, a)
		}
	}
	return out, nil
}
func (r *memAlerts) FindByReading(_ context.Context, readingID string) (*domain.Alert, error) {
	for _, a := range r.m {
		if a.ReadingID == readingID {
			return a, nil
		}
	}
	return nil, nil
}

type fakeNotifier struct {
	calls int
	err   error
}

func (n *fakeNotifier) Notify(context.Context, *domain.Alert, *domain.IoTDevice) error {
	n.calls++
	return n.err
}

type published struct {
	kind, spaceID, target, eventID, decision string
	cmd                                      application.EdgeCommand
	config                                   json.RawMessage
}

type fakePublisher struct {
	sent []published
	err  error
}

func (p *fakePublisher) PublishCommand(_ context.Context, spaceID, target string, cmd application.EdgeCommand) error {
	if p.err == nil {
		p.sent = append(p.sent, published{kind: "command", spaceID: spaceID, target: target, cmd: cmd})
	}
	return p.err
}
func (p *fakePublisher) PublishValidation(_ context.Context, spaceID, eventID, decision string) error {
	if p.err == nil {
		p.sent = append(p.sent, published{kind: "validation", spaceID: spaceID, eventID: eventID, decision: decision})
	}
	return p.err
}
func (p *fakePublisher) PublishConfig(_ context.Context, spaceID string, config json.RawMessage) error {
	if p.err == nil {
		p.sent = append(p.sent, published{kind: "config", spaceID: spaceID, config: config})
	}
	return p.err
}

var testNow = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

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
	d, _ := domain.NewIoTDevice("space1-sensor", "space-1", "Kitchen sensor", nil, testNow)
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
		Now: func() time.Time { return testNow }, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return e
}

func (e *env) send(t *testing.T, cmd application.IngestReadingCommand) application.IngestResult {
	t.Helper()
	cmd.DeviceID = e.deviceID
	res, err := e.ingest.Handle(context.Background(), cmd)
	if err != nil {
		t.Fatalf("ingest %+v: %v", cmd, err)
	}
	return res
}

func TestIngestInRangeCreatesNoAlert(t *testing.T) {
	e := newEnv(t, true)
	res := e.send(t, application.IngestReadingCommand{MetricType: "temperature", Value: 22, Unit: "C"})
	if res.Alert != nil || res.Reading.Status != "validated" || len(e.alerts.m) != 0 || e.notifier.calls != 0 {
		t.Fatalf("unexpected result %+v", res)
	}
}

func TestIngestOutOfRangeCreatesAndSendsAlert(t *testing.T) {
	e := newEnv(t, true)
	// 120°F = 48.89°C, sobre el umbral por defecto de 45°C.
	res := e.send(t, application.IngestReadingCommand{MetricType: "Temperature", Value: 120, Unit: "F"})
	if res.Reading.Status != "out_of_range" || res.Reading.Unit != "°C" || res.Alert == nil {
		t.Fatalf("unexpected result %+v", res)
	}
	if e.notifier.calls != 1 || e.alerts.m[res.Alert.ID].Status != domain.AlertSent {
		t.Fatalf("notifier calls=%d status=%s", e.notifier.calls, e.alerts.m[res.Alert.ID].Status)
	}
}

// El Edge muestrea cada pocos segundos mientras dura la condición: no debe
// haber una alerta (ni un correo) por lectura.
func TestIngestDoesNotRepeatAlertWhileConditionLasts(t *testing.T) {
	e := newEnv(t, true)
	warn := application.IngestReadingCommand{MetricType: "temperature", Value: 29, Unit: "C", LocalState: "WARNING"}

	first := e.send(t, warn)
	if first.Alert == nil || first.Alert.Severity != "medium" {
		t.Fatalf("first WARNING must raise a medium alert: %+v", first)
	}
	if again := e.send(t, warn); again.Alert != nil || again.Reading.Status != "out_of_range" {
		t.Fatalf("repeated WARNING must not raise: %+v", again)
	}
	crit := e.send(t, application.IngestReadingCommand{MetricType: "temperature", Value: 33, Unit: "C", LocalState: "CRITICAL"})
	if crit.Alert == nil || crit.Alert.Severity != "critical" {
		t.Fatalf("escalating to CRITICAL must raise: %+v", crit)
	}
	if len(e.alerts.m) != 2 || e.notifier.calls != 2 {
		t.Fatalf("alerts=%d notifications=%d, want 2 and 2", len(e.alerts.m), e.notifier.calls)
	}

	// Tras cerrar las alertas, una nueva condición vuelve a alertar.
	for _, a := range e.alerts.m {
		_ = a.Close(testNow)
	}
	if res := e.send(t, warn); res.Alert == nil {
		t.Fatal("after closing, a new WARNING must raise")
	}
}

func TestIngestEdgeEvents(t *testing.T) {
	e := newEnv(t, true)
	intrusion := e.send(t, application.IngestReadingCommand{
		MetricType: "intrusion", Value: 1, Unit: "event", EventID: "ev-1", EvidenceRef: "evidence/ev-1.png",
	})
	if intrusion.Alert == nil || intrusion.Alert.Severity != "high" || intrusion.Alert.EventID != "ev-1" ||
		intrusion.Alert.EvidenceRef != "evidence/ev-1.png" || intrusion.Alert.MetricType != "intrusion" {
		t.Fatalf("intrusion: %+v", intrusion.Alert)
	}
	// Una segunda intrusión es otro hecho: alerta aunque la anterior siga abierta.
	if second := e.send(t, application.IngestReadingCommand{MetricType: "intrusion", Value: 1, Unit: "event", EventID: "ev-2"}); second.Alert == nil {
		t.Fatal("second intrusion must raise")
	}
	esc := e.send(t, application.IngestReadingCommand{MetricType: "escalation", Value: 1, Unit: "event", Reason: "temperature_critical"})
	if esc.Alert == nil || esc.Alert.Severity != "critical" || esc.Alert.Reason != "temperature_critical" {
		t.Fatalf("escalation: %+v", esc.Alert)
	}
	if derr := e.send(t, application.IngestReadingCommand{MetricType: "device_error", Value: 1, Unit: "event"}); derr.Alert == nil || derr.Alert.Severity != "medium" {
		t.Fatalf("device_error: %+v", derr.Alert)
	}
	hum := e.send(t, application.IngestReadingCommand{MetricType: "humidity", Value: 50, Unit: "%"})
	if hum.Alert != nil || hum.Reading.Unit != "%" {
		t.Fatalf("humidity in range: %+v", hum)
	}
}

// El Edge reenvía desde su outbox (QoS 1): el mismo readingId no debe
// duplicar la lectura ni la alerta.
func TestIngestIsIdempotentByReadingID(t *testing.T) {
	e := newEnv(t, true)
	measured := time.Date(2026, 10, 1, 11, 59, 58, 0, time.UTC)
	cmd := application.IngestReadingCommand{
		MetricType: "intrusion", Value: 1, Unit: "event", EventID: "ev-1",
		ReadingID: "0b0e7a4e-7b1c-4e0e-9f43-0d6b6f1d2c11", MeasuredAt: &measured,
	}
	first := e.send(t, cmd)
	if first.Duplicate || first.Reading.ID != cmd.ReadingID || first.Reading.MeasuredAt == nil || first.Alert == nil {
		t.Fatalf("first: %+v", first)
	}
	second := e.send(t, cmd)
	if !second.Duplicate || second.Alert == nil || second.Alert.ID != first.Alert.ID {
		t.Fatalf("second must return the stored result: %+v", second)
	}
	if len(e.readings.list) != 1 || len(e.alerts.m) != 1 || e.notifier.calls != 1 {
		t.Fatalf("readings=%d alerts=%d notifications=%d", len(e.readings.list), len(e.alerts.m), e.notifier.calls)
	}
}

func TestIngestNotifierFailureKeepsAlertCreated(t *testing.T) {
	e := newEnv(t, true)
	e.notifier.err = errors.New("smtp down")
	res := e.send(t, application.IngestReadingCommand{MetricType: "voltage", Value: 300, Unit: "V"})
	if res.Alert == nil || e.alerts.m[res.Alert.ID].Status != domain.AlertCreated {
		t.Fatalf("alert must stay created: %+v", res)
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
			return application.IngestReadingCommand{DeviceID: id, MetricType: "pressure", Value: 20, Unit: "hPa"}
		}, domain.ErrInvalid},
		{"bad unit", true, func(id string) application.IngestReadingCommand {
			return application.IngestReadingCommand{DeviceID: id, MetricType: "voltage", Value: 220, Unit: "kg"}
		}, domain.ErrInvalid},
		{"event with a measurement unit", true, func(id string) application.IngestReadingCommand {
			return application.IngestReadingCommand{DeviceID: id, MetricType: "intrusion", Value: 1, Unit: "C"}
		}, domain.ErrInvalid},
		{"bad reading id", true, func(id string) application.IngestReadingCommand {
			return application.IngestReadingCommand{DeviceID: id, MetricType: "temperature", Value: 20, Unit: "C", ReadingID: "a/b"}
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
	res := e.send(t, application.IngestReadingCommand{MetricType: "voltage", Value: 300, Unit: "V"})
	ack := &application.AcknowledgeAlertCommandHandler{Alerts: e.alerts, Now: time.Now}
	cl := &application.CloseAlertCommandHandler{Alerts: e.alerts, Now: time.Now}
	ctx := context.Background()

	dto, err := ack.Handle(ctx, application.AcknowledgeAlertCommand{AlertID: res.Alert.ID})
	if err != nil || dto.Status != "acknowledged" {
		t.Fatalf("ack: %v %+v", err, dto)
	}
	if _, err := cl.Handle(ctx, application.CloseAlertCommand{AlertID: res.Alert.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := ack.Handle(ctx, application.AcknowledgeAlertCommand{AlertID: res.Alert.ID}); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("ack closed alert: %v", err)
	}
	if _, err := cl.Handle(ctx, application.CloseAlertCommand{AlertID: "missing"}); !errors.Is(err, domain.ErrAlertNotFound) {
		t.Fatalf("missing alert: %v", err)
	}
}

func TestValidateSecurityAlert(t *testing.T) {
	ctx := context.Background()
	setup := func(t *testing.T) (*env, *fakePublisher, *application.ValidateSecurityAlertCommandHandler, string) {
		e := newEnv(t, true)
		pub := &fakePublisher{}
		h := &application.ValidateSecurityAlertCommandHandler{Alerts: e.alerts, Publisher: pub, Now: func() time.Time { return testNow }}
		res := e.send(t, application.IngestReadingCommand{MetricType: "intrusion", Value: 1, Unit: "event", EventID: "ev-1"})
		return e, pub, h, res.Alert.ID
	}

	t.Run("confirm acknowledges and publishes", func(t *testing.T) {
		_, pub, h, id := setup(t)
		dto, err := h.Handle(ctx, application.ValidateSecurityAlertCommand{AlertID: id, Decision: "confirm"})
		if err != nil || dto.Status != "acknowledged" {
			t.Fatalf("%v %+v", err, dto)
		}
		if len(pub.sent) != 1 || pub.sent[0].spaceID != "space-1" || pub.sent[0].eventID != "ev-1" || pub.sent[0].decision != "confirm" {
			t.Fatalf("published %+v", pub.sent)
		}
		if _, err := h.Handle(ctx, application.ValidateSecurityAlertCommand{AlertID: id, Decision: "reject"}); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("second decision: %v", err)
		}
	})
	t.Run("reject closes", func(t *testing.T) {
		_, _, h, id := setup(t)
		if dto, err := h.Handle(ctx, application.ValidateSecurityAlertCommand{AlertID: id, Decision: "reject"}); err != nil || dto.Status != "closed" {
			t.Fatalf("%v %+v", err, dto)
		}
	})
	t.Run("broker down leaves the alert pending", func(t *testing.T) {
		e, pub, h, id := setup(t)
		pub.err = domain.ErrBrokerUnavailable
		if _, err := h.Handle(ctx, application.ValidateSecurityAlertCommand{AlertID: id, Decision: "confirm"}); !errors.Is(err, domain.ErrBrokerUnavailable) {
			t.Fatalf("got %v", err)
		}
		if !e.alerts.m[id].IsPending() {
			t.Fatal("alert must stay pending so the user can retry")
		}
	})
	t.Run("invalid decision and non-intrusion alert", func(t *testing.T) {
		e, pub, h, id := setup(t)
		if _, err := h.Handle(ctx, application.ValidateSecurityAlertCommand{AlertID: id, Decision: "maybe"}); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("got %v", err)
		}
		temp := e.send(t, application.IngestReadingCommand{MetricType: "temperature", Value: 60, Unit: "C"})
		if _, err := h.Handle(ctx, application.ValidateSecurityAlertCommand{AlertID: temp.Alert.ID, Decision: "confirm"}); !errors.Is(err, domain.ErrNotValidatable) {
			t.Fatalf("got %v", err)
		}
		if len(pub.sent) != 0 {
			t.Fatalf("nothing must be published: %+v", pub.sent)
		}
	})
}

func TestSendEdgeCommandAndConfig(t *testing.T) {
	ctx := context.Background()
	pub := &fakePublisher{}
	send := &application.SendEdgeCommandCommandHandler{Publisher: pub}

	dto, err := send.Handle(ctx, application.SendEdgeCommandCommand{SpaceID: "space1", Target: "valve", Action: " open "})
	if err != nil || dto.CommandID == "" || dto.Action != "open" {
		t.Fatalf("%v %+v", err, dto)
	}
	got := pub.sent[0]
	if got.spaceID != "space1" || got.target != "valve" || got.cmd.Action != "open" || got.cmd.Params == nil || got.cmd.CommandID != dto.CommandID {
		t.Fatalf("published %+v", got)
	}
	// El target y el espacio son segmentos del tópico: sin comodines ni separadores.
	for _, bad := range []application.SendEdgeCommandCommand{
		{SpaceID: "space1", Target: "+", Action: "open"},
		{SpaceID: "space1", Target: "a/b", Action: "open"},
		{SpaceID: "#", Target: "valve", Action: "open"},
		{SpaceID: "space1", Target: "valve", Action: "  "},
	} {
		if _, err := send.Handle(ctx, bad); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%+v: got %v", bad, err)
		}
	}

	cfg := &application.PushEdgeConfigCommandHandler{Publisher: pub}
	if err := cfg.Handle(ctx, application.PushEdgeConfigCommand{SpaceID: "space1", Config: json.RawMessage(`{"temperature":{"t1_warning":28}}`)}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`[]`, `{}`, `"x"`, `not json`} {
		if err := cfg.Handle(ctx, application.PushEdgeConfigCommand{SpaceID: "space1", Config: json.RawMessage(bad)}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("config %s: got %v", bad, err)
		}
	}
	if len(pub.sent) != 2 || pub.sent[1].kind != "config" {
		t.Fatalf("published %+v", pub.sent)
	}
}

func TestQueriesAndSummary(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	e.send(t, application.IngestReadingCommand{MetricType: "temperature", Value: 20, Unit: "C"})
	e.send(t, application.IngestReadingCommand{MetricType: "voltage", Value: 300, Unit: "V"})

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

func TestRegisterDevice(t *testing.T) {
	ctx := context.Background()
	devices := &memDevices{m: map[string]*domain.IoTDevice{}}
	reg := &application.RegisterDeviceCommandHandler{Devices: devices, Now: time.Now}

	dto, err := reg.Handle(ctx, application.RegisterDeviceCommand{
		SpaceID: "space-1", Name: "Sensor",
		Thresholds: []application.ThresholdInput{{MetricType: "temperature", MinValue: 10, MaxValue: 30, Unit: "°C"}},
	})
	if err != nil || dto.Active || dto.ID == "" || len(dto.Thresholds) != 1 {
		t.Fatalf("register: %v %+v", err, dto)
	}
	set := &application.SetDeviceActiveCommandHandler{Devices: devices}
	if d, err := set.Activate(ctx, application.ActivateDeviceCommand{DeviceID: dto.ID}); err != nil || !d.Active {
		t.Fatalf("activate: %v %+v", err, d)
	}

	// Id acordado con el Edge, activo desde el registro y con umbral máximo 0.
	agreed := application.RegisterDeviceCommand{
		ID: "space1-intrusion", SpaceID: "space1", Name: "Intrusion", Active: true,
		Thresholds: []application.ThresholdInput{{MetricType: "intrusion", MinValue: 0, MaxValue: 0, Unit: "event"}},
	}
	if d, err := reg.Handle(ctx, agreed); err != nil || d.ID != "space1-intrusion" || !d.Active {
		t.Fatalf("agreed id: %v %+v", err, d)
	}
	if _, err := reg.Handle(ctx, agreed); !errors.Is(err, domain.ErrDeviceExists) {
		t.Fatalf("repeated id must not overwrite: %v", err)
	}
	if _, err := reg.Handle(ctx, application.RegisterDeviceCommand{SpaceID: "s", Name: "x",
		Thresholds: []application.ThresholdInput{{MetricType: "temperature", MinValue: 30, MaxValue: 10}}}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad threshold: %v", err)
	}
}
