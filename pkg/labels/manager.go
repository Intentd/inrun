package labels

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
)

// Manager handles label and annotation mutations on domain objects.
// It is configuration-driven and stateless between calls.
type Manager struct {
	// standalone indicates that no runtime reconciler is present.
	// When true, the manager will NOT add finalizers (they require a controller).
	standalone bool

	// deletionProtectionEnabled controls whether the deletion‑protection label
	// is applied. This should reflect the global security setting from Blueprint.
	deletionProtectionEnabled bool

	deletionProtectionLabel string
	managedLabel            string
	managedByAnnotation     string
	managedSinceAnnotation  string
	strictModeExemptLabel   string
}

// Config holds the configuration for a [Manager].
type Config struct {
	// Standalone disables finalizer management. Set this when there is no
	// runtime reconciler to process finalizers (e.g., gateway-only mode).
	Standalone bool

	// DeletionProtectionEnabled mirrors the global
	// security.deletionProtection.enabled setting from the Blueprint. When true,
	// [Manager.EnsureDeletionProtectionLabel] may add the protection label.
	DeletionProtectionEnabled bool
}

// NewManager constructs a Manager with the given configuration.
// All label and annotation key constants are resolved at construction time,
// so callers pay that cost once per reconcile cycle rather than per call.
func NewManager(cfg Config) *Manager {
	mgr := &Manager{
		standalone:                cfg.Standalone,
		deletionProtectionEnabled: cfg.DeletionProtectionEnabled,
	}

	mgr.deletionProtectionLabel = DeletionProtectionLabel
	mgr.strictModeExemptLabel = StrictModeExemptKey
	mgr.managedLabel = ManagedKey
	mgr.managedByAnnotation = AnnotationManagedBy
	mgr.managedSinceAnnotation = AnnotationManagedSince
	return mgr
}

// EnsureManagedLabel adds the standard Inrun ownership label to obj if it
// is missing or has the wrong value.
//
// Label applied: inrun.dev/managed = "true"
//
// Returns true if the label was absent and has been added; false if it was
// already present with the correct value (no mutation occurred).
//
// The caller must persist any change via kube.PatchLabels.
func (m *Manager) EnsureManagedLabel(obj domain.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	if v, ok := labels[ManagedKey]; ok && v == ManagedValue {
		return false
	}
	labels[ManagedKey] = ManagedValue
	obj.SetLabels(labels)
	return true
}

// EnsureManagedAnnotations adds the standard Inrun management annotations
// to obj if they are absent or empty.
//
// Annotations applied:
//   - inrun.dev/managed-by: <operatorName>
//   - inrun.dev/managed-since: <UTC RFC 3339 timestamp>
//
// Existing values are never overwritten — the managed-since timestamp records
// the first time Inrun took ownership and must not drift on subsequent
// reconciles.
//
// Returns true if any annotation was added; false if both were already present.
// The caller must persist any change via kube.PatchAnnotations.
func (m *Manager) EnsureManagedAnnotations(obj domain.Object, operatorName string) bool {
	changed := false
	ann := obj.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	if v, ok := ann[AnnotationManagedBy]; !ok || v == "" {
		ann[AnnotationManagedBy] = operatorName
		changed = true
	}
	if v, ok := ann[AnnotationManagedSince]; !ok || v == "" {
		ann[AnnotationManagedSince] = time.Now().UTC().Format(time.RFC3339)
		changed = true
	}
	if changed {
		obj.SetAnnotations(ann)
	}
	return changed
}

// EnsureDeletionProtectionLabel reconciles the deletion-protection label on obj
// to match the desired state expressed by shouldHave.
//
//   - shouldHave = true  → label is added:   inrun.dev/deletion-protection = "true"
//   - shouldHave = false → label is removed:  key is deleted from the label map
//
// The typical caller logic for shouldHave is:
//
//	shouldHave = blueprint.IsDeletionProtectionEnabled() && crd.ShouldProtectCRs()
//
// Returns true if the label map was modified; false if it was already in the
// desired state. The caller must persist any change via kube.PatchLabels.
//
// Note: removing this label while strict mode is active will be blocked by the
// strict-mode admission webhook. See [EnsureStrictModeExemptLabel] for how the
// reconciler handles the two-phase transition safely.
func (m *Manager) EnsureDeletionProtectionLabel(obj domain.Object, shouldHave bool) bool {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	desiredValue := ""
	if shouldHave {
		desiredValue = DeletionProtectionValue
	}

	currentValue := labels[DeletionProtectionLabel]
	if currentValue == desiredValue {
		return false
	}

	if desiredValue == "" {
		delete(labels, DeletionProtectionLabel)
	} else {
		labels[DeletionProtectionLabel] = desiredValue
	}
	obj.SetLabels(labels)
	return true
}

// EnsureStrictModeExemptLabel reconciles the strict-mode exemption label on
// obj to match the effective strict-mode state for this CRD.
//
//   - strictModeEnabled = true  → label is removed: key is deleted from the label map
//   - strictModeEnabled = false → label is added:   inrun.dev/strict-mode-exempt = "true"
//
// The exemption label signals to the strict-mode admission webhook that this
// resource is allowed to have its deletion-protection label removed. It is
// added when a CRD declares strictMode: false in its per-CRD deletionProtection
// override, overriding the global strictMode setting.
//
// Two-phase removal: when the reconciler is transitioning a resource from
// protected to unprotected (removing deletion-protection), it must keep the
// exemption label present in the same patch so the webhook allows the UPDATE.
// Once deletion-protection is gone the webhook objectSelector no longer matches
// and the exemption label can be removed freely on the next cycle. The
// generic.Reconciler handles this automatically — callers of this method should
// not need to think about it.
//
// Returns true if the label map was modified; false if it was already correct.
// The caller must persist any change via kube.PatchLabels.
func (m *Manager) EnsureStrictModeExemptLabel(obj domain.Object, strictModeEnabled bool) bool {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	desiredValue := ""
	if !strictModeEnabled {
		desiredValue = StrictModeExemptValue
	}

	currentValue := labels[StrictModeExemptKey]
	if currentValue == desiredValue {
		logger.Debug().
			Str("resource", obj.GetName()).
			Bool("strictModeEnabled", strictModeEnabled).
			Str("current", currentValue).
			Str("desired", desiredValue).
			Msg("label: exemption label already correct")
		return false
	}

	if desiredValue == "" {
		delete(labels, StrictModeExemptKey)
		logger.Debug().
			Str("resource", obj.GetName()).
			Msg("label: removing exemption label")
	} else {
		labels[StrictModeExemptKey] = desiredValue
		logger.Debug().
			Str("resource", obj.GetName()).
			Msg("label: adding exemption label")
	}
	obj.SetLabels(labels)
	return true
}

// EnsureUserLabels merges user-defined labels onto obj.
// Values are expected to have been resolved from templates before this call.
// Existing keys set by the user are overwritten; keys absent from extra are
// left untouched. Returns true if any label was added or changed.
//
// The caller must persist any change via kube.PatchLabels.
func (m *Manager) EnsureUserLabels(obj domain.Object, extra map[string]string) bool {
	if len(extra) == 0 {
		return false
	}
	lbls := obj.GetLabels()
	if lbls == nil {
		lbls = make(map[string]string, len(extra))
	}
	changed := false
	for k, v := range extra {
		if lbls[k] != v {
			lbls[k] = v
			changed = true
		}
	}
	if changed {
		obj.SetLabels(lbls)
	}
	return changed
}

// EnsureUserAnnotations merges user-defined annotations onto obj.
// Values are expected to have been resolved from templates before this call.
// Existing keys set by the user are overwritten; keys absent from extra are
// left untouched. Returns true if any label was added or changed.
//
// The caller must persist any change via kube.PatchAnnotations.
func (m *Manager) EnsureUserAnnotations(obj domain.Object, extra map[string]string) bool {
	if len(extra) == 0 {
		return false
	}
	ann := obj.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string, len(extra))
	}
	changed := false
	for k, v := range extra {
		if ann[k] != v {
			ann[k] = v
			changed = true
		}
	}
	if changed {
		obj.SetAnnotations(ann)
	}
	return changed
}

// CheckReserved rejects keys in the runtime's domain (inrun.dev/… and <sub>.inrun.dev/…).
func CheckReserved(field string, m map[string]string) error {
	var bad []string
	for k := range m {
		prefix, _, ok := strings.Cut(k, "/")
		if !ok {
			continue
		}
		if prefix == ReservedDomain || strings.HasSuffix(prefix, "."+ReservedDomain) {
			bad = append(bad, k)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	return fmt.Errorf("%s %s: the %s domain is reserved for the runtime", field, strings.Join(bad, ", "), ReservedDomain)
}

// ── Getters ───────────────────────────────────────────────────────────────────

func (m *Manager) IsStandalone() bool {
	return m.standalone
}

func (m *Manager) IsDeletionProtectionEnabled() bool {
	return m.deletionProtectionEnabled
}

func (m *Manager) GetDeletionProtectionLabel() string {
	return m.deletionProtectionLabel
}

func (m *Manager) GetStrictModeExemptLabel() string {
	return m.strictModeExemptLabel
}

func (m *Manager) GetManagedLabel() string {
	return m.managedLabel
}

func (m *Manager) GetManagedByAnnotation() string {
	return m.managedByAnnotation
}

func (m *Manager) GetManagedSinceAnnotation() string {
	return m.managedSinceAnnotation
}

// ── Finalizer helpers ─────────────────────────────────────────────────────────
// Stateless helpers for in-memory finalizer manipulation on any domain.Object.
// Safe to import from any layer — no API calls, no configuration required.

func ContainsFinalizer(o domain.Object, finalizer string) bool {
	for _, f := range o.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

func AddFinalizer(o domain.Object, finalizer string) (updated bool) {
	if ContainsFinalizer(o, finalizer) {
		return false
	}
	o.SetFinalizers(append(o.GetFinalizers(), finalizer))
	return true
}

func RemoveFinalizer(o domain.Object, finalizer string) (updated bool) {
	cur := o.GetFinalizers()
	n := 0
	for i := range cur {
		if cur[i] != finalizer {
			cur[n] = cur[i]
			n++
		}
	}
	o.SetFinalizers(cur[:n])
	return n != len(cur)
}

// EnsureFinalizers adds any finalizers from want that are not already present
// on obj. Returns true when the object was mutated and needs a patch.
func EnsureFinalizers(o domain.Object, want []string) (updated bool) {
	for _, f := range want {
		if AddFinalizer(o, f) {
			updated = true
		}
	}
	return updated
}

// StripFinalizers removes all finalizers in strip from obj.
// Returns true when the object was mutated and needs a patch.
func StripFinalizers(o domain.Object, strip []string) (updated bool) {
	for _, f := range strip {
		if RemoveFinalizer(o, f) {
			updated = true
		}
	}
	return updated
}
