package intent

import (
	"github.com/intentd/tools/kube"
	"github.com/intentd/tools/structured"
)

var validateK8sName = kube.ValidKubernetesName

// mapContains is a nil-safe map membership check.
// Wraps structured.MapContains for local convenience.
func mapContains[V any](m map[string]V, key string) bool {
	return structured.MapContains(m, key)
}
