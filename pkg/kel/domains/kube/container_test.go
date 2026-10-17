// pkg/note/container_test.go
package note

import (
	"testing"
)

// mock object that mimics a Deployment's pod template spec
var testDeployment = map[string]interface{}{
	"spec": map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"image": "nginx:1.21",
						"env": []interface{}{
							map[string]interface{}{
								"name":  "APP_ENV",
								"value": "production",
							},
							map[string]interface{}{
								"name":  "LOG_LEVEL",
								"value": "debug",
							},
						},
						"ports": []interface{}{
							map[string]interface{}{
								"name":          "http",
								"containerPort": 8080,
							},
							map[string]interface{}{
								"name":          "metrics",
								"containerPort": 9090,
							},
						},
					},
					map[string]interface{}{
						"image": "sidecar:latest",
						"env": []interface{}{
							map[string]interface{}{"name": "PROXY_PORT", "value": "3128"},
						},
						"ports": []interface{}{
							map[string]interface{}{
								"containerPort": 3128,
							},
						},
					},
				},
			},
		},
	},
}

func TestNoteContainerImage(t *testing.T) {
	tests := []struct {
		name  string
		obj   interface{}
		index int
		want  string
	}{
		{"valid index 0", testDeployment, 0, "nginx:1.21"},
		{"valid index 1", testDeployment, 1, "sidecar:latest"},
		{"out of range", testDeployment, 2, ""},
		{"negative index", testDeployment, -1, ""},
		{"nil object", nil, 0, ""},
		{"no containers", map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{},
				},
			},
		}, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteContainerImage(tt.obj, tt.index); got != tt.want {
				t.Errorf("noteContainerImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteContainerEnv(t *testing.T) {
	tests := []struct {
		name  string
		obj   interface{}
		index int
		key   string
		want  string
	}{
		{"found env", testDeployment, 0, "APP_ENV", "production"},
		{"found env second container", testDeployment, 1, "PROXY_PORT", "3128"},
		{"missing env key", testDeployment, 0, "MISSING", ""},
		{"out of range index", testDeployment, 2, "APP_ENV", ""},
		{"nil object", nil, 0, "APP_ENV", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteContainerEnv(tt.obj, tt.key, tt.index); got != tt.want {
				t.Errorf("noteContainerEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteHasContainerPort(t *testing.T) {
	tests := []struct {
		name         string
		obj          interface{}
		containerIdx int
		port         int
		want         bool
	}{
		{"existing port", testDeployment, 0, 8080, true},
		{"existing second port", testDeployment, 0, 9090, true},
		{"non-existent port", testDeployment, 0, 9999, false},
		{"second container port", testDeployment, 1, 3128, true},
		{"out of range container index", testDeployment, 2, 8080, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHasContainerPort(tt.obj, tt.containerIdx, tt.port); got != tt.want {
				t.Errorf("noteHasContainerPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteContainerPort(t *testing.T) {
	tests := []struct {
		name         string
		obj          interface{}
		containerIdx int
		portIdx      int // optional – we test both with and without
		want         int
	}{
		// No
		// Variadic: no optional index → first port
		{"first port, implicit index", testDeployment, 0, -1, 8080},
		// Explicit index 0 → first port
		{"first port, explicit index 0", testDeployment, 0, 0, 8080},
		// Second port (index 1)
		{"second port", testDeployment, 0, 1, 9090},
		// Second container, first port (portIdx omitted → first)
		{"second container, first port", testDeployment, 1, -1, 3128},
		// Non‑existent container index
		{"container out of range", testDeployment, 2, -1, 0},
		// Port index out of range
		{"port index out of range", testDeployment, 0, 5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int
			if tt.portIdx == -1 {
				// no optional argument – use the variadic version without second int
				got = noteContainerPort(tt.obj, tt.containerIdx)
			} else {
				got = noteContainerPort(tt.obj, tt.containerIdx, tt.portIdx)
			}
			if got != tt.want {
				t.Errorf("noteContainerPort() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNoteContainerPortByName(t *testing.T) {
	tests := []struct {
		name     string
		obj      interface{}
		index    int
		portName string
		want     int
	}{
		{"http port", testDeployment, 0, "http", 8080},
		{"metrics port", testDeployment, 0, "metrics", 9090},
		{"missing name", testDeployment, 0, "unknown", 0},
		{"second container unnamed port", testDeployment, 1, "", 0}, // no name, returns 0
		{"out of range index", testDeployment, 2, "http", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteContainerPortByName(tt.obj, tt.index, tt.portName); got != tt.want {
				t.Errorf("noteContainerPortByName() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNoteContainerPorts(t *testing.T) {
	tests := []struct {
		name  string
		obj   interface{}
		index int
		want  string
	}{
		{"first container ports", testDeployment, 0, "8080,9090"},
		{"second container ports", testDeployment, 1, "3128"},
		{"out of range", testDeployment, 2, ""},
		{"no ports", map[string]interface{}{}, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteContainerPorts(tt.obj, tt.index); got != tt.want {
				t.Errorf("noteContainerPorts() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteContainerCount(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{"two containers", testDeployment, 2},
		{"no containers", map[string]interface{}{}, 0},
		{"nil object", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteContainerCount(tt.obj); got != tt.want {
				t.Errorf("noteContainerCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
