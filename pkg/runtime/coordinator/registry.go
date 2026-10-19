// pkg/runtime/coordinator/registry.go
package coordinator

import (
	"strings"
	"sync"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type ResourceBlueprint struct {
	mu      sync.Mutex
	entries map[string]contract.RegistryEntry
}

// compile time check
var _ contract.RuntimeResourceBlueprint = (*ResourceBlueprint)(nil)

func NewCoordinatorRegistry() *ResourceBlueprint {
	return &ResourceBlueprint{
		entries: make(map[string]contract.RegistryEntry),
	}
}

func (r *ResourceBlueprint) Register(
	gvk string,
	crd types.CRDEntry,
	inf cache.SharedIndexInformer,
	rec func() domain.Reconciler,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[gvk] = contract.RegistryEntry{
		CRD:               crd,
		Informer:          inf,
		ReconcilerFactory: rec,
	}
}

func (r *ResourceBlueprint) Unregister(gvk string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.entries, gvk)
}

func (r *ResourceBlueprint) Get(gvk string) (contract.RegistryEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[gvk]
	return entry, ok
}

func (r *ResourceBlueprint) ListGVKs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var gvkList []string
	for gvk := range r.entries {
		gvkList = append(gvkList, gvk)
	}
	return gvkList
}

func (r *ResourceBlueprint) GetWorkers(gvk string, defaultWorkers int) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[gvk]
	if !ok {
		return defaultWorkers
	}
	return entry.CRD.SetWorkers(defaultWorkers)
}

func (r *ResourceBlueprint) Entries() map[string]contract.RegistryEntry {
	return r.entries
}

// GetInformerByName returns the SharedIndexInformer for a CRD by its lowercase
// name (the spec.crds map key: "pipeline", "database", "website").
//
// The registry stores entries keyed by GVK string
// (e.g. "demo.inrun.dev/v1alpha1, Kind=Pipeline"). This method searches
// by Kind name match (case-insensitive) so callers use simple names.
//
// Returns nil, false when no CRD with that name is registered.
func (r *ResourceBlueprint) GetInformerByName(name string) (cache.SharedIndexInformer, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	nameLower := strings.ToLower(name)

	for _, entry := range r.entries {
		if strings.ToLower(entry.CRD.Name) == nameLower {
			return entry.Informer, true
		}
	}
	return nil, false
}

// GetInformerByLabelSelector returns the SharedIndexInformer for a CRD whose
// metadata.labelSelector contain the given key/value pair.
//
// This enables cross‑CRD observation by semantic grouping rather than
// by CRD name. Platform teams can labelSelector CRDs (e.g. "tier=platform",
// "domain=payments") and application‑level logic can reference them
// without knowing the exact CRD name.
//
// Lookup rules:
//   - Match is case‑insensitive on both key and value
//   - Returns the first CRD whose labelSelector contain key=value
//   - Returns nil, false when no CRD matches
//
// This is used by generic.Reconciler via the BlueprintRegistry interface
// to support cross‑context reads without importing pkg/coordinator
// directly (avoiding import cycles).
func (r *ResourceBlueprint) GetInformerByLabelSelector(key, value string) (cache.SharedIndexInformer, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	keyLower := strings.ToLower(key)
	valueLower := strings.ToLower(value)

	for _, entry := range r.entries {
		labels := entry.CRD.LabelSelector
		if labels == nil {
			continue // no labels
		}

		for k, v := range labels {
			if strings.ToLower(k) == keyLower && strings.ToLower(v) == valueLower {
				return entry.Informer, true
			}
		}
	}

	return nil, false
}

// GetCrossAccessByName returns the CrossAccess field of the named CRD.
// nil means readable (default). *false means the CRD has opted out of cross reads.
func (r *ResourceBlueprint) GetCrossAccessByName(name string) *bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	nameLower := strings.ToLower(name)
	for _, entry := range r.entries {
		if strings.ToLower(entry.CRD.Name) == nameLower {
			return entry.CRD.CrossAccess
		}
	}
	return nil
}
