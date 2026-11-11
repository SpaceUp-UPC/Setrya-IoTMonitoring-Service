package mqtt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/application"
	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/internal/domain"
)

func TestParsePayload(t *testing.T) {
	cmd, err := ParsePayload([]byte(`{"deviceId":"d1","metricType":"temperature","value":0,"unit":"C"}`))
	if err != nil || cmd.DeviceID != "d1" || cmd.Value != 0 || cmd.MeasuredAt != nil {
		t.Fatalf("got %+v err=%v", cmd, err)
	}
	for _, bad := range []string{`not json`, `{"deviceId":"d1","metricType":"temperature","unit":"C"}`} {
		if _, err := ParsePayload([]byte(bad)); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

// Payload tal como lo arma el Edge (edge/communication/events.py).
func TestParsePayloadEdgeEvent(t *testing.T) {
	raw := `{"deviceId":"space1-intrusion","metricType":"intrusion","value":1,"unit":"event",
		"readingId":"0b0e7a4e-7b1c-4e0e-9f43-0d6b6f1d2c11","measuredAt":"2026-11-09T05:39:45Z",
		"localState":"CRITICAL","reason":"unknown_person","eventId":"ev-1","evidenceRef":"evidence/ev-1.png",
		"campoFuturo":true}`
	cmd, err := ParsePayload([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 9, 5, 39, 45, 0, time.UTC)
	if cmd.ReadingID != "0b0e7a4e-7b1c-4e0e-9f43-0d6b6f1d2c11" || cmd.MeasuredAt == nil || !cmd.MeasuredAt.Equal(want) ||
		cmd.LocalState != "CRITICAL" || cmd.Reason != "unknown_person" || cmd.EventID != "ev-1" || cmd.EvidenceRef != "evidence/ev-1.png" {
		t.Fatalf("got %+v", cmd)
	}
	// Una fecha mal formada no descarta la lectura.
	cmd, err = ParsePayload([]byte(`{"deviceId":"d","metricType":"humidity","value":50,"unit":"%","measuredAt":"ayer"}`))
	if err != nil || cmd.MeasuredAt != nil {
		t.Fatalf("got %+v err=%v", cmd, err)
	}
}

func TestTopics(t *testing.T) {
	if got := CommandTopic("space1", "valve"); got != "sentrya/space1/valve/commands" {
		t.Error(got)
	}
	if got := ValidationTopic("space1"); got != "sentrya/space1/security/validation" {
		t.Error(got)
	}
	if got := ConfigTopic("space1"); got != "sentrya/space1/config" {
		t.Error(got)
	}
}

func TestPublishWithoutConnection(t *testing.T) {
	ctx := context.Background()
	notStarted := NewAdapter(Config{}, nil, nil)
	for name, p := range map[string]application.EdgePublisher{"not started": notStarted, "disabled": DisabledPublisher{}} {
		if err := p.PublishCommand(ctx, "s", "valve", application.EdgeCommand{Action: "open"}); !errors.Is(err, domain.ErrBrokerUnavailable) {
			t.Errorf("%s command: %v", name, err)
		}
		if err := p.PublishValidation(ctx, "s", "ev", "confirm"); !errors.Is(err, domain.ErrBrokerUnavailable) {
			t.Errorf("%s validation: %v", name, err)
		}
		if err := p.PublishConfig(ctx, "s", []byte(`{}`)); !errors.Is(err, domain.ErrBrokerUnavailable) {
			t.Errorf("%s config: %v", name, err)
		}
	}
}
