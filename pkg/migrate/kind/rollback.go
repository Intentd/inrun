package kind

import (
	"context"
	"encoding/json"
	"fmt"

	client "github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/migrate/core"
	"github.com/rs/zerolog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type rollbackOutcome int

const (
	rollbackNone     rollbackOutcome = iota // no snapshot
	rollbackRestored                        // source back, destination deleted
	rollbackDrifted                         // destination edited after migration; left alone
)

// rollbackCRKind undoes a CR-level migration from the destination's snapshot.
// The source is recreated before the destination is deleted, so a failed
// create leaves the destination and its snapshot in place.
func (m *migrator) rollback(
	ctx context.Context,
	dst *unstructured.Unstructured,
) (rollbackOutcome, map[string]interface{}, error) {
	raw := dst.GetAnnotations()[labels.AnnotationMigrateKindSnapshot]
	if raw == "" {
		return rollbackNone, nil, nil
	}

	var snap kindSnapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		return rollbackNone, nil, fmt.Errorf("decode snapshot: %w", err)
	}

	if destinationDrifted(dst, &snap) {
		return rollbackDrifted, nil, nil
	}

	// Always: a later run may have pruned the source after this snapshot was
	// written. recreateSource skips a source that still exists.
	if err := m.recreateSource(ctx, &snap); err != nil {
		return rollbackNone, nil, fmt.Errorf("recreate source: %w", err)
	}

	// Precondition: fail rather than delete a destination edited after the
	// drift check above.
	if err := m.Kube.Delete(ctx, dst, metav1.DeleteOptions{DryRun: m.DryRunOpt(), Preconditions: unchanged(dst)}); err != nil {
		return rollbackNone, nil, fmt.Errorf("delete destination: %w", err)
	}
	return rollbackRestored, snap.Source.Spec, nil
}

// destinationDrifted reports whether a source field changed or went missing on
// the destination. Fields the server added (defaults) are not drift.
func destinationDrifted(dst *unstructured.Unstructured, snap *kindSnapshot) bool {
	current, _, _ := unstructured.NestedMap(dst.Object, "spec")
	return len(missingPaths(snap.Source.Spec, current)) > 0
}

// recreateSource creates the source from the snapshot. A source that already
// exists (prune failed, or an earlier rollback restored it) is left as is.
func (m *migrator) recreateSource(ctx context.Context, snap *kindSnapshot) error {
	gvk := schema.FromAPIVersionAndKind(snap.Source.APIVersion, snap.Source.Kind)
	gvr, err := client.GVRForGVK(m.Kube, gvk)
	if err != nil {
		return fmt.Errorf("resolve source GVR: %w", err)
	}

	meta := snap.Source.Metadata
	resource := m.Kube.DynamicClient().Resource(gvr)
	var ri dynamic.ResourceInterface = resource
	if meta.Namespace != "" {
		ri = resource.Namespace(meta.Namespace)
	}

	_, err = ri.Get(ctx, meta.Name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get source: %w", err)
	}

	created, err := ri.Create(ctx, snap.sourceObject(), metav1.CreateOptions{DryRun: m.DryRunOpt()})
	if err != nil {
		return fmt.Errorf("create source: %w", err)
	}
	if err := m.writeStatus(ctx, ri, created, snap.Source.Status); err != nil {
		return fmt.Errorf("restore source status: %w", err)
	}
	return nil
}

func (m *migrator) processRollback(
	ctx context.Context,
	dst *unstructured.Unstructured,
	log zerolog.Logger,
) (core.CRResult, string) {
	// Report the source being restored, not the destination: after a scope
	// change the two differ.
	ref := dst
	if src, ok := snapshotSource(dst); ok {
		ref = &unstructured.Unstructured{}
		ref.SetNamespace(src.Metadata.Namespace)
		ref.SetName(src.Metadata.Name)
	}

	outcome, afterSpec, err := m.rollback(ctx, dst)
	if err != nil {
		return m.Fail(ref, err, log)
	}

	result := core.CRResult{Namespace: ref.GetNamespace(), Name: ref.GetName()}
	switch outcome {
	case rollbackRestored:
		result.Fields = []string{"spec"}
		result.AfterSpec = afterSpec
	case rollbackDrifted:
		result.AllDrifted = true
	default:
		result.Skipped = true
	}
	return result, ""
}

// listMigrated lists the destinations whose snapshot names gvk as the source
// and whose recorded source metadata is in the effective selection. Labels and
// namespace are matched on the snapshot, since the destination's may differ.
func (m *migrator) listMigrated(
	ctx context.Context,
	sel core.Selection,
	gvk schema.GroupVersionKind,
	dstResource dynamic.NamespaceableResourceInterface,
	dstClusterScoped bool,
) ([]unstructured.Unstructured, error) {

	items, err := client.ListAllForNamespaces(ctx, dstResource, dstClusterScoped, nil,
		metav1.ListOptions{Limit: m.PageSize})
	if err != nil {
		return nil, fmt.Errorf("list destinations: %w", err)
	}

	var out []unstructured.Unstructured
	for _, item := range items {
		raw := item.GetAnnotations()[labels.AnnotationMigrateKindSnapshot]
		if raw == "" {
			continue
		}
		var snap kindSnapshot
		if err := json.Unmarshal([]byte(raw), &snap); err != nil {
			// Keep it so the per-CR step reports the bad snapshot.
			out = append(out, item)
			continue
		}
		if !snap.isFrom(gvk) {
			continue
		}
		src := snap.Source.Metadata
		ok, err := sel.Includes(src.Name, src.Namespace, src.Labels)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}
