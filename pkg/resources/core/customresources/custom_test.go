package customresources

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/testutil"
)

var (
	gvk = schema.GroupVersionKind{Group: "db.example.com", Version: "v1", Kind: "Database"}
	gvr = schema.GroupVersionResource{Group: "db.example.com", Version: "v1", Resource: "databases"}
)

func spec() Spec {
	return Spec{
		APIVersion: "db.example.com/v1",
		Kind:       "Database",
		Metadata:   types.CustomResourceMetadata{Name: "orders", Labels: map[string]string{"tier": "data"}},
		Spec:       map[string]any{"engine": "postgres"},
	}
}

func clients() (*dynamicfake.FakeDynamicClient, meta.RESTMapper) {
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "DatabaseList"})
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gvk.GroupVersion()})
	mapper.Add(gvk, meta.RESTScopeNamespace)
	return dyn, mapper
}

func get(t *testing.T, dyn *dynamicfake.FakeDynamicClient) (*unstructured.Unstructured, error) {
	t.Helper()
	return dyn.Resource(gvr).Namespace("team").Get(context.Background(), "orders", metav1.GetOptions{})
}

func TestBuild(t *testing.T) {
	s := spec()
	u := Build(testutil.Owner(), s)

	if u.GetKind() != "Database" || u.GetName() != "orders" || u.GetNamespace() != "team" {
		t.Errorf("object = %s %s/%s, want Database team/orders", u.GetKind(), u.GetNamespace(), u.GetName())
	}
	if !testutil.Owned(u) {
		t.Errorf("owner references = %+v, want one to the owner", u.GetOwnerReferences())
	}
	if engine, _, _ := unstructured.NestedString(u.Object, "spec", "engine"); engine != "postgres" {
		t.Errorf("spec.engine = %q, want postgres", engine)
	}

	lbls := u.GetLabels()
	lbls["changed"] = "yes"
	if _, shared := s.Metadata.Labels["changed"]; shared {
		t.Error("Build shares the spec's labels map with the object; it should copy it")
	}
}

func TestCreateAndApply(t *testing.T) {
	ctx := context.Background()
	dyn, mapper := clients()
	owner, s := testutil.Owner(), spec()

	if res, err := Create(ctx, dyn, mapper, owner, s, nil); err != nil || res != types.Created {
		t.Fatalf("first Create = %q, %v; want created", res, err)
	}
	if res, err := Create(ctx, dyn, mapper, owner, s, nil); err != nil || res != types.Unchanged {
		t.Fatalf("second Create = %q, %v; want unchanged", res, err)
	}
	if res, err := Apply(ctx, dyn, mapper, owner, s, nil); err != nil || res != types.Unchanged {
		t.Fatalf("Apply in sync = %q, %v; want unchanged", res, err)
	}

	s.Spec = map[string]any{"engine": "mysql"}
	mutated := false
	mutate := func(u *unstructured.Unstructured) {
		mutated = true
		u.SetLabels(map[string]string{"protected": "true"})
	}
	if res, err := Apply(ctx, dyn, mapper, owner, s, mutate); err != nil || res != types.Applied {
		t.Fatalf("Apply after a change = %q, %v; want applied", res, err)
	}
	if !mutated {
		t.Error("Apply did not call mutate before the update")
	}

	u, err := get(t, dyn)
	if err != nil {
		t.Fatal(err)
	}
	if engine, _, _ := unstructured.NestedString(u.Object, "spec", "engine"); engine != "mysql" {
		t.Errorf("spec.engine = %q, want mysql", engine)
	}
	if u.GetLabels()["protected"] != "true" {
		t.Errorf("labels = %v, want mutate's label", u.GetLabels())
	}
}

func TestApply_CreatesWhenMissing(t *testing.T) {
	dyn, mapper := clients()
	res, err := Apply(context.Background(), dyn, mapper, testutil.Owner(), spec(), nil)
	if err != nil || res != types.Created {
		t.Fatalf("Apply of a missing object = %q, %v; want created", res, err)
	}
}

func TestDeleteIfOwned(t *testing.T) {
	ctx := context.Background()
	dyn, mapper := clients()
	if _, err := Create(ctx, dyn, mapper, testutil.Owner(), spec(), nil); err != nil {
		t.Fatal(err)
	}

	del := func(owned func(metav1.Object) bool) types.Result {
		t.Helper()
		res, err := DeleteIfOwned(ctx, dyn, mapper, "orders", "team", "db.example.com/v1", "Database", owned)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	if res := del(func(metav1.Object) bool { return false }); res != types.NotOwned {
		t.Errorf("DeleteIfOwned of another owner's = %q, want not owned", res)
	}
	if res := del(types.OwnedByReference(testutil.Owner())); res != types.Deleted {
		t.Errorf("DeleteIfOwned of ours = %q, want deleted", res)
	}
	if _, err := get(t, dyn); err == nil {
		t.Error("object should be gone")
	}
	if res := del(types.OwnedByReference(testutil.Owner())); res != types.NotFound {
		t.Errorf("DeleteIfOwned of a deleted one = %q, want not found", res)
	}
}
