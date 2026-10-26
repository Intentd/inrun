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

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert/yaml"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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

// setupObjects reads the CRD's setup.apply files — the objects inrun applies
// before the operator starts, such as a remote reconciler's token Secret —
// then the extra files, the simulate spec's setup. All paths are absolute:
// the merger resolves the CRD's, the caller the spec's.
func setupObjects(crd *types.CRDEntry, extra []string) ([]*unstructured.Unstructured, error) {
	var paths []string
	if crd.Setup != nil {
		for _, entry := range crd.Setup.Apply {
			paths = append(paths, entry.Path)
		}
	}
	paths = append(paths, extra...)
	var objs []*unstructured.Unstructured
	for _, path := range paths {
		data, err := fileio.ReadLocal(path)
		if err != nil {
			return nil, fmt.Errorf("setup %s: %w", path, err)
		}
		dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			u := &unstructured.Unstructured{}
			if err := dec.Decode(&u.Object); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("setup %s: %w", path, err)
			}
			if u.GetKind() != "" {
				objs = append(objs, u)
			}
		}
	}
	return objs, nil
}
