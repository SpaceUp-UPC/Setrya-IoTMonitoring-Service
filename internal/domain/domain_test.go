package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func activeDevice(t *testing.T) *IoTDevice {
	t.Helper()
	d, err := NewIoTDevice("", "space-1", "Kitchen sensor", nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	d.Activate()
	return d
}

func reading(t *testing.T, d *IoTDevice, metric MetricType, value float64, state LocalState) *Reading {
	t.Helper()
	r, err := NewReading(d.ID, metric, value, "u", ReadingDetails{LocalState: state}, t0)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestThresholdRange(t *testing.T) {
	if _, err := NewThresholdRange(11, 10, "°C"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
	tr, _ := NewThresholdRange(10, 30, "°C")
	if !tr.Contains(10) || !tr.Contains(30) || tr.Contains(30.1) {
		t.Fatal("Contains boundaries wrong")
	}
	if got := tr.Deviation(40); got != 0.5 {
		t.Fatalf("deviation = %v, want 0.5", got)
	}
	if tr.Deviation(20) != 0 {
		t.Fatal("deviation inside range must be 0")
	}
	// Umbral máximo 0 de las lecturas evento: sin amplitud.
	event, err := NewThresholdRange(0, 0, EventUnit)
	if err != nil || event.Contains(1) || event.Deviation(1) != 1 {
		t.Fatalf("event threshold: err=%v", err)
	}
}

func TestValidateID(t *testing.T) {
	for _, ok := range []string{"space1-temperature", "a", "A_b-9", strings.Repeat("x", 64)} {
		if err := ValidateID("id", ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a/b", "a+b", "#", "-a", "a b", strings.Repeat("x", 65)} {
		if err := ValidateID("id", bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestNewIoTDeviceAgreedID(t *testing.T) {
	d, err := NewIoTDevice("space1-temperature", "space1", "Temp", nil, t0)
	if err != nil || d.ID != "space1-temperature" || d.Active {
		t.Fatalf("got %+v err=%v", d, err)
	}
	if _, err := NewIoTDevice("bad/id", "space1", "Temp", nil, t0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := NewIoTDevice("", "sp ace", "Temp", nil, t0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad space: %v", err)
	}
}

func TestEvaluateSeverity(t *testing.T) {
	p := NewAlertPolicyService()
	d := activeDevice(t)
	tr := ThresholdRange{MinValue: 0, MaxValue: 100}
	cases := []struct {
		metric MetricType
		value  float64
		state  LocalState
		want   SeverityLevel
	}{
		{MetricTemperature, 105, "", SeverityLow},
		{MetricTemperature, 120, "", SeverityMedium},
		{MetricTemperature, 140, "", SeverityHigh},
		{MetricTemperature, 160, "", SeverityCritical},
		{MetricTemperature, -60, "", SeverityCritical},
		// El estado local del Edge manda sobre la desviación.
		{MetricTemperature, 29, LocalStateWarning, SeverityMedium},
		{MetricTemperature, 33, LocalStateCritical, SeverityCritical},
		{MetricIntrusion, 1, "", SeverityHigh},
		{MetricEscalation, 1, "", SeverityCritical},
		{MetricDeviceError, 1, "", SeverityMedium},
	}
	for _, c := range cases {
		if got := p.EvaluateSeverity(reading(t, d, c.metric, c.value, c.state), tr); got != c.want {
			t.Errorf("%s %v %q: got %s want %s", c.metric, c.value, c.state, got, c.want)
		}
	}
}

func TestShouldRaise(t *testing.T) {
	p := NewAlertPolicyService()
	open := []*Alert{{Severity: SeverityMedium, Status: AlertSent}}

	if p.ShouldRaise(MetricTemperature, SeverityMedium, open) {
		t.Error("same severity with an open alert must not raise again")
	}
	if !p.ShouldRaise(MetricTemperature, SeverityCritical, open) {
		t.Error("higher severity must raise")
	}
	if !p.ShouldRaise(MetricTemperature, SeverityLow, nil) {
		t.Error("no open alerts must raise")
	}
	if !p.ShouldRaise(MetricIntrusion, SeverityHigh, []*Alert{{Severity: SeverityHigh, Status: AlertSent}}) {
		t.Error("each intrusion must raise")
	}
	if p.ShouldRaise(MetricDeviceError, SeverityMedium, open) {
		t.Error("device_error with an open alert must not raise again")
	}
}

func TestValidateReading(t *testing.T) {
	svc := NewMonitoringDomainService(NewAlertPolicyService())
	d := activeDevice(t)
	r := reading(t, d, MetricTemperature, 20, "")

	if err := svc.ValidateReading(d, r); err != nil {
		t.Fatal(err)
	}
	if err := svc.ValidateReading(nil, r); !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("got %v", err)
	}
	d.Deactivate()
	if err := svc.ValidateReading(d, r); !errors.Is(err, ErrDeviceInactive) {
		t.Fatalf("got %v", err)
	}
}

func TestDetectOutOfRange(t *testing.T) {
	svc := NewMonitoringDomainService(NewAlertPolicyService())
	d := activeDevice(t)

	if _, out := svc.DetectOutOfRange(d, reading(t, d, MetricTemperature, 50, "")); !out {
		t.Error("50°C should be out of the default range")
	}
	d.Thresholds[MetricTemperature] = ThresholdRange{MinValue: 0, MaxValue: 60, Unit: "°C"}
	if _, out := svc.DetectOutOfRange(d, reading(t, d, MetricTemperature, 50, "")); out {
		t.Error("50°C should be inside the device range 0-60")
	}
	// El Edge ya la marcó WARNING con su perfil: es anómala aunque el umbral de la nube no lo diga.
	if _, out := svc.DetectOutOfRange(d, reading(t, d, MetricTemperature, 29, LocalStateWarning)); !out {
		t.Error("a WARNING reading must be abnormal")
	}
	if _, out := svc.DetectOutOfRange(d, reading(t, d, MetricTemperature, 26, LocalStateNormal)); out {
		t.Error("a NORMAL in-range reading must not be abnormal")
	}
	if _, out := svc.DetectOutOfRange(d, reading(t, d, MetricIntrusion, 1, "")); !out {
		t.Error("an event reading with value 1 must be abnormal")
	}
	if _, out := svc.DetectOutOfRange(d, reading(t, d, MetricHumidity, 50, "")); out {
		t.Error("50% humidity should be in range")
	}
}

func TestCreateAlertMessages(t *testing.T) {
	svc := NewMonitoringDomainService(NewAlertPolicyService())
	d := activeDevice(t)
	event := EventDetails{Reason: "temperature_critical", EventID: "ev-1", EvidenceRef: "evidence/ev-1.png"}

	a := svc.CreateAlert(d, reading(t, d, MetricEscalation, 1, ""), ThresholdRange{}, event, t0)
	if a.Severity != SeverityCritical || a.Metric != MetricEscalation || a.EventID != "ev-1" ||
		a.EvidenceRef != "evidence/ev-1.png" || !strings.Contains(a.Message, "temperature_critical") {
		t.Fatalf("unexpected alert %+v", a)
	}
	long := svc.CreateAlert(d, reading(t, d, MetricIntrusion, 1, ""), ThresholdRange{}, EventDetails{Reason: strings.Repeat("ñ", 400)}, t0)
	if n := len([]rune(long.Message)); n > 255 {
		t.Fatalf("message has %d runes, column allows 255", n)
	}
}

func TestAlertLifecycle(t *testing.T) {
	d := activeDevice(t)
	a := NewAlert(reading(t, d, MetricTemperature, 50, ""), d, SeverityHigh, "msg", EventDetails{}, t0)

	if a.Status != AlertCreated || a.SpaceID != "space-1" || !a.IsPending() {
		t.Fatalf("unexpected new alert %+v", a)
	}
	if err := a.Acknowledge(t0); err != nil || a.Status != AlertAcknowledged || a.AcknowledgedAt == nil || a.IsPending() {
		t.Fatalf("acknowledge failed: %v %+v", err, a)
	}
	if err := a.Acknowledge(t0); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("double acknowledge: %v", err)
	}
	if err := a.Close(t0); err != nil || a.IsOpen() {
		t.Fatalf("close failed: %v", err)
	}
	if err := a.Close(t0); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("double close: %v", err)
	}
}

func TestNewReadingValidation(t *testing.T) {
	if _, err := NewReading("d", MetricVoltage, 1, "", ReadingDetails{}, t0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty unit: %v", err)
	}
	if _, err := NewReading("d", MetricVoltage, 1, "V", ReadingDetails{ID: "bad/id"}, t0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad reading id: %v", err)
	}
	r, err := NewReading("d", MetricVoltage, 1, "V", ReadingDetails{ID: "3f1c2a52-9b1e-4a51-8f43-0d6b6f1d2c11"}, t0)
	if err != nil || r.ID != "3f1c2a52-9b1e-4a51-8f43-0d6b6f1d2c11" {
		t.Fatalf("edge reading id: %v", err)
	}
	if _, err := ParseMetricType("pressure"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown metric: %v", err)
	}
	for _, m := range []string{"humidity", "intrusion", "escalation", "device_error"} {
		if _, err := ParseMetricType(m); err != nil {
			t.Errorf("edge metric %s rejected: %v", m, err)
		}
	}
}
