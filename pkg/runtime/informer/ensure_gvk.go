// Problem statement
//
// Typed informer payloads can arrive with an empty TypeMeta (apiVersion/kind)
// on the very first watch event after a CR is created. When TypeMeta is missing,
// downstream code that creates ownerReferences or relies on GVKs will fail with
// API server validation errors (for example: "metadata.ownerReferences.apiVersion:
// Invalid value: \"\": version must not be empty").
//
// This is a problem specifically for *typed* objects delivered by the informer:
// the informer cache may hand a typed object whose GVK was stripped by the
// watch event. Unstructured objects do not suffer this issue and therefore do
// not require the same normalization.
//
// To guarantee correctness for all operator styles — including custom
// constructors that implement Reconcile(ctx, key) directly — we must ensure
// the object's GVK is populated *at the informer level before the object is
// enqueued or handed to user code*. The reconciler-level GVK fix remains as a
// fallback for rare race conditions, but it is not sufficient for the
// Constructor use case where user code reads directly from the informer cache.
//
// This file implements the informer-level normalization: inspect each Add/Update/Delete
// payload (including DeletedFinalStateUnknown wrappers) and set the GVK from the
// known CRD entry when it is missing. Log when a GVK is set for observability.

package informer

import (
	"github.com/intentd/inrun/pkg/logger"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// ensureGVKOnRuntimeObject sets the GVK on the provided runtime.Object if it is empty.
// It handles typed objects and unstructured objects. gvk must be non-nil.
func ensureGVKOnRuntimeObject(obj runtime.Object, gvk *schema.GroupVersionKind) {
	if obj == nil || gvk == nil {
		return
	}

	// If object already has GVK, nothing to do
	if !obj.GetObjectKind().GroupVersionKind().Empty() {
		return
	}

	// For unstructured objects, set APIVersion/Kind directly
	if u, ok := obj.(*unstructured.Unstructured); ok {
		u.SetAPIVersion(gvk.GroupVersion().String())
		u.SetKind(gvk.Kind)
		logger.Info().Str("gvk", gvk.String()).Msg("set GVK on unstructured object")
		return
	}

	// For typed objects, use the runtime.ObjectKind setter
	obj.GetObjectKind().SetGroupVersionKind(*gvk)
	logger.Debug().Str("gvk", gvk.String()).Msg("set GVK on typed object")
}

// normalizeInformerObject inspects the informer payload and ensures any runtime.Object
// inside has its GVK set. It handles DeletedFinalStateUnknown wrappers.
func normalizeInformerObject(payload interface{}, gvk *schema.GroupVersionKind) {
	switch v := payload.(type) {
	case runtime.Object:
		ensureGVKOnRuntimeObject(v, gvk)
	case cache.DeletedFinalStateUnknown:
		// DeletedFinalStateUnknown may wrap the last known object
		if obj, ok := v.Obj.(runtime.Object); ok {
			ensureGVKOnRuntimeObject(obj, gvk)
		}
	default:
		// Some informers deliver pointers to typed objects via interface{}; try reflection fallback
		if obj, ok := payload.(runtime.Object); ok {
			ensureGVKOnRuntimeObject(obj, gvk)
		} else {
			// Not a runtime.Object we can fix; log at debug level
			logger.Debug().Msg("normalizeInformerObject: payload is not a runtime.Object; skipping GVK fix")
		}
	}
}
