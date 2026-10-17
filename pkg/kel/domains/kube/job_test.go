// pkg/note/kube_job_test.go
package note

import (
	"testing"
)

func TestNoteJobSucceeded(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"no status", map[string]interface{}{}, false},
		{"status without succeeded", map[string]interface{}{
			"status": map[string]interface{}{},
		}, false},
		{"succeeded = 0", map[string]interface{}{
			"status": map[string]interface{}{
				"succeeded": int64(0),
			},
		}, false},
		{"succeeded = 1", map[string]interface{}{
			"status": map[string]interface{}{
				"succeeded": int64(1),
			},
		}, true},
		{"succeeded = 2", map[string]interface{}{
			"status": map[string]interface{}{
				"succeeded": int64(2),
			},
		}, true},
		{"succeeded as float64", map[string]interface{}{
			"status": map[string]interface{}{
				"succeeded": float64(1),
			},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteJobSucceeded(tt.obj); got != tt.want {
				t.Errorf("noteJobSucceeded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteJobFailed(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"no status", map[string]interface{}{}, false},
		{"status without failed", map[string]interface{}{
			"status": map[string]interface{}{},
		}, false},
		{"failed = 0", map[string]interface{}{
			"status": map[string]interface{}{
				"failed": int64(0),
			},
		}, false},
		{"failed = 1", map[string]interface{}{
			"status": map[string]interface{}{
				"failed": int64(1),
			},
		}, true},
		{"failed = 2", map[string]interface{}{
			"status": map[string]interface{}{
				"failed": int64(2),
			},
		}, true},
		{"failed as float64", map[string]interface{}{
			"status": map[string]interface{}{
				"failed": float64(1),
			},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteJobFailed(tt.obj); got != tt.want {
				t.Errorf("noteJobFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteJobActive(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"no status", map[string]interface{}{}, false},
		{"status without active", map[string]interface{}{
			"status": map[string]interface{}{},
		}, false},
		{"active = 0", map[string]interface{}{
			"status": map[string]interface{}{
				"active": int64(0),
			},
		}, false},
		{"active = 1", map[string]interface{}{
			"status": map[string]interface{}{
				"active": int64(1),
			},
		}, true},
		{"active = 2", map[string]interface{}{
			"status": map[string]interface{}{
				"active": int64(2),
			},
		}, true},
		{"active as float64", map[string]interface{}{
			"status": map[string]interface{}{
				"active": float64(1),
			},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteJobActive(tt.obj); got != tt.want {
				t.Errorf("noteJobActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteJobFirstExitCode(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int64
	}{
		{"nil", nil, -1},
		{"no _pods", map[string]interface{}{}, -1},
		{"empty _pods", map[string]interface{}{"_pods": []interface{}{}}, -1},
		{"pod not terminated (exitCode -1)", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Running", "exitCode": int64(-1)},
			},
		}, -1},
		{"pod succeeded exitCode 0", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Succeeded", "exitCode": int64(0)},
			},
		}, 0},
		{"pod failed exitCode 1", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Failed", "exitCode": int64(1)},
			},
		}, 1},
		{"returns first terminated among multiple", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Running", "exitCode": int64(-1)},
				map[string]interface{}{"name": "job-def", "phase": "Succeeded", "exitCode": int64(0)},
			},
		}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteJobFirstExitCode(tt.obj); got != tt.want {
				t.Errorf("noteJobFirstExitCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteJobActivePodNames(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no _pods", map[string]interface{}{}, ""},
		{"all succeeded", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Succeeded"},
			},
		}, ""},
		{"one running", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Running"},
			},
		}, "job-abc"},
		{"mixed", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Running"},
				map[string]interface{}{"name": "job-def", "phase": "Succeeded"},
				map[string]interface{}{"name": "job-xyz", "phase": "Pending"},
			},
		}, "job-abc, job-xyz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteJobActivePodNames(tt.obj); got != tt.want {
				t.Errorf("noteJobActivePodNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteJobSucceededPodNames(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"none succeeded", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Running"},
			},
		}, ""},
		{"one succeeded", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Succeeded"},
				map[string]interface{}{"name": "job-def", "phase": "Failed"},
			},
		}, "job-abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteJobSucceededPodNames(tt.obj); got != tt.want {
				t.Errorf("noteJobSucceededPodNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteJobFailedPodNames(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"none failed", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Succeeded"},
			},
		}, ""},
		{"one failed", map[string]interface{}{
			"_pods": []interface{}{
				map[string]interface{}{"name": "job-abc", "phase": "Succeeded"},
				map[string]interface{}{"name": "job-def", "phase": "Failed"},
			},
		}, "job-def"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteJobFailedPodNames(tt.obj); got != tt.want {
				t.Errorf("noteJobFailedPodNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteCronJobActiveCount(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{"nil", nil, 0},
		{"no status", map[string]interface{}{}, 0},
		{"status without active", map[string]interface{}{
			"status": map[string]interface{}{},
		}, 0},
		{"two active", map[string]interface{}{
			"status": map[string]interface{}{
				"active": []interface{}{
					map[string]interface{}{"name": "job-1"},
					map[string]interface{}{"name": "job-2"},
				},
			},
		}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteCronJobActiveCount(tt.obj); got != tt.want {
				t.Errorf("noteCronJobActiveCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteCronJobLastScheduleTime(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no status", map[string]interface{}{}, ""},
		{"no lastScheduleTime", map[string]interface{}{
			"status": map[string]interface{}{},
		}, ""},
		{"has lastScheduleTime", map[string]interface{}{
			"status": map[string]interface{}{
				"lastScheduleTime": "2026-05-19T10:00:00Z",
			},
		}, "2026-05-19T10:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteCronJobLastScheduleTime(tt.obj); got != tt.want {
				t.Errorf("noteCronJobLastScheduleTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteCronJobLastSuccessTime(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no status", map[string]interface{}{}, ""},
		{"no lastSuccessfulTime", map[string]interface{}{
			"status": map[string]interface{}{},
		}, ""},
		{"has lastSuccessfulTime", map[string]interface{}{
			"status": map[string]interface{}{
				"lastSuccessfulTime": "2026-05-19T10:00:00Z",
			},
		}, "2026-05-19T10:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteCronJobLastSuccessTime(tt.obj); got != tt.want {
				t.Errorf("noteCronJobLastSuccessTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func makeCronJobWithEnrichment(lastJobName string, lastJobSucceeded bool, lastSuccessfulJobName string) map[string]interface{} {
	lastJob := map[string]interface{}{
		"metadata": map[string]interface{}{"name": lastJobName},
		"status":   map[string]interface{}{"succeeded": int64(0)},
	}
	if lastJobSucceeded {
		lastJob["status"] = map[string]interface{}{"succeeded": int64(1)}
	}
	obj := map[string]interface{}{
		"_lastJob": lastJob,
	}
	if lastSuccessfulJobName != "" {
		obj["_lastSuccessfulJob"] = map[string]interface{}{
			"metadata": map[string]interface{}{"name": lastSuccessfulJobName},
			"status":   map[string]interface{}{"succeeded": int64(1)},
		}
	}
	return obj
}

func TestNoteCronJobLastJobName(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no enrichment", map[string]interface{}{}, ""},
		{"has last job", makeCronJobWithEnrichment("my-job-123", false, ""), "my-job-123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteCronJobLastJobName(tt.obj); got != tt.want {
				t.Errorf("noteCronJobLastJobName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteCronJobLastJobSucceeded(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"no enrichment", map[string]interface{}{}, false},
		{"last job failed", makeCronJobWithEnrichment("my-job-123", false, ""), false},
		{"last job succeeded", makeCronJobWithEnrichment("my-job-123", true, ""), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteCronJobLastJobSucceeded(tt.obj); got != tt.want {
				t.Errorf("noteCronJobLastJobSucceeded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteCronJobLastSuccessfulJobName(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no enrichment", map[string]interface{}{}, ""},
		{"has successful job", makeCronJobWithEnrichment("my-job-200", false, "my-job-100"), "my-job-100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteCronJobLastSuccessfulJobName(tt.obj); got != tt.want {
				t.Errorf("noteCronJobLastSuccessfulJobName() = %q, want %q", got, tt.want)
			}
		})
	}
}
