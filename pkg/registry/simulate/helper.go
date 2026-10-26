package simulate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/intent"
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

// resolveCacheKey returns the cache key for a CR.
func resolveCacheKey(cr *unstructured.Unstructured) (string, error) {
	key, err := cache.MetaNamespaceKeyFunc(cr)
	if err != nil {
		return "", fmt.Errorf("computing CR key: %w", err)
	}
	return key, nil
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
func noWrites(ops Ops) bool {
	for _, op := range ops {
		if types.IsWriteVerb(op.Verb) {
			return false
		}
	}
	return true
}

// opsMatch returns true when two op slices have the same verb+resource sequence.
func opsMatch(a, b Ops) bool {
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

// prepareCRDEntry prepares a CRD entry and resource for simulation, and
// returns the resource to simulate.
//
// When target is set, it is applied to a copy of the resource so the
// reconciler resolves the resource as though it had been created through
// that target; the copy is what the caller must seed and reconcile, and
// the caller's resource is left as it was.
func prepareCRDEntry(
	bp *blueprint.Blueprint,
	cr *unstructured.Unstructured,
	name, target string,
) (*Result, types.CRDEntry, *unstructured.Unstructured, error) {
	entry, ok := bp.CRDEntry(name)
	if !ok {
		return nil, types.CRDEntry{}, nil, fmt.Errorf("CRD %q not found in Blueprint", name)
	}

	result := &Result{}

	if target != "" {
		cr = cr.DeepCopy()
		intent.SetTarget(cr, target)
	}

	return result, entry, cr, nil
}
