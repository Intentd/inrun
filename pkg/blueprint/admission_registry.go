// pkg/blueprint/admission_registry.go
package blueprint

import (
	"fmt"
	"sync"

	"github.com/intentd/inrun/pkg/types"
)

// ── AdmissionRegistry interface ───────────────────────────────────────────
//
// The AdmissionRegistry holds the validation and mutation rules for every
// CRD declared in the Blueprint that has such rules. It is built at startup
// from the loaded Blueprint and consulted by the /validate and /mutate
// handlers on every admission request.
//
// Keyed by GVR string (not Kind) to handle the case where the same Kind
// exists in multiple API groups. Format: "group/version/resource"
// For core group: "version/resource" e.g. "v1/pods"

// AdmissionRegistry is the interface used by the health server's admission handlers.
type AdmissionRegistry interface {
	// GetValidationRules returns the validation config for a GVR key.
	// Returns nil when no rules are registered for that resource.
	GetValidationRules(gvrKey string) *types.ValidationConfig

	// GetMutationRules returns the mutation config for a GVR key.
	// Returns nil when no rules are registered for that resource.
	GetMutationRules(gvrKey string) *types.MutationConfig

	// RegisterValidationRules stores validation rules for a GVR key.
	RegisterValidationRules(gvrKey string, cfg *types.ValidationConfig)

	// RegisterMutationRules stores mutation rules for a GVR key.
	RegisterMutationRules(gvrKey string, cfg *types.MutationConfig)

	// ValidationGVRs returns all GVR keys that have validation rules.
	// Used at startup to build the ValidatingWebhookConfiguration rules.
	ValidationGVRs() []GVREntry

	// MutationGVRs returns all GVR keys that have mutation rules.
	// Used at startup to build the MutatingWebhookConfiguration rules.
	MutationGVRs() []GVREntry
}

// GVREntry holds the parsed GVR components for webhook configuration.
// Built from the key during registry population.
type GVREntry struct {
	// Key — the full GVR key string: "group/version/resource" or "version/resource"
	Key string

	// Group — API group. Empty for core group resources.
	Group string

	// Version — API version.
	Version string

	// Resource — plural resource name.
	Resource string

	// Operations — which operations this GVR should be webhoooked for.
	// Comes from AdmissionWebhookConfig.Operations or the default ["CREATE", "UPDATE"]
	Operations []string
}

// InMemoryAdmissionRegistry is the concrete implementation used at runtime.
// Safe for concurrent use — the /validate and /mutate handlers read from it
// concurrently; the Blueprint load writes to it once at startup.
type InMemoryAdmissionRegistry struct {
	mu         sync.RWMutex
	validation map[string]*types.ValidationConfig
	mutation   map[string]*types.MutationConfig
	valGVRs    []GVREntry
	mutGVRs    []GVREntry
}

// NewInMemoryAdmissionRegistry returns an initialised registry.
func NewInMemoryAdmissionRegistry() *InMemoryAdmissionRegistry {
	return &InMemoryAdmissionRegistry{
		validation: make(map[string]*types.ValidationConfig),
		mutation:   make(map[string]*types.MutationConfig),
	}
}

func (c *Blueprint) AdmissionRegistry() AdmissionRegistry {
	return c.admissionRegistry
}

func (r *InMemoryAdmissionRegistry) GetValidationRules(gvrKey string) *types.ValidationConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.validation[gvrKey]
}

func (r *InMemoryAdmissionRegistry) GetMutationRules(gvrKey string) *types.MutationConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mutation[gvrKey]
}

func (r *InMemoryAdmissionRegistry) RegisterValidationRules(gvrKey string, cfg *types.ValidationConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validation[gvrKey] = cfg
}

func (r *InMemoryAdmissionRegistry) RegisterMutationRules(gvrKey string, cfg *types.MutationConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mutation[gvrKey] = cfg
}

func (r *InMemoryAdmissionRegistry) ValidationGVRs() []GVREntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]GVREntry, len(r.valGVRs))
	copy(result, r.valGVRs)
	return result
}

func (r *InMemoryAdmissionRegistry) MutationGVRs() []GVREntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]GVREntry, len(r.mutGVRs))
	copy(result, r.mutGVRs)
	return result
}

// addValidationGVR records a GVREntry for webhook configuration generation.
func (r *InMemoryAdmissionRegistry) addValidationGVR(entry GVREntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.valGVRs = append(r.valGVRs, entry)
}

// addMutationGVR records a GVREntry for webhook configuration generation.
func (r *InMemoryAdmissionRegistry) addMutationGVR(entry GVREntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mutGVRs = append(r.mutGVRs, entry)
}

// AddValidationGVR registers both validation rules and the GVR entry so that
// ValidationGVRs() returns the entry. Used in tests and tooling.
func (r *InMemoryAdmissionRegistry) AddValidationGVR(entry GVREntry, cfg *types.ValidationConfig) {
	r.RegisterValidationRules(entry.Key, cfg)
	r.addValidationGVR(entry)
}

// AddMutationGVR registers both mutation rules and the GVR entry so that
// MutationGVRs() returns the entry. Used in tests and tooling.
func (r *InMemoryAdmissionRegistry) AddMutationGVR(entry GVREntry, cfg *types.MutationConfig) {
	r.RegisterMutationRules(entry.Key, cfg)
	r.addMutationGVR(entry)
}

// ── Registration from Blueprint entries ─────────────────────────────────────

// registerAdmissionRulesFromEntry populates the admission registry from one
// CRD entry. Called during BuildRuntimeBlueprint after all CRD entries
// are loaded and enriched.
//
// Only CRDs with validation or mutation rules declared are registered.
// CRDs with neither are not added to the registry — they will not appear
// in the webhook configurations and no admission calls will be made for them.
func (reg *InMemoryAdmissionRegistry) registerAdmissionRulesFromEntry(entry types.CRDEntry) {
	if entry.APITypes.Plural == "" {
		// Not fully enriched — cannot build a GVR key
		return
	}

	gvrKey := buildGVRKey(entry.APITypes.Group, entry.APITypes.Version, entry.APITypes.Plural)
	webhooks := entry.EffectiveWebhooks()
	ops := webhooks.EffectiveOperations()

	// Register validation rules if declared and webhook is enabled
	if entry.HasValidationRules() && webhooks.WebhookValidationEnabled() {

		reg.RegisterValidationRules(gvrKey, entry.EffectiveValidation())
		reg.addValidationGVR(GVREntry{
			Key:        gvrKey,
			Group:      entry.APITypes.Group,
			Version:    entry.APITypes.Version,
			Resource:   entry.APITypes.Plural,
			Operations: ops,
		})
	}

	// Register mutation rules if declared and webhook is enabled
	if entry.HasMutationRules() && webhooks.WebhookMutationEnabled() {

		reg.RegisterMutationRules(gvrKey, entry.EffectiveMutation())
		reg.addMutationGVR(GVREntry{
			Key:        gvrKey,
			Group:      entry.APITypes.Group,
			Version:    entry.APITypes.Version,
			Resource:   entry.APITypes.Plural,
			Operations: ops,
		})
	}
}

// buildGVRKey constructs the registry key from group, version, and resource.
func buildGVRKey(group, version, resource string) string {
	if group == "" {
		return fmt.Sprintf("%s/%s", version, resource)
	}
	return fmt.Sprintf("%s/%s/%s", group, version, resource)
}
