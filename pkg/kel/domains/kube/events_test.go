// pkg/note/kube_events_test.go
package note

import (
	"testing"
)

func makeWarning(reason, message string, count int64) map[string]interface{} {
	return map[string]interface{}{
		"reason":        reason,
		"message":       message,
		"count":         count,
		"lastTimestamp": "2026-05-19T10:00:00Z",
	}
}

func TestNoteHasWarnings(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"no _warnings", map[string]interface{}{}, false},
		{"empty _warnings", map[string]interface{}{"_warnings": []interface{}{}}, false},
		{"one warning", map[string]interface{}{
			"_warnings": []interface{}{makeWarning("BackOff", "Back-off restarting", 3)},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHasWarnings(tt.obj); got != tt.want {
				t.Errorf("noteHasWarnings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteWarningCount(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{"nil", nil, 0},
		{"no _warnings", map[string]interface{}{}, 0},
		{"empty _warnings", map[string]interface{}{"_warnings": []interface{}{}}, 0},
		{"one warning", map[string]interface{}{
			"_warnings": []interface{}{makeWarning("BackOff", "Back-off restarting", 3)},
		}, 1},
		{"two warnings", map[string]interface{}{
			"_warnings": []interface{}{
				makeWarning("BackOff", "Back-off restarting", 3),
				makeWarning("Unhealthy", "Liveness probe failed", 1),
			},
		}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteWarningCount(tt.obj); got != tt.want {
				t.Errorf("noteWarningCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteFirstWarning(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no _warnings", map[string]interface{}{}, ""},
		{"empty _warnings", map[string]interface{}{"_warnings": []interface{}{}}, ""},
		{"one warning", map[string]interface{}{
			"_warnings": []interface{}{makeWarning("BackOff", "Back-off restarting failed container", 3)},
		}, "Back-off restarting failed container"},
		{"returns first of two", map[string]interface{}{
			"_warnings": []interface{}{
				makeWarning("BackOff", "Back-off restarting failed container", 3),
				makeWarning("Unhealthy", "Liveness probe failed", 1),
			},
		}, "Back-off restarting failed container"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteFirstWarning(tt.obj); got != tt.want {
				t.Errorf("noteFirstWarning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteWarningMessages(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no _warnings", map[string]interface{}{}, ""},
		{"one warning", map[string]interface{}{
			"_warnings": []interface{}{makeWarning("BackOff", "Back-off restarting failed container", 3)},
		}, "Back-off restarting failed container"},
		{"two warnings", map[string]interface{}{
			"_warnings": []interface{}{
				makeWarning("BackOff", "Back-off restarting failed container", 3),
				makeWarning("Unhealthy", "Liveness probe failed", 1),
			},
		}, "Back-off restarting failed container, Liveness probe failed"},
		{"empty message skipped", map[string]interface{}{
			"_warnings": []interface{}{
				map[string]interface{}{"reason": "BackOff", "message": "", "count": int64(1), "lastTimestamp": ""},
				makeWarning("Unhealthy", "Liveness probe failed", 1),
			},
		}, "Liveness probe failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteWarningMessages(tt.obj); got != tt.want {
				t.Errorf("noteWarningMessages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteWarningReasons(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no _warnings", map[string]interface{}{}, ""},
		{"one warning", map[string]interface{}{
			"_warnings": []interface{}{makeWarning("BackOff", "Back-off restarting failed container", 3)},
		}, "BackOff"},
		{"two warnings", map[string]interface{}{
			"_warnings": []interface{}{
				makeWarning("BackOff", "Back-off restarting failed container", 3),
				makeWarning("Unhealthy", "Liveness probe failed", 1),
			},
		}, "BackOff, Unhealthy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteWarningReasons(tt.obj); got != tt.want {
				t.Errorf("noteWarningReasons() = %v, want %v", got, tt.want)
			}
		})
	}
}
