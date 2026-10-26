// Package field migrates CRs in place: each serve field with migrate.from has
// its old value read, transformed like the gateway does on apply, and written
// to its current path. A snapshot annotation makes it reversible.
package field

import (
	"context"
	"fmt"

	client "github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/migrate/core"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// runForFields migrates CRs of this CRD in place, using the migrate.from
// paths declared on its serve fields.
func (m *migrator) run(ctx context.Context, crd *types.CRDEntry, gvk schema.GroupVersionKind, log zerolog.Logger) (*core.KindResult, error) {
	result := &core.KindResult{Kind: gvk.Kind}

	fields := migratableFields(crd)
	if len(fields) == 0 {
		log.Debug().Msg("no migrate.from fields — nothing to do")
		return result, nil
	}

	resource, clusterScoped, err := client.DynamicResourceFor(m.Kube, gvk)
	if err != nil {
		return result, fmt.Errorf("resolve GVR for %s: %w", gvk.Kind, err)
	}

	sel, err := m.Selection(crd)
	if err != nil {
		return result, err
	}
	result.Selection = sel

	allItems, err := m.ListSources(ctx, sel, gvk, resource, clusterScoped, log)
	if err != nil {
		return result, err
	}

	var strictErrs []string
	for i := range allItems {
		m.Report(gvk.Kind, i+1, len(allItems))
		cr := &allItems[i]
		if m.Mode == core.ModeForward && !m.Admits(crd, cr) {
			result.CRs = append(result.CRs, core.CRResult{Namespace: cr.GetNamespace(), Name: cr.GetName(), Skipped: true, Excluded: true})
			continue
		}
		crResult, strictErr := m.process(ctx, cr, resource, fields, log)
		result.CRs = append(result.CRs, crResult)
		if strictErr != "" {
			strictErrs = append(strictErrs, strictErr)
		}
	}

	return result, core.StrictError(strictErrs)
}

// processCR handles a single CR: skips terminating objects, calls migrateCR,
// and builds the result. Returns a non-empty strictErr string when Strict mode
// is on and the CR failed — the caller accumulates these for a final error.
func (m *migrator) process(
	ctx context.Context,
	cr *unstructured.Unstructured,
	resource dynamic.NamespaceableResourceInterface,
	fields []migratableField,
	log zerolog.Logger,
) (core.CRResult, string) {
	if cr.GetDeletionTimestamp() != nil {
		log.Debug().Str("cr", core.CRRef(cr)).Msg("skipping terminating CR")
		return core.CRResult{Namespace: cr.GetNamespace(), Name: cr.GetName(), Skipped: true}, ""
	}

	var changedFields, driftedFields []string
	var afterSpec map[string]interface{}
	var err error
	switch m.Mode {
	case core.ModeClean:
		changedFields, err = m.cleanCR(ctx, cr, resource)
	case core.ModeRollback:
		changedFields, driftedFields, afterSpec, err = m.rollbackCR(ctx, cr, resource, fields)
	default:
		changedFields, afterSpec, err = m.migrateCR(ctx, cr, resource, fields)
	}
	result := core.CRResult{
		Namespace:     cr.GetNamespace(),
		Name:          cr.GetName(),
		DriftedFields: driftedFields,

		// Skipped is true when there was nothing to do: no migrate.from fields
		// matched, no snapshot was present, or the CR is terminating. It is false
		// when any work was attempted — including a rollback where every field
		// drifted, which sets AllDrifted instead.
		Skipped: len(changedFields) == 0 && len(driftedFields) == 0 && err == nil,

		// AllDrifted is true when a rollback had a snapshot but every field in it
		// drifted, so nothing was restored. Fields is empty and DriftedFields holds
		// the drifted fields. Only set in rollback mode.
		AllDrifted: m.Mode == core.ModeRollback && len(changedFields) == 0 && len(driftedFields) > 0 && err == nil,
	}
	if len(changedFields) > 0 {
		result.Fields = changedFields
		result.AfterSpec = afterSpec
	}
	if err != nil {
		result.Err = err
		if m.Strict {
			return result, fmt.Sprintf("%s: %s", core.CRRef(cr), err)
		}
		log.Warn().Str("cr", core.CRRef(cr)).Err(err).Msg("migration skipped")
	}
	return result, ""
}

// migrator runs one field-level migration.
type migrator struct{ *core.Runner }

// Run migrates the CRs of crd in place, using the migrate.from paths on its
// serve fields.
func Run(ctx context.Context, r *core.Runner, crd *types.CRDEntry, gvk schema.GroupVersionKind, log zerolog.Logger) (*core.KindResult, error) {
	return (&migrator{r}).run(ctx, crd, gvk, log)
}

var (
	setNested     = structured.SetNestedPath
	getNested     = structured.GetNestedPath
	deletetNested = structured.DeleteNestedPath
	jsonNormalize = structured.JSONNormalize
)
