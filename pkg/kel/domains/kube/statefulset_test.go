// pkg/note/kube_statefulset_test.go
package note

import "testing"

func makeStatefulSetWithRevisions(current, update string) map[string]interface{} {
	return map[string]interface{}{
		"status": map[string]interface{}{
			"currentRevision": current,
			"updateRevision":  update,
		},
	}
}

func makeStatefulSetWithPVCs(count int) map[string]interface{} {
	pvcs := make([]interface{}, count)
	for i := range pvcs {
		pvcs[i] = map[string]interface{}{"metadata": map[string]interface{}{"name": "pvc"}}
	}
	return map[string]interface{}{"_pvcs": pvcs}
}

func TestNoteStatefulSetCurrentRevision(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no status", map[string]interface{}{}, ""},
		{"has revision", makeStatefulSetWithRevisions("my-sts-abc123", "my-sts-abc123"), "my-sts-abc123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteStatefulSetCurrentRevision(tt.obj); got != tt.want {
				t.Errorf("noteStatefulSetCurrentRevision() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteStatefulSetUpdateRevision(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no status", map[string]interface{}{}, ""},
		{"pending update", makeStatefulSetWithRevisions("my-sts-old", "my-sts-new"), "my-sts-new"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteStatefulSetUpdateRevision(tt.obj); got != tt.want {
				t.Errorf("noteStatefulSetUpdateRevision() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteStatefulSetPVCCount(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{"nil", nil, 0},
		{"no enrichment", map[string]interface{}{}, 0},
		{"three pvcs", makeStatefulSetWithPVCs(3), 3},
		{"one pvc", makeStatefulSetWithPVCs(1), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteStatefulSetPVCCount(tt.obj); got != tt.want {
				t.Errorf("noteStatefulSetPVCCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
