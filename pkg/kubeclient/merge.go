package kubeclient

import sigs "sigs.k8s.io/controller-runtime/pkg/client"

// Patch is sigs.k8s.io/controller-runtime/pkg/client.Patch, re-exported so
// constructor reconcilers only need to import this package. Values produced by
// sigs.MergeFrom, sigs.StrategicMergeFrom, and sigs.Apply satisfy this type
// directly — no adapter or wrapper needed when migrating from controller-runtime.
type Patch = sigs.Patch

// MergeFrom returns a JSON Merge Patch (RFC 7396) from base to the modified object
// passed to kube.Patch. Call before mutating the object:
//
//	patch := kubeclient.MergeFrom(existing.DeepCopy())
//	existing.Spec = desired.Spec
//	return kube.Patch(ctx, existing, patch)
//
// Use for CRDs and any object where replace semantics are correct.
// For core Kubernetes types with list merge keys, prefer StrategicMergeFrom.
//
// Delegates to sigs.k8s.io/controller-runtime/pkg/client.MergeFrom — the sigs
// version works here directly if you already import controller-runtime.
func MergeFrom(base sigs.Object) Patch {
	return sigs.MergeFrom(base)
}

// StrategicMergeFrom returns a Strategic Merge Patch for core Kubernetes types
// (Deployment, DaemonSet, StatefulSet, etc.) that carry patchMergeKey annotations.
// The API server merges list entries by key (e.g. containers by name) rather than
// replacing the entire list. For CRDs, use MergeFrom — the API server has no
// schema knowledge for custom types.
//
// Delegates to sigs.k8s.io/controller-runtime/pkg/client.StrategicMergeFrom — the
// sigs version works here directly if you already import controller-runtime.
func StrategicMergeFrom(base sigs.Object) Patch {
	return sigs.StrategicMergeFrom(base)
}
