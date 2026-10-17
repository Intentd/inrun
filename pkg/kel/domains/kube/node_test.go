// pkg/note/kube_node_test.go
package note

import (
	"testing"
)

func makeNode(conditions map[string]string, allocatable map[string]string, taints []string) map[string]interface{} {
	condList := make([]interface{}, 0, len(conditions))
	for k, v := range conditions {
		condList = append(condList, map[string]interface{}{"type": k, "status": v})
	}
	alloc := map[string]interface{}{}
	for k, v := range allocatable {
		alloc[k] = v
	}
	taintList := make([]interface{}, 0, len(taints))
	for _, k := range taints {
		taintList = append(taintList, map[string]interface{}{"key": k, "effect": "NoSchedule"})
	}
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": taintList,
		},
		"status": map[string]interface{}{
			"conditions":  condList,
			"allocatable": alloc,
		},
	}
}

func TestNoteNodeReady(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"no status", map[string]interface{}{}, false},
		{"Ready=True", makeNode(map[string]string{"Ready": "True"}, nil, nil), true},
		{"Ready=False", makeNode(map[string]string{"Ready": "False"}, nil, nil), false},
		{"Ready absent", makeNode(map[string]string{"MemoryPressure": "False"}, nil, nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteNodeReady(tt.obj); got != tt.want {
				t.Errorf("noteNodeReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteNodeAllocatableCPU(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no status", map[string]interface{}{}, ""},
		{"with cpu", makeNode(nil, map[string]string{"cpu": "3920m"}, nil), "3920m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteNodeAllocatableCPU(tt.obj); got != tt.want {
				t.Errorf("noteNodeAllocatableCPU() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteNodeAllocatableMemory(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no status", map[string]interface{}{}, ""},
		{"with memory", makeNode(nil, map[string]string{"memory": "15032020Ki"}, nil), "15032020Ki"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteNodeAllocatableMemory(tt.obj); got != tt.want {
				t.Errorf("noteNodeAllocatableMemory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteNodeCondition(t *testing.T) {
	tests := []struct {
		name     string
		obj      interface{}
		condType string
		want     string
	}{
		{"nil", nil, "Ready", ""},
		{"condition not found", makeNode(map[string]string{"Ready": "True"}, nil, nil), "MemoryPressure", ""},
		{"Ready=True", makeNode(map[string]string{"Ready": "True"}, nil, nil), "Ready", "True"},
		{"MemoryPressure=False", makeNode(map[string]string{"Ready": "True", "MemoryPressure": "False"}, nil, nil), "MemoryPressure", "False"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteNodeCondition(tt.obj, tt.condType); got != tt.want {
				t.Errorf("noteNodeCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteNodeTaints(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no taints", makeNode(nil, nil, nil), ""},
		{"one taint", makeNode(nil, nil, []string{"node.kubernetes.io/not-ready"}), "node.kubernetes.io/not-ready"},
		{"two taints", makeNode(nil, nil, []string{"key1", "key2"}), "key1, key2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteNodeTaints(tt.obj); got != tt.want {
				t.Errorf("noteNodeTaints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func makePodWithNode(nodeName, zone, region, instanceType string) map[string]interface{} {
	return map[string]interface{}{
		"_node": map[string]interface{}{
			"name":         nodeName,
			"zone":         zone,
			"region":       region,
			"instanceType": instanceType,
		},
	}
}

func TestNotePodNodeName(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no enrichment", map[string]interface{}{}, ""},
		{"has node", makePodWithNode("ip-10-0-1-5", "us-east-2a", "us-east-2", "t3.medium"), "ip-10-0-1-5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePodNodeName(tt.obj); got != tt.want {
				t.Errorf("notePodNodeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotePodNodeZone(t *testing.T) {
	obj := makePodWithNode("node", "us-east-2a", "us-east-2", "t3.medium")
	if got := notePodNodeZone(obj); got != "us-east-2a" {
		t.Errorf("notePodNodeZone() = %q, want %q", got, "us-east-2a")
	}
}

func TestNotePodNodeRegion(t *testing.T) {
	obj := makePodWithNode("node", "us-east-2a", "us-east-2", "t3.medium")
	if got := notePodNodeRegion(obj); got != "us-east-2" {
		t.Errorf("notePodNodeRegion() = %q, want %q", got, "us-east-2")
	}
}

func TestNotePodNodeInstanceType(t *testing.T) {
	obj := makePodWithNode("node", "us-east-2a", "us-east-2", "t3.medium")
	if got := notePodNodeInstanceType(obj); got != "t3.medium" {
		t.Errorf("notePodNodeInstanceType() = %q, want %q", got, "t3.medium")
	}
}
