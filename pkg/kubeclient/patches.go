package kubeclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/intentd/inrun/domain"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// PartPatch is a JSON Merge Patch of one part of an object — its finalizers,
// labels, annotations, status or spec — as the Patch* methods send it. The
// builders below are the one place each body is made: the real client sends
// it to the API server, the fake client (kubefake) applies the same body to
// its store, so both write exactly the same thing.
type PartPatch struct {
	// Part names what the patch writes: one of the PatchPart* names.
	Part string
	// Body is the merge patch.
	Body []byte
	// Subresource is "status" for a status patch; empty otherwise.
	Subresource string
}

// FinalizersPatch replaces the object's finalizer list with finalizers,
// touching only metadata.finalizers.
func FinalizersPatch(finalizers []string) (PartPatch, error) {
	body, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"finalizers": finalizers},
	})
	if err != nil {
		return PartPatch{}, fmt.Errorf("marshalling finalizer patch: %w", err)
	}
	return PartPatch{Part: PatchPartFinalizers, Body: body}, nil
}

// LabelsPatch transitions labels from base to desired: keys in base but not
// in desired are set to null so the server deletes them, keys in desired
// that differ from base are added or updated, unchanged keys are left out.
// It reports false when there is nothing to send.
func LabelsPatch(base, desired map[string]string) (PartPatch, bool, error) {
	labelPatch := make(map[string]interface{})
	for key := range base {
		if _, ok := desired[key]; !ok {
			labelPatch[key] = nil
		}
	}
	for key, val := range desired {
		if base[key] != val {
			labelPatch[key] = val
		}
	}
	if len(labelPatch) == 0 {
		return PartPatch{}, false, nil
	}
	body, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"labels": labelPatch},
	})
	if err != nil {
		return PartPatch{}, false, fmt.Errorf("marshalling label patch: %w", err)
	}
	return PartPatch{Part: PatchPartLabels, Body: body}, true, nil
}

// AnnotationsPatch merges annotations onto the object: keys present are added
// or updated, keys absent are left unchanged (not deleted).
func AnnotationsPatch(annotations map[string]string) (PartPatch, error) {
	body, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"annotations": annotations},
	})
	if err != nil {
		return PartPatch{}, fmt.Errorf("marshalling annotation patch: %w", err)
	}
	return PartPatch{Part: PatchPartAnnotations, Body: body}, nil
}

// StatusPatch merges statusFields into the object's status, on the /status
// subresource. It reports false when there is nothing to send.
func StatusPatch(statusFields map[string]interface{}) (PartPatch, bool, error) {
	if len(statusFields) == 0 {
		return PartPatch{}, false, nil
	}
	body, err := json.Marshal(map[string]interface{}{"status": statusFields})
	if err != nil {
		return PartPatch{}, false, fmt.Errorf("marshalling status patch: %w", err)
	}
	return PartPatch{Part: PatchPartStatus, Body: body, Subresource: "status"}, true, nil
}

// SpecPatch merges specFields into the object's spec.
func SpecPatch(specFields map[string]interface{}) (PartPatch, error) {
	body, err := json.Marshal(map[string]interface{}{"spec": specFields})
	if err != nil {
		return PartPatch{}, fmt.Errorf("marshalling spec patch: %w", err)
	}
	return PartPatch{Part: PatchPartSpec, Body: body}, nil
}

// sendPart sends p to obj through the dynamic client, with the request's
// context tagged with p.Part so a recording transport names the write as the
// fake client does (see withPatchPart).
func (k *Kubeclient) sendPart(ctx context.Context, obj runtime.Object, p PartPatch, opts metav1.PatchOptions) error {
	ctx = withPatchPart(ctx, p.Part)
	mapping, err := k.gvrFor(obj)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("getting accessor: %w", err)
	}
	var subresources []string
	if p.Subresource != "" {
		subresources = []string{p.Subresource}
	}
	res := k.dynamic.Resource(mapping.Resource)
	if ns := accessor.GetNamespace(); ns != "" {
		_, err = res.Namespace(ns).Patch(ctx, accessor.GetName(), types.MergePatchType, p.Body, opts, subresources...)
	} else {
		_, err = res.Patch(ctx, accessor.GetName(), types.MergePatchType, p.Body, opts, subresources...)
	}
	return err
}

// PatchFinalizers replaces the object's finalizer list with finalizers by
// sending a minimal JSON Merge Patch that touches only metadata.finalizers.
// Sending only the changed field avoids resourceVersion conflicts that would
// arise from patching the full object.
func (k *Kubeclient) PatchFinalizers(
	ctx context.Context,
	obj runtime.Object,
	finalizers []string,
	opts metav1.PatchOptions,
) error {
	p, err := FinalizersPatch(finalizers)
	if err != nil {
		return err
	}
	return k.sendPart(ctx, obj, p, opts)
}

// PatchLabels transitions the object's labels from base to desired by sending
// a JSON Merge Patch. Keys present in base but absent in desired are set to
// null so the server deletes them. Keys in desired that differ from base are
// added or updated. Unchanged keys are omitted from the patch body.
//
// base must be a snapshot of the labels as they exist on the server immediately
// before any in-memory mutations are applied (the controller-runtime MergeFrom
// pattern). Pass nil for base when the object is brand-new and has no labels.
func (k *Kubeclient) PatchLabels(
	ctx context.Context,
	obj runtime.Object,
	base, desired map[string]string,
	opts metav1.PatchOptions,
) error {
	p, changed, err := LabelsPatch(base, desired)
	if err != nil || !changed {
		return err
	}
	return k.sendPart(ctx, obj, p, opts)
}

// PatchAnnotations merges annotations onto the object by sending a JSON Merge
// Patch containing only the desired annotation keys. Keys present in annotations
// are added or updated on the server; keys absent from the patch are left
// unchanged (not deleted).
//
// This is intentionally a one-way merge: Inrun's annotation management
// only ever adds keys (managed-by and managed-since are write-once and never
// removed). If key deletion is ever needed, mirror the base/desired pattern
// used by [PatchLabels].
func (k *Kubeclient) PatchAnnotations(
	ctx context.Context,
	obj runtime.Object,
	annotations map[string]string,
	opts metav1.PatchOptions,
) error {
	p, err := AnnotationsPatch(annotations)
	if err != nil {
		return err
	}
	return k.sendPart(ctx, obj, p, opts)
}

// PatchSpec merges specFields into the object's spec with a JSON Merge Patch.
func (k *Kubeclient) PatchSpec(
	ctx context.Context,
	obj domain.Object,
	specFields map[string]interface{},
	opts metav1.PatchOptions,
) error {
	p, err := SpecPatch(specFields)
	if err != nil {
		return err
	}
	return k.sendPart(ctx, obj, p, opts)
}
