package funcs

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCronToMapTemplate(t *testing.T) {
	expr := "*/5 0 * * 1"
	got, err := cronToMapTemplate(expr)
	if err != nil {
		t.Fatalf("cronToMapTemplate() error = %v", err)
	}
	if !strings.HasPrefix(got, jsonValueSentinel) {
		t.Errorf("cronToMapTemplate() missing sentinel prefix: %q", got)
	}
	jsonPart := strings.TrimPrefix(got, jsonValueSentinel)
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonPart), &m); err != nil {
		t.Errorf("cronToMapTemplate() JSON unmarshal error: %v", err)
	}
	if m["minute"] != "*/5" {
		t.Errorf("minute = %v, want */5", m["minute"])
	}
}
