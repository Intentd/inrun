package simulate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/inrundev/inrun/pkg/catalog"
	"github.com/inrundev/inrun/pkg/children"
	"github.com/inrundev/inrun/pkg/config"
	"github.com/inrundev/inrun/pkg/intent"
	"github.com/inrundev/inrun/pkg/kubeclient"
	"github.com/inrundev/inrun/pkg/labels"
	"github.com/inrundev/inrun/pkg/runtime/informer"
	"github.com/inrundev/inrun/pkg/runtime/queue"
	"github.com/inrundev/inrun/pkg/types"
	"github.com/inrundev/inrun/pkg/utils"
	"github.com/stretchr/testify/assert/yaml"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// outsidePodNamespace is the namespace the runtime uses outside a pod; a
// remote reconciler's secretRef without a namespace resolves to it.
const outsidePodNamespace = "default"

// effectiveOperatorBox returns the effective operatorBox for a given CR.
func effectiveOperatorBox(entry types.CRDEntry, cr *unstructured.Unstructured, target string) *types.OperatorBoxConfig {
	if target != "" {
		return entry.EffectiveOperatorBox(target)
	}

	effectiveTarget := intent.Target(cr.GetAnnotations())
	return entry.EffectiveOperatorBox(effectiveTarget)
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

// extractMetaName reads metadata.name from a raw Kubernetes API response body.
func extractMetaName(body []byte) string {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &obj); err == nil {
		return obj.Metadata.Name
	}
	return ""
}

// seedManagedMeta pre-populates managed labels and annotations on the CR so the
// reconciler's idempotency guards skip those patches in every cycle.
func seedManagedMeta(cr *unstructured.Unstructured, catalogName string) {
	lbls := cr.GetLabels()
	if lbls == nil {
		lbls = map[string]string{}
	}
	lbls[labels.ManagedKey] = labels.ManagedValue
	lbls[labels.DeletionProtectionLabel] = labels.DeletionProtectionValue
	cr.SetLabels(lbls)

	ann := cr.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[labels.AnnotationManagedBy] = catalogName
	ann[labels.AnnotationManagedSince] = time.Now().UTC().Format(time.RFC3339)
	cr.SetAnnotations(ann)
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
	data, err := utils.ReadLocal(path)
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

func informerFactory(kube *kubeclient.Kubeclient, restConfig *rest.Config, scheme *runtime.Scheme, cat *catalog.Catalog, qr *queue.QueueRegistry) *informer.Factory {
	provider := kube.NewClientProvider()

	for _, crd := range cat.Enabled() {
		crd := crd
		if crd.IsDynamic() {
			continue
		}
		object, list := crd.GetRuntimeObjects()

		provider.Register(object, func(k *kubeclient.Kubeclient) (informer.GenericClient, error) {
			return k.NewClient(list, kubeclient.CRDInfo{
				Kind:          crd.APITypes.Kind,
				Group:         crd.APITypes.Group,
				Version:       crd.APITypes.Version,
				APIPath:       crd.APITypes.APIPath,
				GroupVersion:  crd.GroupVersion,
				Plural:        crd.APITypes.Plural,
				Namespace:     crd.Namespace,
				Namespaced:    crd.IsNamespaced(),
				ForceConflict: crd.ResolveForceConflict(),
			})
		})
	}

	return informer.SharedInformerFactory(restConfig, informer.FactoryOptions{
		Provider:      provider,
		Scheme:        scheme,
		DefaultWq:     queue.NewWorkqueue("default-harness-workqueue"),
		Config:        config.NewDefaultConfig(),
		Catalog:       cat,
		ClientSet:     kube.Clientset(),
		QueueRegistry: qr,
	})
}
