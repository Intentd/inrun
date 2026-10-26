// pkg/blueprint/conversion_registry.go
package blueprint

import (
	"sync"

	"github.com/intentd/inrun/pkg/types"
)

// ConversionRegistry is the interface used by the health server's /convert handler.
// Decoupled from the Blueprint struct so the health server has no import cycle.
type ConversionRegistry interface {
	GetConversionRules(kind string) *types.ConversionRules
	RegisterConversionRules(rules *types.ConversionRules)
}

// InMemoryConversionRegistry holds per-Kind conversion rules.
// Safe for concurrent use — the /convert endpoint reads from multiple goroutines
// and Blueprint load writes once at startup.
type InMemoryConversionRegistry struct {
	mu    sync.RWMutex
	rules map[string]*types.ConversionRules
}

// NewInMemoryConversionRegistry returns an initialised registry.
func NewInMemoryConversionRegistry() *InMemoryConversionRegistry {
	return &InMemoryConversionRegistry{
		rules: make(map[string]*types.ConversionRules),
	}
}

// GetConversionRules returns the rules for a given Kind.
// Returns nil when no rules are registered for that Kind.
func (r *InMemoryConversionRegistry) GetConversionRules(kind string) *types.ConversionRules {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rules[kind]
}

// RegisterConversionRules stores rules for the Kind declared in rules.Kind.
// Called once per CRD entry during Blueprint load.
func (r *InMemoryConversionRegistry) RegisterConversionRules(rules *types.ConversionRules) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[rules.Kind] = rules
}

// registerConversionRulesFromSpec builds ConversionRules from a CRDEntry
// and registers them in the registry.
//
// Called from BuildRuntimeBlueprint for every CRD entry that declares
// a conversion block. Only the CRD with storageVersion declared does this —
// other versions of the same CRD don't need conversion rules registered
// because conversion is always expressed relative to the storage version.
func (reg *InMemoryConversionRegistry) registerConversionRulesFromSpec(entry types.CRDEntry) {
	if entry.EffectiveConversion() == nil || entry.IsConversionParticipant() {
		return
	}

	rules := &types.ConversionRules{
		Kind:           entry.APITypes.Kind,
		StorageVersion: entry.EffectiveConversion().StorageVersion,
		Paths:          entry.EffectiveConversion().Paths,
	}

	reg.RegisterConversionRules(rules)
}

func (c *Blueprint) ConversionRegistry() ConversionRegistry {
	return c.conversionRegistry
}

// ManagedConversionCRDs returns the names (plural.group) of the enabled CRDs
// whose spec.conversion Inrun owns: the gateway points each at /convert with
// the generated certificate's CA, and the housekeeper keeps it so.
func (c *Blueprint) ManagedConversionCRDs() []string {
	var names []string
	for _, crd := range c.EnabledCRDs() {
		if crd.ShouldManageCRD() {
			names = append(names, crd.APITypes.Plural+"."+crd.APITypes.Group)
		}
	}
	return names
}

// Test exports
func NewInMemoryRegistryForTest() *InMemoryConversionRegistry {
	return &InMemoryConversionRegistry{
		rules: make(map[string]*types.ConversionRules),
	}
}
