package reconcile

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fakePatcher records each merge patch and the annotations it would leave.
type fakePatcher struct {
	patches []map[string]interface{} // metadata.annotations of each patch
}

func (f *fakePatcher) Patch(_ context.Context, obj domain.Object, p kubeclient.Patch, _ metav1.PatchOptions) error {
	data, err := p.Data(obj)
	if err != nil {
		return err
	}
	var body struct {
		Metadata struct {
			Annotations map[string]interface{} `json:"annotations"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return err
	}
	f.patches = append(f.patches, body.Metadata.Annotations)
	return nil
}

func newCR(phase string, ann map[string]string) *unstructured.Unstructured {
	return object.New("demo.inrun.dev/v1alpha1", "Job", "j", object.InNamespace("default"),
		object.WithStatus(map[string]any{"phase": phase}), object.WithAnnotations(ann))
}

func cleanupBox(after time.Duration) types.OperatorBoxConfig {
	return types.OperatorBoxConfig{Runtime: &types.RuntimeConfig{Cleanup: &types.CleanupConfig{
		Or:    []types.Condition{{Field: "status.phase", Equals: "Completed"}},
		After: types.Duration{Duration: after},
	}}}
}

func resolverFor(t *testing.T, obj domain.Object) *template.Resolver {
	t.Helper()
	r, err := template.NewResolver(context.Background(), obj)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func near(got, want time.Duration) bool {
	d := got - want
	return d > -2*time.Second && d < 2*time.Second
}

func TestPatchAnnotations_RemovalSendsNull(t *testing.T) {
	f := &fakePatcher{}
	obj := newCR("Running", map[string]string{"keep": "1", cleanupDueAnnotation: "x"})

	if err := patchAnnotations(context.Background(), f, obj, map[string]string{"new": "2"}, cleanupDueAnnotation); err != nil {
		t.Fatal(err)
	}
	got := f.patches[0]
	if v, ok := got[cleanupDueAnnotation]; !ok || v != nil {
		t.Errorf("removed annotation must be sent as null, got %v (present=%v)", v, ok)
	}
	if got["new"] != "2" {
		t.Errorf("set annotation missing: %v", got)
	}
	if _, ok := got["keep"]; ok {
		t.Errorf("unchanged annotation must not be in the patch: %v", got)
	}
	if obj.GetAnnotations()[cleanupDueAnnotation] != "x" {
		t.Error("the object itself must not be modified (it may be the cache's copy)")
	}
}

func TestSettleResultCleanup(t *testing.T) {
	ctx := context.Background()

	t.Run("no request, nothing pending: no patch", func(t *testing.T) {
		f := &fakePatcher{}
		w, err := settleResultCleanup(ctx, f, newCR("Running", nil), nil)
		if err != nil || w.pending || len(f.patches) != 0 {
			t.Fatalf("w=%+v err=%v patches=%v", w, err, f.patches)
		}
	})

	t.Run("first request starts the clock", func(t *testing.T) {
		f := &fakePatcher{}
		w, err := settleResultCleanup(ctx, f, newCR("Done", nil), &domain.Cleanup{After: time.Hour, Reason: "finished"})
		if err != nil || !w.pending || !near(w.wait, time.Hour) {
			t.Fatalf("w=%+v err=%v", w, err)
		}
		due, _ := time.Parse(time.RFC3339, f.patches[0][cleanupDueAnnotation].(string))
		if !near(time.Until(due), time.Hour) {
			t.Errorf("due = %v, want about an hour from now", due)
		}
		if f.patches[0][cleanupReasonAnnotation] != "finished" {
			t.Errorf("reason not recorded: %v", f.patches[0])
		}
	})

	t.Run("later requests keep the due time", func(t *testing.T) {
		f := &fakePatcher{}
		due := time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)
		obj := newCR("Done", map[string]string{cleanupDueAnnotation: due})
		w, err := settleResultCleanup(ctx, f, obj, &domain.Cleanup{After: time.Hour})
		if err != nil || len(f.patches) != 0 || !near(w.wait, 10*time.Minute) {
			t.Fatalf("w=%+v err=%v patches=%v", w, err, f.patches)
		}
	})

	t.Run("nil withdraws a pending request", func(t *testing.T) {
		f := &fakePatcher{}
		obj := newCR("Running", map[string]string{cleanupDueAnnotation: "2030-01-01T00:00:00Z", cleanupReasonAnnotation: "r"})
		w, err := settleResultCleanup(ctx, f, obj, nil)
		if err != nil || w.pending || len(f.patches) != 1 {
			t.Fatalf("w=%+v err=%v patches=%v", w, err, f.patches)
		}
		for _, k := range []string{cleanupDueAnnotation, cleanupReasonAnnotation} {
			if v, ok := f.patches[0][k]; !ok || v != nil {
				t.Errorf("%s must be removed (null), got %v", k, f.patches[0])
			}
		}
	})
}

func TestCheckCleanup(t *testing.T) {
	ctx := context.Background()

	t.Run("reconciler request past due: delete with its reason", func(t *testing.T) {
		obj := newCR("Done", map[string]string{
			cleanupDueAnnotation:    time.Now().Add(-time.Second).UTC().Format(time.RFC3339),
			cleanupReasonAnnotation: "job finished",
		})
		c, err := checkCleanup(ctx, &fakePatcher{}, obj, types.OperatorBoxConfig{}, resolverFor(t, obj))
		if err != nil || !c.due || c.reason != "job finished" {
			t.Fatalf("c=%+v err=%v", c, err)
		}
	})

	t.Run("reconciler request not yet due: pending", func(t *testing.T) {
		obj := newCR("Done", map[string]string{
			cleanupDueAnnotation: time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
		})
		c, err := checkCleanup(ctx, &fakePatcher{}, obj, types.OperatorBoxConfig{}, resolverFor(t, obj))
		if err != nil || c.due || !c.pending || !near(c.wait, 5*time.Minute) {
			t.Fatalf("c=%+v err=%v", c, err)
		}
	})

	t.Run("blueprint conditions met, no grace: delete now", func(t *testing.T) {
		obj := newCR("Completed", nil)
		c, err := checkCleanup(ctx, &fakePatcher{}, obj, cleanupBox(0), resolverFor(t, obj))
		if err != nil || !c.due || !strings.Contains(c.reason, "blueprint") {
			t.Fatalf("c=%+v err=%v", c, err)
		}
	})

	t.Run("blueprint conditions met first time: stamp and wait", func(t *testing.T) {
		f := &fakePatcher{}
		obj := newCR("Completed", nil)
		c, err := checkCleanup(ctx, f, obj, cleanupBox(time.Hour), resolverFor(t, obj))
		if err != nil || c.due || !c.pending || !near(c.wait, time.Hour) {
			t.Fatalf("c=%+v err=%v", c, err)
		}
		if _, ok := f.patches[0][cleanupPendingAnnotation].(string); !ok {
			t.Errorf("pending annotation not stamped: %v", f.patches)
		}
	})

	t.Run("blueprint conditions no longer met: clock reset", func(t *testing.T) {
		f := &fakePatcher{}
		obj := newCR("Running", map[string]string{cleanupPendingAnnotation: "2026-01-01T00:00:00Z"})
		c, err := checkCleanup(ctx, f, obj, cleanupBox(time.Hour), resolverFor(t, obj))
		if err != nil || c.due || c.pending {
			t.Fatalf("c=%+v err=%v", c, err)
		}
		if v, ok := f.patches[0][cleanupPendingAnnotation]; !ok || v != nil {
			t.Errorf("pending annotation must be removed (null): %v", f.patches)
		}
	})

	t.Run("both sources pending: the nearer wins", func(t *testing.T) {
		since := time.Now().Add(-50 * time.Minute).UTC().Format(time.RFC3339) // blueprint: 10m left
		obj := newCR("Completed", map[string]string{
			cleanupPendingAnnotation: since,
			cleanupDueAnnotation:     time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
		})
		c, err := checkCleanup(ctx, &fakePatcher{}, obj, cleanupBox(time.Hour), resolverFor(t, obj))
		if err != nil || c.due || !near(c.wait, 10*time.Minute) {
			t.Fatalf("c=%+v err=%v", c, err)
		}
	})
}
