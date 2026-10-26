package prepare

import (
	"context"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// fakeDynamicProvider satisfies dynamicClientProvider for tests.
type fakeDynamicProvider struct {
	dyn dynamic.Interface
}

func (f *fakeDynamicProvider) DynamicClient() dynamic.Interface { return f.dyn }

func websiteGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "inrun.dev", Version: "v1", Resource: "websites"}
}

func newWebsite(namespace, name, domain string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "inrun.dev/v1",
		"kind":       "Website",
		"metadata": map[string]interface{}{
			"namespace": namespace,
			"name":      name,
		},
		"spec": map[string]interface{}{
			"domain": domain,
		},
	}}
}

func newFakeProvider(t *testing.T, objs ...runtime.Object) *fakeDynamicProvider {
	t.Helper()
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		websiteGVR(): "WebsiteList",
	}
	dyn := kubefake.NewDynamic(scheme, gvrToListKind, objs...)
	return &fakeDynamicProvider{dyn: dyn}
}

func TestLiveUniquenessChecker_IsUnique(t *testing.T) {
	t.Run("no other instances", func(t *testing.T) {
		provider := newFakeProvider(t, newWebsite("default", "site-a", "a.example.com"))
		checker := newUniquenessChecker(context.Background(), provider, websiteGVR(), true)

		ok, err := checker.IsUnique("spec.domain", "a.example.com", "default", "site-a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected unique — self should be excluded from the comparison")
		}
	})

	t.Run("duplicate on another instance", func(t *testing.T) {
		provider := newFakeProvider(t,
			newWebsite("default", "site-a", "shared.example.com"),
			newWebsite("default", "site-b", "shared.example.com"),
		)
		checker := newUniquenessChecker(context.Background(), provider, websiteGVR(), true)

		ok, err := checker.IsUnique("spec.domain", "shared.example.com", "default", "site-b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected not-unique — site-a already has this domain")
		}
	})

	t.Run("unique across namespaces", func(t *testing.T) {
		provider := newFakeProvider(t,
			newWebsite("team-a", "site-a", "a.example.com"),
			newWebsite("team-b", "site-b", "b.example.com"),
		)
		checker := newUniquenessChecker(context.Background(), provider, websiteGVR(), true)

		ok, err := checker.IsUnique("spec.domain", "a.example.com", "team-a", "site-a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected unique")
		}
	})

	t.Run("duplicate across namespaces — checked globally, not per namespace", func(t *testing.T) {
		provider := newFakeProvider(t,
			newWebsite("team-a", "site-a", "shared.example.com"),
			newWebsite("team-b", "site-b", "shared.example.com"),
		)
		checker := newUniquenessChecker(context.Background(), provider, websiteGVR(), true)

		ok, err := checker.IsUnique("spec.domain", "shared.example.com", "team-b", "site-b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected not-unique — global uniqueness spans namespaces")
		}
	})
}
