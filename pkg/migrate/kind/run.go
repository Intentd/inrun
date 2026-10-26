// Package kind moves CRs of one CRD to another (migrate.to): each source is
// rebuilt as a destination CR and server-side applied, with a snapshot
// annotation on the destination that makes it reversible.
package kind

import (
	"context"
	"fmt"

	client "github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/migrate/core"
	"github.com/intentd/inrun/pkg/types"
	kubeutil "github.com/intentd/inrun/pkg/util/kube"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// run migrates CRs of this CRD to the destination CRD declared by
// migrate.to. Each source CR is rebuilt as a destination CR (name, namespace,
// spec carried; labels and annotations per Metadata) and server-side applied.
// Prune deletes the source CR after its destination is written.
func (m *migrator) run(ctx context.Context, crd *types.CRDEntry, gvk schema.GroupVersionKind, log zerolog.Logger) (*core.KindResult, error) {
	dstGVK := crd.Migrate.GVK()
	to := dstGVK.Kind
	if to == gvk.Kind {
		to += "." + dstGVK.Group // same kind: the group is what changes
	}
	result := &core.KindResult{Kind: gvk.Kind, To: to}

	if _, err := kubeutil.CheckCRDExists(m.Kube.RestConfig(), &dstGVK); err != nil {
		return result, err
	}

	sel, err := m.Selection(crd)
	if err != nil {
		return result, err
	}
	result.Selection = sel

	// Resolve the source's dynamic resource.
	srcResource, clusterScoped, err := client.DynamicResourceFor(m.Kube, gvk)
	if err != nil {
		return result, fmt.Errorf("resolve GVR for %s: %w", gvk.Kind, err)
	}

	dstResource, dstClusterScoped, err := client.DynamicResourceFor(m.Kube, dstGVK)
	if err != nil {
		return result, fmt.Errorf("resolve destination GVR for %s: %w", dstGVK.Kind, err)
	}

	// Scope compatibility: a namespaced source cannot be applied to a
	// cluster-scoped destination with its namespace intact, and a
	// cluster-scoped source cannot be applied to a namespaced destination
	// without one.
	if clusterScoped && !dstClusterScoped && crd.Migrate.To.Namespace == "" {
		return result, fmt.Errorf(
			"migrate.to.namespace is required: source %s is cluster-scoped, destination %s is namespaced",
			gvk.Kind, dstGVK.Kind,
		)
	}
	if !clusterScoped && dstClusterScoped {
		log.Warn().Msg("source is namespaced, destination is cluster-scoped; namespace will be dropped")
	}

	// Forward reads sources; rollback and clean read the destinations that
	// carry this CRD's snapshot.
	var allItems []unstructured.Unstructured
	switch m.Mode {
	case core.ModeRollback, core.ModeClean:
		allItems, err = m.listMigrated(ctx, sel, gvk, dstResource, dstClusterScoped)
	default:
		allItems, err = m.ListSources(ctx, sel, gvk, srcResource, clusterScoped, log)
	}
	if err != nil {
		return result, err
	}

	var strictErrs []string
	for i := range allItems {
		m.Report(gvk.Kind+" → "+to, i+1, len(allItems))
		crResult, strictErr := m.process(ctx, &allItems[i], crd, dstResource, dstClusterScoped, log)
		result.CRs = append(result.CRs, crResult)
		if strictErr != "" {
			strictErrs = append(strictErrs, strictErr)
		}
	}

	return result, core.StrictError(strictErrs)
}

func (m *migrator) process(
	ctx context.Context,
	cr *unstructured.Unstructured,
	crd *types.CRDEntry,
	dstResource dynamic.NamespaceableResourceInterface,
	dstClusterScoped bool,
	log zerolog.Logger,
) (core.CRResult, string) {
	if cr.GetDeletionTimestamp() != nil {
		log.Debug().Str("cr", core.CRRef(cr)).Msg("skipping terminating CR")
		return core.CRResult{Namespace: cr.GetNamespace(), Name: cr.GetName(), Skipped: true}, ""
	}

	switch m.Mode {
	case core.ModeClean:
		return m.processClean(ctx, cr, dstResource, log)
	case core.ModeRollback:
		return m.processRollback(ctx, cr, log)
	default:
		return m.processForward(ctx, cr, crd, dstResource, dstClusterScoped, log)
	}
}

// migrator runs one kind-level migration.
type migrator struct {
	*core.Runner
	// claimed maps each destination this run writes (or would write) to
	// its source, so two sources landing on one name fail even in dry-run.
	claimed map[string]string
}

// Run moves the CRs of crd to the CRD named by its migrate.to.
func Run(ctx context.Context, r *core.Runner, crd *types.CRDEntry, gvk schema.GroupVersionKind, log zerolog.Logger) (*core.KindResult, error) {
	return newMigrator(r).run(ctx, crd, gvk, log)
}

func newMigrator(r *core.Runner) *migrator {
	return &migrator{Runner: r, claimed: map[string]string{}}
}

var (
	missingPaths  = structured.MissingPaths
	jsonNormalize = structured.JSONNormalize
	copyStringMap = structured.CopyStringMap
	copySpec      = structured.CopySpec
)
