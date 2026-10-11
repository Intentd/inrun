package validate

import (
	"testing"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

var testGVK = schema.GroupVersionKind{Group: "test.io", Version: "v1", Kind: "MyApp"}

func catalogWith(crds map[string]types.CRDEntry) *executor {
	return newCatalogExec(crds)
}

func stubHookFn() func() domain.AnyReconcileHooks {
	return func() domain.AnyReconcileHooks { return nil }
}

func stubRecFn() types.NewReconcilerFunc {
	return func(kubeclient.Interface) domain.Reconciler {
		return nil
	}
}

// crdWithGVK builds a typed (non-dynamic) CRDEntry for tests.
// Setting APITypes.Location makes IsDynamic() return false so the constructor
// and per-target reconciler paths inside AddConstructors() are exercised.
func crdWithGVK(gvk schema.GroupVersionKind) types.CRDEntry {
	return types.CRDEntry{
		GroupVersionKind: gvk,
		APITypes: types.APITypes{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Kind:     gvk.Kind,
			Location: "github.com/test/apis/v1",
		},
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{},
		},
	}
}

func withTargetHookLocation(crd types.CRDEntry, targetName, location string) types.CRDEntry {
	if crd.Serve == nil {
		crd.Serve = &types.ServeConfig{Enabled: true}
	}
	if crd.Serve.Target.Entries == nil {
		crd.Serve.Target.Entries = map[string]*types.ServeTargetConfig{}
	}
	crd.Serve.Target.Entries[targetName] = &types.ServeTargetConfig{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				Hooks: &types.HookDeclaration{Location: location, Function: "New"},
			},
		},
	}
	return crd
}

func withTargetDefaultFalse(crd types.CRDEntry, targetName string) types.CRDEntry {
	if crd.Serve == nil {
		crd.Serve = &types.ServeConfig{Enabled: true}
	}
	if crd.Serve.Target.Entries == nil {
		crd.Serve.Target.Entries = map[string]*types.ServeTargetConfig{}
	}
	crd.Serve.Target.Entries[targetName] = &types.ServeTargetConfig{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				Default:         boolPtr(false),
				ConstructorDecl: &types.ConstructorDeclaration{Location: "github.com/test/rec", Function: "New"},
			},
		},
	}
	return crd
}

// ── addHooks ─────────────────────────────────────────────────────────────────

func TestAddHooks_WiresFactoryWhenRegistered(t *testing.T) {
	fn := stubHookFn()
	types.HookRegistry[testGVK] = fn
	t.Cleanup(func() { delete(types.HookRegistry, testGVK) })

	crd := crdWithGVK(testGVK)
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddHooks(); err != nil {
		t.Fatalf("addHooks returned error: %v", err)
	}
	got := k.k.EnabledCRDs()["myapp"]
	if got.OperatorBox.Reconcile.HookFactory == nil {
		t.Error("expected HookFactory to be set, got nil")
	}
}

func TestAddHooks_NoEntryIsOK(t *testing.T) {
	delete(types.HookRegistry, testGVK)

	crd := crdWithGVK(testGVK)
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddHooks(); err != nil {
		t.Fatalf("addHooks returned unexpected error: %v", err)
	}
	if k.k.EnabledCRDs()["myapp"].OperatorBox.Reconcile.HookFactory != nil {
		t.Error("HookFactory should be nil when no registry entry exists")
	}
}

func TestAddHooks_ErrorWhenTargetSharesBinaryButNotRegistered(t *testing.T) {
	// Target shares the CRD-level hook binary (same location) but no factory is
	// registered. addHooks should error — the shared binary is missing.
	delete(types.HookRegistry, testGVK)

	crd := crdWithGVK(testGVK)
	// Set the CRD-level hook location.
	crd.OperatorBox.Reconcile.Hooks = &types.HookDeclaration{Location: "github.com/test/hooks", Function: "New"}
	// Target declares the same location — sharing the binary.
	crd = withTargetHookLocation(crd, "v2", "github.com/test/hooks")
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddHooks(); err == nil {
		t.Fatal("expected error when target shares binary but factory not registered, got nil")
	}
}

func TestAddHooks_NoErrorWhenTargetHasDistinctBinary(t *testing.T) {
	// Target has a different location → addTargetHooks handles it; addHooks should not error.
	delete(types.HookRegistry, testGVK)

	crd := withTargetHookLocation(crdWithGVK(testGVK), "v2", "github.com/test/v2hooks")
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddHooks(); err != nil {
		t.Fatalf("addHooks should not error for distinct-binary target (addTargetHooks validates): %v", err)
	}
}

func TestAddHooks_SkipsNonDefaultReconcilers(t *testing.T) {
	crd := crdWithGVK(testGVK)
	crd.OperatorBox.Reconcile.Default = boolPtr(false)
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddHooks(); err != nil {
		t.Fatalf("addHooks returned error: %v", err)
	}
}

// ── AddConstructors ────────────────────────────────────────────────────────────

func TestAddConstructors_DefaultReconcileSkipsConstructor(t *testing.T) {
	crd := crdWithGVK(testGVK)
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddConstructors(); err != nil {
		t.Fatalf("AddConstructors returned error: %v", err)
	}
	if k.k.EnabledCRDs()["myapp"].OperatorBox.Reconcile.Constructor != nil {
		t.Error("Constructor should not be set for default reconciler")
	}
}

func TestAddConstructors_WiresConstructorWhenRegistered(t *testing.T) {
	fn := stubRecFn()
	types.ReconcilerRegistry[testGVK] = fn
	t.Cleanup(func() { delete(types.ReconcilerRegistry, testGVK) })

	crd := crdWithGVK(testGVK)
	crd.OperatorBox.Reconcile.Default = boolPtr(false)
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddConstructors(); err != nil {
		t.Fatalf("AddConstructors returned error: %v", err)
	}
	if k.k.EnabledCRDs()["myapp"].OperatorBox.Reconcile.Constructor == nil {
		t.Error("expected Constructor to be set, got nil")
	}
}

func TestAddConstructors_ErrorWhenDefaultFalseAndNotRegistered(t *testing.T) {
	delete(types.ReconcilerRegistry, testGVK)

	crd := crdWithGVK(testGVK)
	crd.OperatorBox.Reconcile.Default = boolPtr(false)
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddConstructors(); err == nil {
		t.Fatal("expected error for missing constructor registration, got nil")
	}
}

// ── addTargetHooks ────────────────────────────────────────────────────────────

func TestAddTargetHooks_WiresFactoryForDistinctBinary(t *testing.T) {
	fn := stubHookFn()
	types.TargetHookRegistry[testGVK] = map[string]func() domain.AnyReconcileHooks{
		"v2": fn,
	}
	t.Cleanup(func() { delete(types.TargetHookRegistry, testGVK) })

	crd := withTargetHookLocation(crdWithGVK(testGVK), "v2", "github.com/test/v2hooks")
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetHooks(); err != nil {
		t.Fatalf("addTargetHooks returned error: %v", err)
	}
	got := k.k.EnabledCRDs()["myapp"]
	if got.TargetHookFactories == nil || got.TargetHookFactories["v2"] == nil {
		t.Error("expected TargetHookFactories[v2] to be set, got nil")
	}
}

func TestAddTargetHooks_SkipsTargetWithSameBinaryAsBase(t *testing.T) {
	// Target location matches CRD-level → no TargetHookRegistry needed.
	crd := crdWithGVK(testGVK)
	crd.OperatorBox.Reconcile.Hooks = &types.HookDeclaration{Location: "github.com/test/hooks"}
	crd = withTargetHookLocation(crd, "v2", "github.com/test/hooks") // same location
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetHooks(); err != nil {
		t.Fatalf("addTargetHooks returned error: %v", err)
	}
	if k.k.EnabledCRDs()["myapp"].TargetHookFactories != nil {
		t.Error("TargetHookFactories should be nil when target shares base binary")
	}
}

func TestAddTargetHooks_ErrorWhenGVKMissingFromRegistry(t *testing.T) {
	delete(types.TargetHookRegistry, testGVK)

	crd := withTargetHookLocation(crdWithGVK(testGVK), "v2", "github.com/test/v2hooks")
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetHooks(); err == nil {
		t.Fatal("expected error when GVK missing from TargetHookRegistry, got nil")
	}
}

func TestAddTargetHooks_ErrorWhenTargetNameMissingFromRegistry(t *testing.T) {
	types.TargetHookRegistry[testGVK] = map[string]func() domain.AnyReconcileHooks{
		"other": stubHookFn(), // registered for a different target
	}
	t.Cleanup(func() { delete(types.TargetHookRegistry, testGVK) })

	crd := withTargetHookLocation(crdWithGVK(testGVK), "v2", "github.com/test/v2hooks")
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetHooks(); err == nil {
		t.Fatal("expected error when target name missing from TargetHookRegistry, got nil")
	}
}

func TestAddTargetHooks_SkipsWhenNoServeTargetEntries(t *testing.T) {
	crd := crdWithGVK(testGVK)
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetHooks(); err != nil {
		t.Fatalf("addTargetHooks returned error for CRD with no serve.target.entries: %v", err)
	}
}

// ── addTargetConstructors ─────────────────────────────────────────────────────

func TestAddTargetConstructors_WiresFactoryForDistinctConstructor(t *testing.T) {
	fn := stubRecFn()
	types.TargetReconcilerRegistry[testGVK] = map[string]types.NewReconcilerFunc{
		"v2": fn,
	}
	t.Cleanup(func() { delete(types.TargetReconcilerRegistry, testGVK) })

	crd := withTargetDefaultFalse(crdWithGVK(testGVK), "v2")
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetConstructors(); err != nil {
		t.Fatalf("addTargetConstructors returned error: %v", err)
	}
	got := k.k.EnabledCRDs()["myapp"]
	if got.TargetReconcilerFactories == nil || got.TargetReconcilerFactories["v2"] == nil {
		t.Error("expected TargetReconcilerFactories[v2] to be set, got nil")
	}
}

func TestAddTargetConstructors_SkipsTargetWithDefaultReconciler(t *testing.T) {
	// target has default: true → no TargetReconcilerRegistry needed
	crd := crdWithGVK(testGVK)
	if crd.Serve == nil {
		crd.Serve = &types.ServeConfig{Enabled: true}
	}
	crd.Serve.Target.Entries = map[string]*types.ServeTargetConfig{
		"v2": {OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{Default: boolPtr(true)}}},
	}
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetConstructors(); err != nil {
		t.Fatalf("addTargetConstructors returned error: %v", err)
	}
	if k.k.EnabledCRDs()["myapp"].TargetReconcilerFactories != nil {
		t.Error("TargetReconcilerFactories should be nil when target uses default reconciler")
	}
}

func TestAddTargetConstructors_ErrorWhenGVKMissingFromRegistry(t *testing.T) {
	delete(types.TargetReconcilerRegistry, testGVK)

	crd := withTargetDefaultFalse(crdWithGVK(testGVK), "v2")
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetConstructors(); err == nil {
		t.Fatal("expected error when GVK missing from TargetReconcilerRegistry, got nil")
	}
}

func TestAddTargetConstructors_ErrorWhenTargetNameMissingFromRegistry(t *testing.T) {
	types.TargetReconcilerRegistry[testGVK] = map[string]types.NewReconcilerFunc{
		"other": stubRecFn(),
	}
	t.Cleanup(func() { delete(types.TargetReconcilerRegistry, testGVK) })

	crd := withTargetDefaultFalse(crdWithGVK(testGVK), "v2")
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetConstructors(); err == nil {
		t.Fatal("expected error when target name missing from TargetReconcilerRegistry, got nil")
	}
}

func TestAddTargetConstructors_SkipsWhenNoServeTargetEntries(t *testing.T) {
	crd := crdWithGVK(testGVK)
	k := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	if err := k.k.AddTargetConstructors(); err != nil {
		t.Fatalf("addTargetConstructors returned error for CRD with no serve.target.entries: %v", err)
	}
}
