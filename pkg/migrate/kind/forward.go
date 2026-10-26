package kind

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/migrate/core"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/rs/zerolog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
)

// maxAnnotationBytes is the Kubernetes limit on the total size of an
// object's annotations.
const maxAnnotationBytes = 256 * 1024

// buildDestination builds the destination CR and attaches the snapshot. The
// destination is not applied.
func (m *migrator) buildDestination(
	cr *unstructured.Unstructured,
	crd *types.CRDEntry,
	dstClusterScoped bool,
) (*unstructured.Unstructured, error) {
	resolver := template.NewResolverFromMap(cr.Object)

	dst, err := migrateTo(crd, cr, resolver)
	if err != nil {
		return nil, err
	}
	switch {
	case dstClusterScoped:
		dst.SetNamespace("")
	case cr.GetNamespace() == "":
		dst.SetNamespace(crd.Migrate.To.Namespace)
	}

	snapshotJSON, err := crSnapshot(cr, dst, crd.Migrate.Prune)
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	ann := dst.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[labels.AnnotationMigrateKindSnapshot] = string(snapshotJSON)
	ann[labels.AnnotationMigrateTimestamp] = time.Now().UTC().Format(time.RFC3339)
	if size := annotationBytes(ann); size > maxAnnotationBytes {
		return nil, fmt.Errorf("snapshot too large: annotations would be %d bytes, limit is %d", size, maxAnnotationBytes)
	}
	dst.SetAnnotations(ann)

	return dst, nil
}

func annotationBytes(ann map[string]string) int {
	n := 0
	for k, v := range ann {
		n += len(k) + len(v)
	}
	return n
}

// getDestination returns the live destination, or nil when it does not exist.
func (m *migrator) getDestination(
	ctx context.Context,
	dst *unstructured.Unstructured,
	dstResource dynamic.NamespaceableResourceInterface,
) (*unstructured.Unstructured, error) {
	existing, err := dstResource.Namespace(dst.GetNamespace()).
		Get(ctx, dst.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get destination: %w", err)
	}
	return existing, nil
}

// apply server-side applies the destination and returns the object the
// server produced. dryRun applies with DryRunAll.
func (m *migrator) apply(
	ctx context.Context,
	dst *unstructured.Unstructured,
	dstResource dynamic.NamespaceableResourceInterface,
	dryRun bool,
) (*unstructured.Unstructured, error) {
	body, err := json.Marshal(dst.Object)
	if err != nil {
		return nil, fmt.Errorf("marshal destination: %w", err)
	}
	opts := metav1.PatchOptions{
		FieldManager: config.FieldManagerGateway,
		Force:        ptr.To(m.Force),
	}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	out, err := dstResource.Namespace(dst.GetNamespace()).
		Patch(ctx, dst.GetName(), k8stypes.ApplyPatchType, body, opts)
	if err != nil {
		return nil, fmt.Errorf("apply destination: %w", err)
	}
	return out, nil
}

// prune deletes the source. Called only after the destination holds every
// source field. The precondition makes the delete fail if the source changed
// since it was read, so a newer write is never lost; the next run reports
// the destination as drifted.
func (m *migrator) prune(ctx context.Context, src *unstructured.Unstructured) error {
	err := m.Kube.Delete(ctx, src, metav1.DeleteOptions{DryRun: m.DryRunOpt(), Preconditions: unchanged(src)})
	if apierrors.IsConflict(err) {
		return fmt.Errorf("source changed during migration; not pruned, rerun to review")
	}
	if err != nil {
		return fmt.Errorf("destination applied; prune source: %w", err)
	}
	return nil
}

// unchanged returns delete preconditions that match obj as it was read.
func unchanged(obj *unstructured.Unstructured) *metav1.Preconditions {
	uid, rv := obj.GetUID(), obj.GetResourceVersion()
	p := &metav1.Preconditions{}
	if uid != "" {
		p.UID = &uid
	}
	if rv != "" {
		p.ResourceVersion = &rv
	}
	return p
}

func (m *migrator) processForward(
	ctx context.Context,
	cr *unstructured.Unstructured,
	crd *types.CRDEntry,
	dstResource dynamic.NamespaceableResourceInterface,
	dstClusterScoped bool,
	log zerolog.Logger,
) (core.CRResult, string) {
	result := core.CRResult{Namespace: cr.GetNamespace(), Name: cr.GetName()}

	if !m.Admits(crd, cr) {
		result.Skipped, result.Excluded = true, true
		return result, ""
	}

	dst, err := m.buildDestination(cr, crd, dstClusterScoped)
	if err != nil {
		return m.Fail(cr, err, log)
	}
	srcSpec, _, _ := unstructured.NestedMap(cr.Object, "spec")

	if owner, ok := m.claimed[core.CRRef(dst)]; ok && owner != core.CRRef(cr) {
		return m.Fail(cr, fmt.Errorf("destination %s is also the target of %s in this run", core.CRRef(dst), owner), log)
	}

	existing, err := m.getDestination(ctx, dst, dstResource)
	if err != nil {
		return m.Fail(cr, err, log)
	}
	if existing != nil {
		if owner, ok := snapshotSource(existing); ok && !owner.is(cr) {
			return m.Fail(cr, fmt.Errorf("destination %s was already migrated from %s", core.CRRef(existing), owner.ref()), log)
		}
		m.claim(dst, cr)
		liveSpec, _, _ := unstructured.NestedMap(existing.Object, "spec")
		if len(missingPaths(srcSpec, liveSpec)) > 0 {
			result.Drifted = true
			return result, ""
		}
		// Already migrated. Finish a prune that an earlier run did not.
		if crd.Migrate.Prune {
			if err := m.prune(ctx, cr); err != nil {
				return m.Fail(cr, err, log)
			}
			result.Pruned = true
			return result, ""
		}
		result.Skipped = true
		return result, ""
	}

	m.claim(dst, cr)

	// Dry-run first: the server applies defaults and prunes unknown fields,
	// so this shows what the destination would really hold.
	preview, err := m.apply(ctx, dst, dstResource, true)
	if err != nil {
		return m.Fail(cr, err, log)
	}
	previewSpec, _, _ := unstructured.NestedMap(preview.Object, "spec")
	if missing := missingPaths(srcSpec, previewSpec); len(missing) > 0 {
		return m.Fail(cr, fmt.Errorf("destination schema drops or changes fields: %s", strings.Join(missing, ", ")), log)
	}

	result.Fields = []string{"spec"}
	result.AfterSpec = previewSpec
	if m.DryRun {
		result.Pruned = crd.Migrate.Prune
		return result, ""
	}

	applied, err := m.apply(ctx, dst, dstResource, false)
	if err != nil {
		return m.Fail(cr, err, log)
	}
	if crd.Migrate.To.CarryStatus {
		status, _, _ := unstructured.NestedMap(cr.Object, "status")
		if err := m.writeStatus(ctx, dstResource.Namespace(dst.GetNamespace()), applied, status); err != nil {
			return m.Fail(cr, fmt.Errorf("carry status: %w", err), log)
		}
	}
	if crd.Migrate.Prune {
		if err := m.prune(ctx, cr); err != nil {
			return m.Fail(cr, err, log)
		}
		result.Pruned = true
	}
	return result, ""
}

// migrateTo builds the destination object for a CR-level migration. It carries
// the source's name, namespace, and spec, retyped to the destination
// apiVersion/kind. Source labels and annotations are kept when preserveSource
// allows, then the declared metadata is applied on top.
func migrateTo(crd *types.CRDEntry, src *unstructured.Unstructured, resolver *template.Resolver) (*unstructured.Unstructured, error) {
	if !crd.HasMigrate() {
		return nil, fmt.Errorf("no migrate.to declared")
	}

	dst := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": crd.Migrate.To.APIVersion,
		"kind":       crd.Migrate.To.Kind,
	}}
	dst.SetName(src.GetName())
	if ns := src.GetNamespace(); ns != "" {
		dst.SetNamespace(ns)
	}

	if spec, ok := src.Object["spec"]; ok {
		dst.Object["spec"] = runtime.DeepCopyJSONValue(spec)
	}

	labels, annotations, err := buildMeta(src, crd.Migrate.To.Metadata, crd.ShouldPreserveMeta(), resolver)
	if err != nil {
		return nil, err
	}
	if len(labels) > 0 {
		dst.SetLabels(labels)
	}
	if len(annotations) > 0 {
		dst.SetAnnotations(annotations)
	}

	return dst, nil
}

// buildMeta merges the source CR's labels and annotations with the declared
// destination metadata, applying template resolution to the declared values.
// Source metadata is copied first when preserve is true, then declared keys
// override on collision.
func buildMeta(
	src *unstructured.Unstructured,
	meta types.CRMigrateMetadata,
	preserve bool,
	resolver *template.Resolver,
) (labels, annotations map[string]string, err error) {
	if labels, err = resolveMetaMap("label", meta.Labels, src.GetLabels(), preserve, resolver); err != nil {
		return nil, nil, err
	}
	if annotations, err = resolveMetaMap("annotation", meta.Annotations, src.GetAnnotations(), preserve, resolver); err != nil {
		return nil, nil, err
	}
	return labels, annotations, nil
}

// resolveMetaMap returns declared merged over source (when preserve), with
// each declared template value resolved against the source.
func resolveMetaMap(
	what string,
	declared map[string]string,
	source map[string]string,
	preserve bool,
	resolver *template.Resolver,
) (map[string]string, error) {
	out := make(map[string]string, len(declared)+len(source))
	if preserve {
		for k, v := range source {
			if isToolingAnnotation(k) {
				continue
			}
			out[k] = v
		}
	}
	resolved, err := resolver.ResolveMap(declared)
	if err != nil {
		return nil, fmt.Errorf("resolve %s %w", what, err)
	}
	for k, v := range resolved {
		out[k] = v
	}
	return out, nil
}

// claim records that cr owns dst's name for the rest of this run.
func (m *migrator) claim(dst, cr *unstructured.Unstructured) {
	if m.claimed != nil {
		m.claimed[core.CRRef(dst)] = core.CRRef(cr)
	}
}
