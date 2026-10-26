package simulate

import (
	"context"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// partResources are the names a write to part of an object is recorded
// under (kubeclient's PatchFinalizers, PatchLabels, ...). They name no
// resource, so there is nothing to read back for them; the object they
// patch is read as itself.
var partResources = map[string]bool{
	kubeclient.PatchPartFinalizers:  true,
	kubeclient.PatchPartLabels:      true,
	kubeclient.PatchPartAnnotations: true,
	kubeclient.PatchPartStatus:      true,
	kubeclient.PatchPartSpec:        true,
}

// finalObjects reads back the CR under simulation and every object a write
// named, as they stand after the last cycle: the CR first, then the others
// in the order they were first written. An object no longer there (deleted)
// is left out, as is a write whose API group and version were not recorded.
func finalObjects(ctx context.Context, kube loopKube, crGVR schema.GroupVersionResource, cr *unstructured.Unstructured, ops Ops) []*unstructured.Unstructured {
	type ref struct {
		gvr             schema.GroupVersionResource
		namespace, name string
	}
	refs := []ref{{crGVR, cr.GetNamespace(), cr.GetName()}}
	seen := map[ref]bool{refs[0]: true}
	for _, op := range ops {
		if !types.IsWriteVerb(op.Verb) || op.Version == "" || op.Name == "" || partResources[op.Resource] {
			continue
		}
		r := ref{schema.GroupVersionResource{Group: op.Group, Version: op.Version, Resource: op.Resource}, op.Namespace, op.Name}
		if !seen[r] {
			seen[r] = true
			refs = append(refs, r)
		}
	}

	var out []*unstructured.Unstructured
	for _, r := range refs {
		if obj, err := kube.Object(ctx, r.gvr, r.namespace, r.name); err == nil {
			out = append(out, obj)
		}
	}
	return out
}
