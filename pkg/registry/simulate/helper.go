package simulate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/intentd/inrun/pkg/util/fileio"

	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert/yaml"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

const (
	// outsidePodNamespace is the namespace the runtime uses outside a pod; a
	// remote reconciler's secretRef without a namespace resolves to it.
	outsidePodNamespace = "default"

	// remoteNote tells the user the run depends on their reconciler process.
	remoteNote = "remote reconciler: each cycle calls the endpoint in reconcile.remote — start the reconciler before simulating"
)

// simulatable returns a copy of crd whose templates, CRD-level and per serve
// target, leave out what a simulation cannot run: cross-namespace copies need
// a live API server to read their source. The notes say what was left out.
// The Blueprint's own entry is not touched.
func simulatable(crd types.CRDEntry) (types.CRDEntry, []string) {
	var notes []string
	filter := func(box *types.OperatorBoxConfig) *types.OperatorBoxConfig {
		if box == nil || box.Reconcile == nil {
			return box
		}
		b, rc := *box, *box.Reconcile
		for _, phase := range []**types.HookTemplates{&rc.OnCreate, &rc.OnReconcile, &rc.OnDelete} {
			if *phase == nil {
				continue
			}
			filtered, skipped := types.FilterSimulatable(**phase)
			*phase = &filtered
			notes = append(notes, skipped...)
		}
		b.Reconcile = &rc
		return &b
	}

	crd.OperatorBox = filter(crd.OperatorBox)
	if crd.Serve != nil && len(crd.Serve.Target.Entries) > 0 {
		serve := *crd.Serve
		entries := make(map[string]*types.ServeTargetConfig, len(serve.Target.Entries))
		for name, e := range serve.Target.Entries {
			if e != nil {
				cp := *e
				cp.OperatorBox = filter(e.OperatorBox)
				e = &cp
			}
			entries[name] = e
		}
		serve.Target.Entries = entries
		crd.Serve = &serve
	}
	return crd, notes
}

// getFromIndexerOrFallback returns a func that fetches CR from indexer or returns fallback.
func getFromIndexerOrFallback(indexer cache.Indexer, key string, cr *unstructured.Unstructured) func() *unstructured.Unstructured {
	return func() *unstructured.Unstructured {
		item, exists, _ := indexer.GetByKey(key)
		if !exists || item == nil {
			return nil
		}
		if u, ok := item.(*unstructured.Unstructured); ok {
			return u
		}
		return cr
	}
}

// resolveCacheKey returns the cache key for a CR.
func resolveCacheKey(cr *unstructured.Unstructured) (string, error) {
	key, err := cache.MetaNamespaceKeyFunc(cr)
	if err != nil {
		return "", fmt.Errorf("computing CR key: %w", err)
	}
	return key, nil
}

// parseCollectionURL returns (resource, namespace) from a collection-level URL
// (the target of a POST create where no name appears in the path).
//
//	/api/v1/namespaces/{ns}/{resource}
//	/api/v1/{resource}
//	/apis/{group}/{version}/namespaces/{ns}/{resource}
//	/apis/{group}/{version}/{resource}
func parseCollectionURL(req *http.Request) (resource, namespace string) {
	p := strings.TrimPrefix(req.URL.Path, "/")
	parts := strings.Split(p, "/")
	switch {
	case len(parts) == 5 && parts[0] == "api" && parts[2] == "namespaces":
		return parts[4], parts[3]
	case len(parts) == 3 && parts[0] == "api":
		return parts[2], ""
	case len(parts) == 6 && parts[0] == "apis" && parts[3] == "namespaces":
		return parts[5], parts[4]
	case len(parts) == 4 && parts[0] == "apis":
		return parts[3], ""
	}
	return
}

// extractMetaName reads metadata.name from a raw Kubernetes API response
// body: JSON for custom resources, and JSON or protobuf for built-in kinds
// (typed clients ask for protobuf), decoded with client-go's scheme.
func extractMetaName(body []byte) string {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &obj); err == nil {
		return obj.Metadata.Name
	}
	decoded, _, err := clientgoscheme.Codecs.UniversalDeserializer().Decode(body, nil, nil)
	if err != nil {
		return ""
	}
	if m, err := meta.Accessor(decoded); err == nil {
		return m.GetName()
	}
	return ""
}

// noWrites reports whether ops contains no write: only reads, or nothing.
func noWrites(ops []Op) bool {
	for _, op := range ops {
		switch op.Verb {
		case "get", "list", "watch":
		default:
			return false
		}
	}
	return true
}

// opsMatch returns true when two op slices have the same verb+resource sequence.
func opsMatch(a, b []Op) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Verb != b[i].Verb || a[i].Resource != b[i].Resource {
			return false
		}
	}
	return true
}

// IsSimulateDoc returns true when the file's kind is "Simulate".
func IsSimulateDoc(path string) bool {
	data, err := fileio.ReadLocal(path)
	if err != nil {
		return false
	}
	var head struct {
		Kind string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(data, &head); err != nil {
		return false
	}
	return head.Kind == config.SimulateKind()
}

// listKindsForSeeds starts from the registry's full built-in list-kind map and
// merges the list kind for every seeded object's GVR. The registry covers
// built-ins a reconciler might List without seeding (e.g. ConfigMaps); the
// seeds cover the CRD under test and any siblings.
func listKindsForSeeds(scheme *runtime.Scheme, seeds []runtime.Object) map[schema.GroupVersionResource]string {
	listKinds := children.ListKindsForFakeDynamic()

	for _, obj := range seeds {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Empty() {
			if kinds, _, err := scheme.ObjectKinds(obj); err == nil && len(kinds) > 0 {
				gvk = kinds[0]
			}
		}
		if gvk.Empty() {
			continue
		}
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)
		listKinds[gvr] = gvk.Kind + "List"
	}

	return listKinds
}

// setupObjects reads the CRD's setup.apply files: the objects inrun applies
// before the operator starts, such as a remote reconciler's token Secret.
// The merger has already made the paths absolute.
func setupObjects(crd *types.CRDEntry) ([]*unstructured.Unstructured, error) {
	if crd.Setup == nil {
		return nil, nil
	}
	var objs []*unstructured.Unstructured
	for _, entry := range crd.Setup.Apply {
		data, err := fileio.ReadLocal(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("setup %s: %w", entry.Path, err)
		}
		dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			u := &unstructured.Unstructured{}
			if err := dec.Decode(&u.Object); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("setup %s: %w", entry.Path, err)
			}
			if u.GetKind() != "" {
				objs = append(objs, u)
			}
		}
	}
	return objs, nil
}

// verbFromAction returns "apply" for Server-Side Apply patch operations,
// "patch" for all other patch types, and the raw verb for everything else.
func verbFromAction(action k8stesting.Action) string {
	if pa, ok := action.(k8stesting.PatchAction); ok {
		if pa.GetPatchType() == k8stypes.ApplyPatchType {
			return "apply"
		}
	}
	return action.GetVerb()
}

// nameFromAction extracts the resource name from a test action. Actions that
// name their object (get, delete, patch) satisfy GetAction; actions that carry
// it (create, update) satisfy CreateAction, so one case covers each.
func nameFromAction(action k8stesting.Action) string {
	switch a := action.(type) {
	case k8stesting.GetAction:
		return a.GetName()
	case k8stesting.CreateAction:
		if acc, ok := a.GetObject().(metav1.Object); ok {
			return acc.GetName()
		}
	}
	return ""
}

// sameKind reports whether a and b are the same kind, when both say.
func sameKind(a, b runtime.Object) bool {
	ka, kb := a.GetObjectKind().GroupVersionKind().Kind, b.GetObjectKind().GroupVersionKind().Kind
	return ka == "" || kb == "" || ka == kb
}

func stringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func nameFromRuntimeObject(obj runtime.Object) string {
	if acc, ok := obj.(interface{ GetName() string }); ok {
		return acc.GetName()
	}
	return ""
}
