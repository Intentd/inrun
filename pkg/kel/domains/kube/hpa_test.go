// pkg/note/kube_hpa_test.go
package note

import (
	"testing"
)

func makeHPA(current, desired, min, max int64) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"minReplicas": min,
			"maxReplicas": max,
		},
		"status": map[string]interface{}{
			"currentReplicas": current,
			"desiredReplicas": desired,
		},
	}
}

func makeHPAScaling(current, desired, min, max int64) map[string]interface{} {
	hpa := makeHPA(current, desired, min, max)
	status := hpa["status"].(map[string]interface{})
	status["conditions"] = []interface{}{
		map[string]interface{}{
			"type":   "ScalingActive",
			"status": "True",
		},
	}
	return hpa
}

func TestNoteHPACurrentReplicas(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int64
	}{
		{"nil", nil, 0},
		{"no status", map[string]interface{}{}, 0},
		{"three replicas", makeHPA(3, 3, 2, 10), 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHPACurrentReplicas(tt.obj); got != tt.want {
				t.Errorf("noteHPACurrentReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteHPADesiredReplicas(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int64
	}{
		{"nil", nil, 0},
		{"no status", map[string]interface{}{}, 0},
		{"five desired", makeHPA(3, 5, 2, 10), 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHPADesiredReplicas(tt.obj); got != tt.want {
				t.Errorf("noteHPADesiredReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteHPAMinReplicas(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int64
	}{
		{"nil", nil, 1},
		{"no spec", map[string]interface{}{}, 1},
		{"min 2", makeHPA(3, 3, 2, 10), 2},
		{"min 0 defaults to 1", makeHPA(3, 3, 0, 10), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHPAMinReplicas(tt.obj); got != tt.want {
				t.Errorf("noteHPAMinReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteHPAMaxReplicas(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int64
	}{
		{"nil", nil, 0},
		{"no spec", map[string]interface{}{}, 0},
		{"max 10", makeHPA(3, 3, 2, 10), 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHPAMaxReplicas(tt.obj); got != tt.want {
				t.Errorf("noteHPAMaxReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteHPAScaling(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"stable current==desired", makeHPAScaling(3, 3, 2, 10), false},
		{"scaling up current<desired", makeHPAScaling(3, 5, 2, 10), true},
		{"scaling down current>desired", makeHPAScaling(5, 3, 2, 10), true},
		{"metric unknown desiredReplicas=0", makeHPA(1, 0, 1, 10), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHPAScaling(tt.obj); got != tt.want {
				t.Errorf("noteHPAScaling() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteHPAAtMax(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"below max", makeHPA(3, 3, 2, 10), false},
		{"at max", makeHPA(10, 10, 2, 10), true},
		{"above max (edge)", makeHPA(11, 11, 2, 10), true},
		{"max 0 (not set)", makeHPA(3, 3, 2, 0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHPAAtMax(tt.obj); got != tt.want {
				t.Errorf("noteHPAAtMax() = %v, want %v", got, tt.want)
			}
		})
	}
}

func makeHPAWithEnrichment(scaleTargetName, scaleTargetKind string, metricTypes []string) map[string]interface{} {
	metrics := make([]interface{}, len(metricTypes))
	for i, t := range metricTypes {
		metrics[i] = map[string]interface{}{"type": t, "name": "cpu"}
	}
	return map[string]interface{}{
		"_scaleTarget": map[string]interface{}{
			"name": scaleTargetName,
			"kind": scaleTargetKind,
		},
		"_currentMetrics": metrics,
	}
}

func TestNoteHPAScaleTargetName(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no enrichment", map[string]interface{}{}, ""},
		{"has target", makeHPAWithEnrichment("my-app", "Deployment", nil), "my-app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHPAScaleTargetName(tt.obj); got != tt.want {
				t.Errorf("noteHPAScaleTargetName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteHPAScaleTargetKind(t *testing.T) {
	obj := makeHPAWithEnrichment("my-app", "Deployment", nil)
	if got := noteHPAScaleTargetKind(obj); got != "Deployment" {
		t.Errorf("noteHPAScaleTargetKind() = %q, want %q", got, "Deployment")
	}
}

func TestNoteHPAMetricTypes(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no enrichment", map[string]interface{}{}, ""},
		{"one metric", makeHPAWithEnrichment("", "", []string{"Resource"}), "Resource"},
		{"two metrics", makeHPAWithEnrichment("", "", []string{"Resource", "External"}), "Resource, External"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHPAMetricTypes(tt.obj); got != tt.want {
				t.Errorf("noteHPAMetricTypes() = %q, want %q", got, tt.want)
			}
		})
	}
}
