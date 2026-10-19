// pkg/resources/pvcs/types.go
package pvcs

// Spec is the fully resolved PersistentVolumeClaim specification.
type Spec struct {
	Name             string
	Namespace        string
	StorageClassName string
	AccessModes      []string
	Storage          string
	VolumeMode       string
	VolumeName       string
	Labels           map[string]string
}
