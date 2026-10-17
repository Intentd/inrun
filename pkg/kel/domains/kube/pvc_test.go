// pkg/note/kube_pvc_test.go
package note

import (
	"testing"
)

func makePVC(phase, storageClass, capacity string, accessModes []interface{}) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"storageClassName": storageClass,
			"accessModes":      accessModes,
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{
					"storage": capacity,
				},
			},
		},
		"status": map[string]interface{}{
			"phase": phase,
		},
	}
}

func TestNotePVCBound(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"no status", map[string]interface{}{}, false},
		{"phase Pending", makePVC("Pending", "standard", "10Gi", nil), false},
		{"phase Bound", makePVC("Bound", "standard", "10Gi", nil), true},
		{"phase Lost", makePVC("Lost", "standard", "10Gi", nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePVCBound(tt.obj); got != tt.want {
				t.Errorf("notePVCBound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotePVCPhase(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no status", map[string]interface{}{}, ""},
		{"Bound", makePVC("Bound", "standard", "10Gi", nil), "Bound"},
		{"Pending", makePVC("Pending", "standard", "10Gi", nil), "Pending"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePVCPhase(tt.obj); got != tt.want {
				t.Errorf("notePVCPhase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotePVCCapacity(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no spec", map[string]interface{}{}, ""},
		{"10Gi", makePVC("Bound", "standard", "10Gi", nil), "10Gi"},
		{"100Gi", makePVC("Bound", "standard", "100Gi", nil), "100Gi"},
		{"no resources", map[string]interface{}{"spec": map[string]interface{}{}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePVCCapacity(tt.obj); got != tt.want {
				t.Errorf("notePVCCapacity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotePVCStorageClass(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no spec", map[string]interface{}{}, ""},
		{"standard", makePVC("Bound", "standard", "10Gi", nil), "standard"},
		{"gp2", makePVC("Bound", "gp2", "10Gi", nil), "gp2"},
		{"empty storageClass", makePVC("Bound", "", "10Gi", nil), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePVCStorageClass(tt.obj); got != tt.want {
				t.Errorf("notePVCStorageClass() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotePVCAccessModes(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no spec", map[string]interface{}{}, ""},
		{"single mode", makePVC("Bound", "standard", "10Gi", []interface{}{"ReadWriteOnce"}), "ReadWriteOnce"},
		{"two modes", makePVC("Bound", "standard", "10Gi", []interface{}{"ReadWriteOnce", "ReadOnlyMany"}), "ReadWriteOnce, ReadOnlyMany"},
		{"empty modes", makePVC("Bound", "standard", "10Gi", []interface{}{}), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePVCAccessModes(tt.obj); got != tt.want {
				t.Errorf("notePVCAccessModes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotePVCProvisioner(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no _pv", makePVC("Bound", "standard", "10Gi", nil), ""},
		{"pv with provisioner annotation", map[string]interface{}{
			"_pv": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"pv.kubernetes.io/provisioned-by": "ebs.csi.aws.com",
					},
				},
			},
		}, "ebs.csi.aws.com"},
		{"pv without annotation", map[string]interface{}{
			"_pv": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
				},
			},
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePVCProvisioner(tt.obj); got != tt.want {
				t.Errorf("notePVCProvisioner() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotePVCVolumeMode(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no _pv", makePVC("Bound", "standard", "10Gi", nil), ""},
		{"Filesystem", map[string]interface{}{
			"_pv": map[string]interface{}{
				"spec": map[string]interface{}{
					"volumeMode": "Filesystem",
				},
			},
		}, "Filesystem"},
		{"Block", map[string]interface{}{
			"_pv": map[string]interface{}{
				"spec": map[string]interface{}{
					"volumeMode": "Block",
				},
			},
		}, "Block"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePVCVolumeMode(tt.obj); got != tt.want {
				t.Errorf("notePVCVolumeMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func makePVCWithStorageClass(provisioner, reclaimPolicy string) map[string]interface{} {
	return map[string]interface{}{
		"_storageClass": map[string]interface{}{
			"provisioner": provisioner,
			"spec": map[string]interface{}{
				"reclaimPolicy": reclaimPolicy,
			},
		},
	}
}

func TestNotePVCStorageClassProvisioner(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no enrichment", map[string]interface{}{}, ""},
		{"has storageclass", makePVCWithStorageClass("ebs.csi.aws.com", "Delete"), "ebs.csi.aws.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePVCStorageClassProvisioner(tt.obj); got != tt.want {
				t.Errorf("notePVCStorageClassProvisioner() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotePVCStorageClassReclaimPolicy(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no enrichment", map[string]interface{}{}, ""},
		{"Delete policy", makePVCWithStorageClass("ebs.csi.aws.com", "Delete"), "Delete"},
		{"Retain policy", makePVCWithStorageClass("ebs.csi.aws.com", "Retain"), "Retain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notePVCStorageClassReclaimPolicy(tt.obj); got != tt.want {
				t.Errorf("notePVCStorageClassReclaimPolicy() = %q, want %q", got, tt.want)
			}
		})
	}
}
