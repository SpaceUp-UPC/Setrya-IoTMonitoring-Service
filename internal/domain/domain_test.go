package domain

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func activeDevice(t *testing.T) *IoTDevice {
	t.Helper()
	d, err := NewIoTDevice("space-1", "Kitchen sensor", nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	d.Activate()
	return d
}

func TestThresholdRange(t *testing.T) {
	if _, err := NewThresholdRange(10, 10, "°C"); !errors.Is(err, ErrInvalid) {
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
}

func TestEvaluateSeverity(t *testing.T) {
	p := NewAlertPolicyService()
	tr := ThresholdRange{MinValue: 0, MaxValue: 100}
	cases := []struct {
		v    float64
		want SeverityLevel
	}{{105, SeverityLow}, {120, SeverityMedium}, {140, SeverityHigh}, {160, SeverityCritical}, {-60, SeverityCritical}}
	for _, c := range cases {
		if got := p.EvaluateSeverity(MetricTemperature, c.v, tr); got != c.want {
			t.Errorf("value %v: got %s want %s", c.v, got, c.want)
		}
	}
}

func TestValidateReading(t *testing.T) {
	svc := NewMonitoringDomainService(NewAlertPolicyService())
	d := activeDevice(t)
	r, _ := NewReading(d.ID, MetricTemperature, 20, "°C", t0)

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

func TestDetectOutOfRangeUsesDeviceThresholdThenDefault(t *testing.T) {
	svc := NewMonitoringDomainService(NewAlertPolicyService())
	d := activeDevice(t)
	hot, _ := NewReading(d.ID, MetricTemperature, 40, "°C", t0)

	if _, out := svc.DetectOutOfRange(d, hot); !out {
		t.Fatal("40°C should be out of default range 15-35")
	}
	d.Thresholds[MetricTemperature] = ThresholdRange{MinValue: 0, MaxValue: 50, Unit: "°C"}
	if _, out := svc.DetectOutOfRange(d, hot); out {
		t.Fatal("40°C should be inside custom range 0-50")
	}
}

func TestAlertLifecycle(t *testing.T) {
	d := activeDevice(t)
	r, _ := NewReading(d.ID, MetricTemperature, 50, "°C", t0)
	a := NewAlert(r, d, SeverityHigh, "msg", t0)

	if a.Status != AlertCreated || a.SpaceID != "space-1" {
		t.Fatalf("unexpected new alert %+v", a)
	}
	if err := a.Acknowledge(t0); err != nil || a.Status != AlertAcknowledged || a.AcknowledgedAt == nil {
		t.Fatalf("acknowledge failed: %v %+v", err, a)
	}
	if err := a.Acknowledge(t0); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("double acknowledge: %v", err)
	}
	if err := a.Close(t0); err != nil || a.Status != AlertClosed {
		t.Fatalf("close failed: %v", err)
	}
	if err := a.Close(t0); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("double close: %v", err)
	}
}

func TestNewReadingValidation(t *testing.T) {
	if _, err := NewReading("d", MetricVoltage, 1, "", t0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty unit: %v", err)
	}
	if _, err := ParseMetricType("humidity"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown metric: %v", err)
	}
}
