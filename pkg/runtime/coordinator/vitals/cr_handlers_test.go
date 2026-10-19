package vitals

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/intentd/inrun/pkg/runtimeapi"

	"github.com/intentd/inrun/domain"

	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// fakeInformer satisfies cache.SharedIndexInformer for tests that only need
// GetIndexer() — matches the same pattern used in pkg/registry/simulate.
type fakeInformer struct {
	indexer cache.Indexer
	cache.SharedIndexInformer
}

func (f *fakeInformer) GetIndexer() cache.Indexer { return f.indexer }

func newTestInformer(t *testing.T, objs ...map[string]interface{}) cache.SharedIndexInformer {
	t.Helper()
	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, o := range objs {
		if err := idx.Add(&unstructured.Unstructured{Object: o}); err != nil {
			t.Fatalf("seeding indexer: %v", err)
		}
	}
	return &fakeInformer{indexer: idx}
}

func website(name, namespace, domain string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "testground.inrun.dev/v1alpha1",
		"kind":       "Website",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"domain": domain,
		},
	}
}

func TestBuildCRListHandler_NoFieldParam(t *testing.T) {
	inf := newTestInformer(t, website("site-a", "default", "a.example.com"))
	crd := types.CRDEntry{Name: "website", APITypes: types.APITypes{Kind: "Website"}}
	handler := BuildCRListHandler(crd, inf, &RuntimeHealth{})

	req := httptest.NewRequest(http.MethodGet, "/blueprint/website/cr", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var resp runtimeapi.CRListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(resp.Items))
	}
	if resp.Items[0].Value != "" {
		t.Errorf("Value = %q, want empty — no ?field= was requested", resp.Items[0].Value)
	}
}

func TestBuildCRListHandler_FieldParamResolvesSpecValue(t *testing.T) {
	inf := newTestInformer(t,
		website("site-a", "default", "a.example.com"),
		website("site-b", "default", "b.example.com"),
	)
	crd := types.CRDEntry{Name: "website", APITypes: types.APITypes{Kind: "Website"}}
	handler := BuildCRListHandler(crd, inf, &RuntimeHealth{})

	req := httptest.NewRequest(http.MethodGet, "/blueprint/website/cr?field=spec.domain", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var resp runtimeapi.CRListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(resp.Items))
	}
	got := map[string]string{}
	for _, item := range resp.Items {
		got[item.Name] = item.Value
	}
	if got["site-a"] != "a.example.com" || got["site-b"] != "b.example.com" {
		t.Errorf("resolved values = %+v, want site-a=a.example.com site-b=b.example.com", got)
	}
}

func TestBuildCRListHandler_FieldParamMissingOnCR(t *testing.T) {
	inf := newTestInformer(t, website("site-a", "default", "a.example.com"))
	crd := types.CRDEntry{Name: "website", APITypes: types.APITypes{Kind: "Website"}}
	handler := BuildCRListHandler(crd, inf, &RuntimeHealth{})

	req := httptest.NewRequest(http.MethodGet, "/blueprint/website/cr?field=spec.doesNotExist", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var resp runtimeapi.CRListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Value != "" {
		t.Errorf("got %+v, want a single item with empty Value", resp.Items)
	}
}

func TestRelevantGVRsFromConfig_ManagedResources(t *testing.T) {
	keys := func(es []childGVREntry) map[string]bool {
		out := map[string]bool{}
		for _, e := range es {
			out[e.Key] = true
		}
		return out
	}

	byKind := &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
		ManagedResources: []domain.ManagedResource{{Kind: "Deployment"}, {Kind: "service"}},
	}}
	if got := keys(relevantGVRsFromConfig(byKind)); len(got) != 2 || !got["deployment"] || !got["service"] {
		t.Errorf("by kind: got %v, want deployment and service", got)
	}

	byPlural := &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
		ManagedResources: []domain.ManagedResource{{Group: "apps", Plural: "deployments"}},
	}}
	if got := keys(relevantGVRsFromConfig(byPlural)); len(got) != 1 || !got["deployment"] {
		t.Errorf("by group and plural: got %v, want deployment", got)
	}

	if got := relevantGVRsFromConfig(&types.OperatorBoxConfig{}); len(got) != len(knownChildGVRs) {
		t.Errorf("nothing declared: got %d kinds, want all %d", len(got), len(knownChildGVRs))
	}
}
