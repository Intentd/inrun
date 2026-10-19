package serve

import (
	"strings"
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/types"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func blueprintWithTargetCRDs(crds map[string]types.CRDEntry) *checker {
	return newBlueprintExec(crds)
}

func crdWithTarget(targetName string) types.CRDEntry {
	return types.CRDEntry{
		APITypes: types.APITypes{Kind: "MyResource"},
		Serve: &types.ServeConfig{
			Enabled: true,
			Target: types.ServeTargetValue{
				Entries: map[string]*types.ServeTargetConfig{
					targetName: {Primary: true},
				},
			},
		},
	}
}

func crdWithTargetAndBox(targetName string, box *types.OperatorBoxConfig) types.CRDEntry {
	crd := crdWithTarget(targetName)
	crd.Serve.Target.Entries[targetName].OperatorBox = box
	return crd
}

// ── isValidServeTarget ────────────────────────────────────────────────────────

func TestIsValidServeTarget_Valid(t *testing.T) {
	cases := []string{"app", "my-app", "web123", "a", "apifixture", "v2-preview"}
	for _, c := range cases {
		if !isValidServeTarget(c) {
			t.Errorf("%q should be valid", c)
		}
	}
}

func TestIsValidServeTarget_Invalid(t *testing.T) {
	cases := []string{"", "MyApp", "my_app", "app app", "app.v2", "APP"}
	for _, c := range cases {
		if isValidServeTarget(c) {
			t.Errorf("%q should be invalid", c)
		}
	}
}

// ── validateServeTarget — format ──────────────────────────────────────────────

func TestValidateServeTarget_ValidFormat(t *testing.T) {
	e := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTarget("my-resource"),
	})
	if err := e.validateServeTarget(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTarget_InvalidFormat(t *testing.T) {
	e := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTarget("My_Resource"),
	})
	err := e.validateServeTarget()
	if err == nil {
		t.Fatal("expected error for invalid target format")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error should mention invalid format, got: %v", err)
	}
}

// ── validateServeTarget — uniqueness ──────────────────────────────────────────

func TestValidateServeTarget_DuplicateTarget(t *testing.T) {
	e := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res1": crdWithTarget("app"),
		"res2": crdWithTarget("app"),
	})
	err := e.validateServeTarget()
	if err == nil {
		t.Fatal("expected error for duplicate target")
	}
	if !strings.Contains(err.Error(), "app") {
		t.Errorf("error should mention the duplicate target name, got: %v", err)
	}
}

func TestValidateServeTarget_UniqueTargets(t *testing.T) {
	e := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res1": crdWithTarget("web"),
		"res2": crdWithTarget("api"),
	})
	if err := e.validateServeTarget(); err != nil {
		t.Fatalf("unexpected error for unique targets: %v", err)
	}
}

func TestValidateServeTarget_NoServe(t *testing.T) {
	e := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": {APITypes: types.APITypes{Kind: "MyResource"}},
	})
	if err := e.validateServeTarget(); err != nil {
		t.Fatalf("unexpected error for CRD without serve: %v", err)
	}
}

// ── validateTargetOperatorBox — valid boxes ───────────────────────────────────

func TestValidateServeTarget_TargetBoxTemplatesOnly(t *testing.T) {
	box := &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{OnCreate: &types.HookTemplates{}},
		Runtime: &types.RuntimeConfig{Finalizers: &types.FinalizerConfig{
			Names: []string{"my.finalizer/cleanup"},
		}},
	}
	e := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTargetAndBox("web", box),
	})
	if err := e.validateServeTarget(); err != nil {
		t.Fatalf("unexpected error for valid target box: %v", err)
	}
}

func TestValidateServeTarget_NilTargetBox(t *testing.T) {
	e := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTargetAndBox("web", nil),
	})
	if err := e.validateServeTarget(); err != nil {
		t.Fatalf("unexpected error for nil target box: %v", err)
	}
}

// ── validateTargetOperatorBox — forbidden fields ──────────────────────────────

func TestValidateServeTarget_ReconcilerWorkers(t *testing.T) {
	box := &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{Workers: 3},
	}
	err := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTargetAndBox("web", box),
	}).validateServeTarget()
	if err == nil {
		t.Fatal("expected error for reconciler.workers on target box")
	}
	if !strings.Contains(err.Error(), "reconcile.workers") {
		t.Errorf("error should name reconciler.workers, got: %v", err)
	}
}

func TestValidateServeTarget_ReconcilerResync(t *testing.T) {
	box := &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{
			Resync: types.Duration{Duration: 30 * time.Second},
		},
	}
	err := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTargetAndBox("web", box),
	}).validateServeTarget()
	if err == nil {
		t.Fatal("expected error for reconciler.resync on target box")
	}
	if !strings.Contains(err.Error(), "reconcile.resync") {
		t.Errorf("error should name reconciler.resync, got: %v", err)
	}
}

func TestValidateServeTarget_ReconcilerQueue(t *testing.T) {
	box := &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{
			Queue: types.Queue{MaxDepth: 50},
		},
	}
	err := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTargetAndBox("web", box),
	}).validateServeTarget()
	if err == nil {
		t.Fatal("expected error for reconciler.queue on target box")
	}
	if !strings.Contains(err.Error(), "reconcile.queue") {
		t.Errorf("error should name reconciler.queue, got: %v", err)
	}
}

func TestValidateServeTarget_ReconcilerProfile(t *testing.T) {
	box := &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{Profile: "high-throughput"},
	}
	err := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTargetAndBox("web", box),
	}).validateServeTarget()
	if err == nil {
		t.Fatal("expected error for reconciler.profile on target box")
	}
	if !strings.Contains(err.Error(), "reconcile.profile") {
		t.Errorf("error should name reconciler.profile, got: %v", err)
	}
}

func TestValidateServeTarget_Autoscale(t *testing.T) {
	box := &types.OperatorBoxConfig{
		Runtime: &types.RuntimeConfig{Autoscale: &types.AutoscaleSpec{}},
	}
	err := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTargetAndBox("web", box),
	}).validateServeTarget()
	if err == nil {
		t.Fatal("expected error for autoscale on target box")
	}
	if !strings.Contains(err.Error(), "autoscale") {
		t.Errorf("error should name autoscale, got: %v", err)
	}
}

func TestValidateServeTarget_MultipleViolations(t *testing.T) {
	box := &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{
			Workers: 5,
			Profile: "high-throughput",
		},
		Runtime: &types.RuntimeConfig{Autoscale: &types.AutoscaleSpec{}},
	}
	err := blueprintWithTargetCRDs(map[string]types.CRDEntry{
		"res": crdWithTargetAndBox("web", box),
	}).validateServeTarget()
	if err == nil {
		t.Fatal("expected error for multiple violations")
	}
	msg := err.Error()
	for _, field := range []string{"reconcile.workers", "reconcile.profile", "autoscale"} {
		if !strings.Contains(msg, field) {
			t.Errorf("error should name %q, got: %v", field, msg)
		}
	}
}
