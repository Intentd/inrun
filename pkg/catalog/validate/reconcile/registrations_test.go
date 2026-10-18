package reconcile

import (
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

var testGVK = schema.GroupVersionKind{Group: "test.io", Version: "v1", Kind: "MyApp"}

func catalogWith(crds map[string]types.CRDEntry) *checker {
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
// and per-target reconciler lookups are exercised.
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
				Default:         ptr.To(false),
				ConstructorDecl: &types.ConstructorDeclaration{Location: "github.com/test/rec", Function: "New"},
			},
		},
	}
	return crd
}

// ── Hooks ─────────────────────────────────────────────────────────────────────

func TestHookFactory_FoundWhenRegistered(t *testing.T) {
	types.HookRegistry[testGVK] = stubHookFn()
	t.Cleanup(func() { delete(types.HookRegistry, testGVK) })

	crd := crdWithGVK(testGVK)
	if _, ok := crd.HookFactory(); !ok {
		t.Error("expected the registered hook factory")
	}
	if !crd.HasHooks() {
		t.Error("a registered hook factory counts as hooks")
	}
}

func TestHookFactory_NoEntryIsOK(t *testing.T) {
	delete(types.HookRegistry, testGVK)

	crd := crdWithGVK(testGVK)
	if _, ok := crd.HookFactory(); ok {
		t.Error("no hook factory without a registry entry")
	}
	if err := crd.CheckSharedTargetHooks(); err != nil {
		t.Fatalf("no targets, no error: %v", err)
	}
}

func TestHookFactory_NotForNonDefaultReconcilers(t *testing.T) {
	types.HookRegistry[testGVK] = stubHookFn()
	t.Cleanup(func() { delete(types.HookRegistry, testGVK) })

	crd := crdWithGVK(testGVK)
	crd.OperatorBox.Reconcile.Default = ptr.To(false)
	if _, ok := crd.HookFactory(); ok {
		t.Error("a CRD with its own reconciler runs no hooks")
	}
}

func TestCheckSharedTargetHooks_ErrorWhenTargetSharesBinaryButNotRegistered(t *testing.T) {
	// The target shares the CRD-level hook binary but no factory is registered.
	delete(types.HookRegistry, testGVK)

	crd := crdWithGVK(testGVK)
	crd.OperatorBox.Reconcile.Hooks = &types.HookDeclaration{Location: "github.com/test/hooks", Function: "New"}
	crd = withTargetHookLocation(crd, "v2", "github.com/test/hooks")

	if err := crd.CheckSharedTargetHooks(); err == nil {
		t.Fatal("expected error when target shares binary but factory not registered, got nil")
	}
}

func TestCheckSharedTargetHooks_NoErrorWhenTargetHasDistinctBinary(t *testing.T) {
	// A distinct binary is TargetHookFactories' concern.
	delete(types.HookRegistry, testGVK)

	crd := withTargetHookLocation(crdWithGVK(testGVK), "v2", "github.com/test/v2hooks")
	if err := crd.CheckSharedTargetHooks(); err != nil {
		t.Fatalf("distinct-binary target is checked by TargetHookFactories: %v", err)
	}
}

// ── Constructors ──────────────────────────────────────────────────────────────

func TestConstructor_NotNeededForDefaultReconcile(t *testing.T) {
	crd := crdWithGVK(testGVK)
	if crd.NeedsConstructor() {
		t.Error("a default-reconciled CRD needs no constructor")
	}
	if crd.HasConstructor() {
		t.Error("HasConstructor should be false for the default reconciler")
	}
}

func TestConstructor_FoundWhenRegistered(t *testing.T) {
	types.ReconcilerRegistry[testGVK] = stubRecFn()
	t.Cleanup(func() { delete(types.ReconcilerRegistry, testGVK) })

	crd := crdWithGVK(testGVK)
	crd.OperatorBox.Reconcile.Default = ptr.To(false)
	if _, ok := crd.Constructor(); !ok {
		t.Error("expected the registered constructor")
	}
	if !crd.HasConstructor() {
		t.Error("HasConstructor should be true")
	}
}

func TestValidateRegistrations_ErrorWhenDefaultFalseAndNotRegistered(t *testing.T) {
	delete(types.ReconcilerRegistry, testGVK)

	crd := crdWithGVK(testGVK)
	crd.OperatorBox.Reconcile.Default = ptr.To(false)
	e := catalogWith(map[string]types.CRDEntry{"myapp": crd})

	err := e.validateRegistrations()
	if err == nil {
		t.Fatal("expected error for missing constructor registration, got nil")
	}
	if !strings.Contains(err.Error(), `CRD "myapp": no constructor registered`) {
		t.Errorf("unexpected message: %v", err)
	}
}

// ── Object types ──────────────────────────────────────────────────────────────

func TestRuntimeObjects_DynamicIsUnstructured(t *testing.T) {
	crd := crdWithGVK(testGVK)
	crd.APITypes.Location = ""
	obj, list, err := crd.RuntimeObjects()
	if err != nil {
		t.Fatalf("dynamic CRDs need no registration: %v", err)
	}
	if obj.GetObjectKind().GroupVersionKind().Kind != "MyApp" ||
		list.GetObjectKind().GroupVersionKind().Kind != "MyAppList" {
		t.Errorf("unexpected kinds: %v, %v", obj.GetObjectKind(), list.GetObjectKind())
	}
}

func TestValidateRegistrations_ErrorWhenTypedObjectNotRegistered(t *testing.T) {
	delete(types.ObjectRegistry, testGVK)

	e := catalogWith(map[string]types.CRDEntry{"myapp": crdWithGVK(testGVK)})
	if err := e.validateRegistrations(); err == nil {
		t.Fatal("expected error for a typed CRD with no object type registered")
	}
}

// ── Target hooks ──────────────────────────────────────────────────────────────

func TestTargetHookFactories_FoundForDistinctBinary(t *testing.T) {
	types.TargetHookRegistry[testGVK] = map[string]func() domain.AnyReconcileHooks{
		"v2": stubHookFn(),
	}
	t.Cleanup(func() { delete(types.TargetHookRegistry, testGVK) })

	crd := withTargetHookLocation(crdWithGVK(testGVK), "v2", "github.com/test/v2hooks")
	got, err := crd.TargetHookFactories()
	if err != nil {
		t.Fatalf("TargetHookFactories returned error: %v", err)
	}
	if got["v2"] == nil {
		t.Error("expected a hook factory for v2")
	}
}

func TestTargetHookFactories_SkipsTargetWithSameBinaryAsBase(t *testing.T) {
	crd := crdWithGVK(testGVK)
	crd.OperatorBox.Reconcile.Hooks = &types.HookDeclaration{Location: "github.com/test/hooks"}
	crd = withTargetHookLocation(crd, "v2", "github.com/test/hooks")

	got, err := crd.TargetHookFactories()
	if err != nil {
		t.Fatalf("TargetHookFactories returned error: %v", err)
	}
	if got != nil {
		t.Error("no target factories when the target shares the base binary")
	}
}

func TestTargetHookFactories_ErrorWhenGVKMissingFromRegistry(t *testing.T) {
	delete(types.TargetHookRegistry, testGVK)

	crd := withTargetHookLocation(crdWithGVK(testGVK), "v2", "github.com/test/v2hooks")
	if _, err := crd.TargetHookFactories(); err == nil {
		t.Fatal("expected error when GVK missing from TargetHookRegistry, got nil")
	}
}

func TestTargetHookFactories_ErrorWhenTargetNameMissingFromRegistry(t *testing.T) {
	types.TargetHookRegistry[testGVK] = map[string]func() domain.AnyReconcileHooks{
		"other": stubHookFn(),
	}
	t.Cleanup(func() { delete(types.TargetHookRegistry, testGVK) })

	crd := withTargetHookLocation(crdWithGVK(testGVK), "v2", "github.com/test/v2hooks")
	if _, err := crd.TargetHookFactories(); err == nil {
		t.Fatal("expected error when target name missing from TargetHookRegistry, got nil")
	}
}

func TestTargetHookFactories_NoneWithoutServeTargetEntries(t *testing.T) {
	crd := crdWithGVK(testGVK)
	if got, err := crd.TargetHookFactories(); err != nil || got != nil {
		t.Fatalf("expected nothing for a CRD without serve.target entries, got %v, %v", got, err)
	}
}

// ── Target constructors ───────────────────────────────────────────────────────

func TestTargetConstructors_FoundForDistinctConstructor(t *testing.T) {
	types.TargetReconcilerRegistry[testGVK] = map[string]types.NewReconcilerFunc{
		"v2": stubRecFn(),
	}
	t.Cleanup(func() { delete(types.TargetReconcilerRegistry, testGVK) })

	crd := withTargetDefaultFalse(crdWithGVK(testGVK), "v2")
	got, err := crd.TargetConstructors()
	if err != nil {
		t.Fatalf("TargetConstructors returned error: %v", err)
	}
	if got["v2"] == nil {
		t.Error("expected a constructor for v2")
	}
}

func TestTargetConstructors_SkipsTargetWithDefaultReconciler(t *testing.T) {
	crd := crdWithGVK(testGVK)
	crd.Serve = &types.ServeConfig{Enabled: true}
	crd.Serve.Target.Entries = map[string]*types.ServeTargetConfig{
		"v2": {OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{Default: ptr.To(true)}}},
	}

	got, err := crd.TargetConstructors()
	if err != nil {
		t.Fatalf("TargetConstructors returned error: %v", err)
	}
	if got != nil {
		t.Error("no target constructors when the target uses the default reconciler")
	}
}

func TestTargetConstructors_ErrorWhenGVKMissingFromRegistry(t *testing.T) {
	delete(types.TargetReconcilerRegistry, testGVK)

	crd := withTargetDefaultFalse(crdWithGVK(testGVK), "v2")
	if _, err := crd.TargetConstructors(); err == nil {
		t.Fatal("expected error when GVK missing from TargetReconcilerRegistry, got nil")
	}
}

func TestTargetConstructors_ErrorWhenTargetNameMissingFromRegistry(t *testing.T) {
	types.TargetReconcilerRegistry[testGVK] = map[string]types.NewReconcilerFunc{
		"other": stubRecFn(),
	}
	t.Cleanup(func() { delete(types.TargetReconcilerRegistry, testGVK) })

	crd := withTargetDefaultFalse(crdWithGVK(testGVK), "v2")
	if _, err := crd.TargetConstructors(); err == nil {
		t.Fatal("expected error when target name missing from TargetReconcilerRegistry, got nil")
	}
}

func TestTargetConstructors_NoneWithoutServeTargetEntries(t *testing.T) {
	crd := crdWithGVK(testGVK)
	if got, err := crd.TargetConstructors(); err != nil || got != nil {
		t.Fatalf("expected nothing for a CRD without serve.target entries, got %v, %v", got, err)
	}
}
