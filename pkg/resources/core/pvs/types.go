// pkg/resources/pvs/types.go
package pvs

// Spec is the fully resolved PersistentVolume specification.
// PersistentVolumes are cluster-scoped — Namespace is not used.
type Spec struct {
	Name             string
	StorageClassName string
	Capacity         string
	AccessModes      []string
	ReclaimPolicy    string
	HostPath         string
	CSIDriver        string
	CSIVolumeHandle  string
	Labels           map[string]string
}
