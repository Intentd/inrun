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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// Wait for CRD creation
func WaitForCRD(cfg *rest.Config, group, kind, version string) error {
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(
		memory.NewMemCacheClient(disco),
	)

	gk := schema.GroupKind{Group: group, Kind: kind}

	_, err = mapper.RESTMapping(gk, version)
	if meta.IsNoMatchError(err) {
		return fmt.Errorf("CRD %s.%s/%s not installed", kind, group, version)
	}
	return err
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
