package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// CheckCRDExists returns nil when the CRD for gvk is registered in the cluster. It
// returns a descriptive error naming the CRD when discovery cannot find it,
// or the underlying error when discovery itself fails. Single-shot: one
// lookup, no retry.
func CheckCRDExists(cfg *rest.Config, gvk *schema.GroupVersionKind) (bool, error) {
	if cfg == nil {
		return false, fmt.Errorf("CheckCRDExists: nil rest config")
	}
	if gvk == nil {
		return false, fmt.Errorf("CheckCRDExists: nil GroupVersionKind")
	}
	if gvk.Kind == "" || gvk.Version == "" {
		return false, fmt.Errorf("CheckCRDExists: incomplete GVK %q (group, kind, and version are required)", *gvk)
	}

	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return false, fmt.Errorf("discovery client: %w", err)
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disco))

	_, err = mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if meta.IsNoMatchError(err) {
		return false, fmt.Errorf("CRD %s.%s/%s not installed", gvk.Kind, gvk.Group, gvk.Version)
	}
	return true, err
}

// MatchesFieldRequirements checks that u satisfies all requirements using
// simple string equality on unstructured field values. Used as a post-filter
// after an index lookup to handle residual requirements the index did not cover.
func MatchesFieldRequirements(u *unstructured.Unstructured, reqs fields.Requirements) bool {
	for _, req := range reqs {
		parts := SplitField(req.Field)
		if len(parts) == 0 {
			continue
		}
		val, _, _ := unstructured.NestedString(u.Object, parts...)
		if val != req.Value {
			return false
		}
	}
	return true
}

func RequireStrParams(required map[string]string) error {
	var missing []string
	for k, v := range required {
		if v == "" {
			missing = append(missing, k)
		}
	}

	if len(missing) > 0 {
		err := fmt.Sprintf("missing required parameter(s): %s", strings.Join(missing, ", "))
		return fmt.Errorf("%s", err)
	}
	return nil
}

// BuildAnnotationPatch constructs a JSON merge patch that sets one annotation.
func BuildAnnotationPatch(key, value string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				key: value,
			},
		},
	}
}

// Used to merge labels
func Merge(target *string, incoming, sep string) {
	if *target == "" {
		*target = incoming
	} else {
		*target = *target + sep + incoming
	}
}

// IsRunningInPod returns true when running inside a Kubernetes pod.
// The service account token is always present inside a pod.
func IsRunningInPod() bool {
	_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
	return err == nil
}

// DumpCRsJSON writes each object in crs to w, grouped by kind (sorted) and
// indented as pretty-printed JSON. It is meant for debugging and test logs,
// not for machine-readable output — marshal errors are reported inline and do
// not stop the walk, so a single bad object doesn't hide the rest.
func DumpCRsJSON(w io.Writer, crs map[string][]*unstructured.Unstructured) {
	for _, kind := range sortedKinds(crs) {
		for i, u := range crs[kind] {
			b, err := json.MarshalIndent(u.Object, "", "  ")
			if err != nil {
				fmt.Fprintf(w, "%s[%d]: marshal error: %v\n", kind, i, err)
				continue
			}
			fmt.Fprintf(w, "%s[%d]:\n%s\n", kind, i, b)
		}
	}
}

// DumpCRsYAML is DumpCRsJSON with YAML output. Same contract: grouped by kind
// (sorted), best-effort on per-object marshal errors, intended for humans
// reading debug or test output.
func DumpCRsYAML(w io.Writer, crs map[string][]*unstructured.Unstructured) {
	for _, kind := range sortedKinds(crs) {
		for i, u := range crs[kind] {
			b, err := yaml.Marshal(u.Object)
			if err != nil {
				fmt.Fprintf(w, "%s[%d]: marshal error: %v\n", kind, i, err)
				continue
			}
			fmt.Fprintf(w, "%s[%d]:\n%s\n", kind, i, b)
		}
	}
}

// sortedKinds returns the keys of crs in ascending order so dumps are stable
// across runs — Go map iteration order is randomized.
func sortedKinds(crs map[string][]*unstructured.Unstructured) []string {
	kinds := make([]string, 0, len(crs))
	for k := range crs {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}
