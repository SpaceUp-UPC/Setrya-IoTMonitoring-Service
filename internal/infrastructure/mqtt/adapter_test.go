package mqtt

import "testing"

func TestParsePayload(t *testing.T) {
	cmd, err := ParsePayload([]byte(`{"deviceId":"d1","metricType":"temperature","value":0,"unit":"C"}`))
	if err != nil || cmd.DeviceID != "d1" || cmd.Value != 0 {
		t.Fatalf("got %+v err=%v", cmd, err)
	}
	for _, bad := range []string{`not json`, `{"deviceId":"d1","metricType":"temperature","unit":"C"}`} {
		if _, err := ParsePayload([]byte(bad)); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}
