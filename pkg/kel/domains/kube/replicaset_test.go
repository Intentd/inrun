// pkg/note/kube_replicaset_test.go
package note

import "testing"

func makeReplicaSetWithOwner(ownerName, ownerKind string) map[string]interface{} {
	return map[string]interface{}{
		"_owner": map[string]interface{}{
			"name": ownerName,
			"kind": ownerKind,
			"uid":  "abc-123",
		},
	}
}

func makeDeploymentWithReplicaSets(count int) map[string]interface{} {
	rs := make([]interface{}, count)
	for i := range rs {
		rs[i] = map[string]interface{}{"metadata": map[string]interface{}{"name": "rs"}}
	}
	return map[string]interface{}{"_replicaSets": rs}
}

func TestNoteReplicaSetOwnerName(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no owner", map[string]interface{}{}, ""},
		{"has owner", makeReplicaSetWithOwner("my-app", "Deployment"), "my-app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteReplicaSetOwnerName(tt.obj); got != tt.want {
				t.Errorf("noteReplicaSetOwnerName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteReplicaSetOwnerKind(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no owner", map[string]interface{}{}, ""},
		{"has owner", makeReplicaSetWithOwner("my-app", "Deployment"), "Deployment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteReplicaSetOwnerKind(tt.obj); got != tt.want {
				t.Errorf("noteReplicaSetOwnerKind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteDeploymentReplicaSetCount(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{"nil", nil, 0},
		{"no enrichment", map[string]interface{}{}, 0},
		{"two replica sets", makeDeploymentWithReplicaSets(2), 2},
		{"one replica set", makeDeploymentWithReplicaSets(1), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteDeploymentReplicaSetCount(tt.obj); got != tt.want {
				t.Errorf("noteDeploymentReplicaSetCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
