package intent

import "github.com/intentd/inrun/pkg/utils"

var (
	validateK8sName = utils.ValidKubernetesName
	isNestedPath    = utils.IsNestedPath
	setNestedPath   = utils.SetNestedPath
)

// mapContains is a nil-safe map membership check.
// Wraps utils.MapContains for local convenience.
func mapContains[V any](m map[string]V, key string) bool {
	return utils.MapContains(m, key)
}
