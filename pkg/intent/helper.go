package intent

import (
	"github.com/intentd/inrun/pkg/util/kube"
	"github.com/intentd/inrun/pkg/util/structured"
)

var (
	validateK8sName = kube.ValidKubernetesName
	isNestedPath    = structured.IsNestedPath
	setNestedPath   = structured.SetNestedPath
)

// mapContains is a nil-safe map membership check.
// Wraps structured.MapContains for local convenience.
func mapContains[V any](m map[string]V, key string) bool {
	return structured.MapContains(m, key)
}
