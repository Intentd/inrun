package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kubeutil "github.com/intentd/inrun/pkg/util/kube"

	"github.com/intentd/inrun/pkg/config"
	client "github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// Migrator applies serve field schema migrations to live CRs.
//
// For each field with migrate.from declared in the Catalog, it:
//  1. reads the old value from the migrate.from spec path
//  2. runs the field's value/values transform (if any) — same logic as apply
//  3. writes the result to the field's current spec path
//  4. removes the migrate.from path
//
// Migration is idempotent: CRs where migrate.from is absent are skipped.
// Merge patch (RFC 7396) is used — SSA would reject the live object if it is
// still in the old schema shape.
type Migrator struct {
	Kube      client.Interface
	Functions types.KELFunctionRegistry
	Mode      MigrateMode
	DryRun    bool
	// Strict causes runFor to return an error if any individual CR fails to
	// migrate. When false (the default), per-CR failures are logged as warnings
	// and migration continues — useful for inspecting partial failures without
	// blocking a deployment.
	Strict bool
	// PageSize controls how many CRs are fetched per List call. 0 means no
	// explicit limit (the API server decides). Set a small value in tests to
	// exercise the pagination loop with few objects.
	PageSize int64
	// Namespaces restricts migration to the given namespaces. Empty means all
	// namespaces. Ignored for cluster-scoped CRDs.
	Namespaces []string
	// Name, when set, restricts runFor to the single CR with this name. Works across
	// all namespaces for namespaced CRDs; no-op for cluster-scoped CRDs if the
	// name does not match.
	Name string
	// Selector filters CRs by a Kubernetes label selector (e.g. "tier=gold",
	// "env in (staging,dev)"). Empty means no filtering. Validated with
	// labels.ValidateLabelSelector before use.
	Selector string
	// Log is the logger for the migrator's own diagnostics. Nil uses the
	// default (the process logger). Pass zerolog.Nop() to silence the
	// migrator entirely — for CLI commands that render their own output.
	Log *zerolog.Logger
}

// CRMigrateResult holds the outcome for one CR.
type CRMigrateResult struct {
	Namespace     string
	Name          string
	Fields        []string               // field names that were (or would be) migrated/rolled back
	DriftedFields []string               // fields skipped because their value changed after migration
	AllDrifted    bool                   // snapshot present, every field drifted — nothing restored
	AfterSpec     map[string]interface{} // resulting spec after migration (nil if skipped/failed)
	Skipped       bool                   // migrate.from absent or no snapshot — nothing to do
	Err           error                  // non-nil only in strict mode or for fatal per-CR errors
}

// Ref returns the CR's "namespace/name", or just "name" when cluster-scoped.
func (r CRMigrateResult) Ref() string {
	return metaRef(r.Namespace, r.Name)
}

// KindMigrateResult holds the outcome for all CRs of one kind.
type KindMigrateResult struct {
	Kind string
	CRs  []CRMigrateResult
}

// MigrateMode identifies the operation a Migrator performs on custom resources.
type MigrateMode int

const (
	// ModeForward migrates custom resources to the current schema.
	ModeForward MigrateMode = iota

	// ModeRollback restores migrated values from their saved snapshots.
	ModeRollback

	// ModeClean removes migration annotations (inrun.dev/migrate-snapshot and
	// inrun.dev/migrate-timestamp) without modifying the spec.
	ModeClean
)

// String returns the mode's CLI name: "forward", "rollback", or "clean".
// It matches the value used in JSON output and the CLI's --output=json
// report, so a string form of the mode is stable across all surfaces.
func (m MigrateMode) String() string {
	switch m {
	case ModeForward:
		return "forward"
	case ModeRollback:
		return "rollback"
	case ModeClean:
		return "clean"
	default:
		return fmt.Sprintf("MigrateMode(%d)", int(m))
	}
}

// New returns a Migrator configured for the requested migration mode.
// Common options are applied consistently across forward, rollback, and clean runs.
func New(ctx context.Context, kube client.Interface, funcs types.KELFunctionRegistry, mode MigrateMode, opts MigratorOpts) *Migrator {
	m := &Migrator{
		Kube:       kube,
		Functions:  funcs,
		Mode:       mode,
		DryRun:     opts.DryRun,
		Strict:     opts.Strict,
		PageSize:   opts.PageSize,
		Namespaces: opts.Namespaces,
		Name:       opts.Name,
		Selector:   opts.Selector,
		Log:        logger.FromContext(ctx),
	}

	if opts.Log != nil {
		m.Log = opts.Log
	}
	return m
}

// MigratorOpts carries the common configuration shared by forward rollback and clean runs.
type MigratorOpts struct {
	DryRun     bool
	Strict     bool
	PageSize   int64
	Namespaces []string
	Name       string
	Selector   string
	Log        *zerolog.Logger
}

// runFor migrates all CRs of the GVK described by crd, using the
// migrate.from paths declared on its serve fields.
func (m *Migrator) runFor(ctx context.Context, crd *types.CRDEntry) (*KindMigrateResult, error) {
	gvk := crd.GVK()
	log := m.Log.With().
		Str("kind", gvk.Kind).
		Bool("dryRun", m.DryRun).
		Logger()

	// Pre-flight: the CRD must be installed before anything else. If it isn't,
	// every subsequent List and Patch will fail with a misleading error; fail
	// here with a message that names the CRD.
	if _, err := kubeutil.CheckCRDExists(m.Kube.RestConfig(), &gvk); err != nil {
		return nil, err
	}

	result := &KindMigrateResult{Kind: gvk.Kind}

	fields := migratableFields(crd)
	if len(fields) == 0 {
		log.Debug().Msg("no migrate.from fields — nothing to do")
		return result, nil
	}

	resource, clusterScoped, err := client.DynamicResourceFor(m.Kube, gvk)
	if err != nil {
		return result, fmt.Errorf("resolve GVR for %s: %w", gvk.Kind, err)
	}

	if clusterScoped && len(m.Namespaces) > 0 {
		log.Warn().Strs("namespaces", m.Namespaces).
			Msg("--namespace ignored: CRD is cluster-scoped")
	}

	// Page through all CRs; a single List may be truncated on large clusters.
	// When Namespaces is set, list each namespace separately so the API server
	// does the scoping rather than fetching all CRs and discarding.
	opts := metav1.ListOptions{
		Limit:         m.PageSize,
		LabelSelector: m.Selector,
	}
	if m.Name != "" {
		opts.FieldSelector = "metadata.name=" + m.Name
	}
	allItems, err := client.ListAllForNamespaces(ctx, resource, clusterScoped, m.Namespaces, opts)
	if err != nil {
		return result, fmt.Errorf("list %s: %w", gvk.Kind, err)
	}

	var strictErrs []string
	for i := range allItems {
		crResult, strictErr := m.processCR(ctx, &allItems[i], resource, fields, log)
		result.CRs = append(result.CRs, crResult)
		if strictErr != "" {
			strictErrs = append(strictErrs, strictErr)
		}
	}

	if len(strictErrs) > 0 {
		return result, fmt.Errorf("migration errors:\n%s", strings.Join(strictErrs, "\n"))
	}

	return result, nil
}

// processCR handles a single CR: skips terminating objects, calls migrateCR,
// and builds the result. Returns a non-empty strictErr string when Strict mode
// is on and the CR failed — the caller accumulates these for a final error.
func (m *Migrator) processCR(
	ctx context.Context,
	cr *unstructured.Unstructured,
	resource dynamic.NamespaceableResourceInterface,
	fields []migratableField,
	log zerolog.Logger,
) (CRMigrateResult, string) {
	if cr.GetDeletionTimestamp() != nil {
		log.Debug().Str("cr", crRef(cr)).Msg("skipping terminating CR")
		return CRMigrateResult{Namespace: cr.GetNamespace(), Name: cr.GetName(), Skipped: true}, ""
	}

	var changedFields, driftedFields []string
	var afterSpec map[string]interface{}
	var err error
	switch m.Mode {
	case ModeClean:
		changedFields, err = m.cleanCR(ctx, cr, resource)
	case ModeRollback:
		changedFields, driftedFields, afterSpec, err = m.rollbackCR(ctx, cr, resource, fields)
	default:
		changedFields, afterSpec, err = m.migrateCR(ctx, cr, resource, fields)
	}
	result := CRMigrateResult{
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
		AllDrifted: m.Mode == ModeRollback && len(changedFields) == 0 && len(driftedFields) > 0 && err == nil,
	}
	if len(changedFields) > 0 {
		result.Fields = changedFields
		result.AfterSpec = afterSpec
	}
	if err != nil {
		result.Err = err
		if m.Strict {
			return result, fmt.Sprintf("%s: %s", crRef(cr), err)
		}
		log.Warn().Str("cr", crRef(cr)).Err(err).Msg("migration skipped")
	}
	return result, ""
}

func (m *Migrator) migrateCR(
	ctx context.Context,
	cr *unstructured.Unstructured,
	resource dynamic.NamespaceableResourceInterface,
	fields []migratableField,
) ([]string, map[string]interface{}, error) {
	spec, _, _ := unstructured.NestedMap(cr.Object, "spec")
	if spec == nil {
		spec = map[string]interface{}{}
	}

	resolver := template.NewResolverFromMap(spec).WithUserFunctions(m.Functions)

	var migrated []string
	snapshot := map[string]interface{}{}
	for _, f := range fields {
		oldVal, ok, err := prepareAndTransform(resolver, spec, f)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			migrated = append(migrated, f.Name)
			snapshot[f.MigrateFrom] = buildSnapshotEntry(spec, f, oldVal)
		}
	}

	if len(migrated) == 0 {
		return nil, nil, nil
	}

	// Marshal the snapshot before building the patch: if serialization fails the
	// migration must not proceed — a patch without the recovery annotation would
	// leave the CR mutated with no rollback path.
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal migration snapshot: %w", err)
	}

	patch := buildMigrationPatch(cr, spec, fields, migrated, snapshotJSON, time.Now())

	if err := m.patchCR(ctx, resource, cr, patch); err != nil {
		return migrated, spec, fmt.Errorf("apply migration patch: %w", err)
	}

	return migrated, spec, nil
}

// rollbackCR restores a CR from its migrate-snapshot annotation. For each
// field in the snapshot it first checks whether the current spec value is still
// the direct product of the original migration (catalog transform applied to
// the snapshot value). Fields whose value changed after migration are reported
// as drifted and left untouched.
//
// Returns (restored, drifted, afterSpec, error). All three slices are nil when
// the CR carries no snapshot (never migrated or already rolled back).
func (m *Migrator) rollbackCR(
	ctx context.Context,
	cr *unstructured.Unstructured,
	resource dynamic.NamespaceableResourceInterface,
	fields []migratableField,
) (restored []string, drifted []string, afterSpec map[string]interface{}, err error) {
	snapshotJSON := cr.GetAnnotations()[labels.AnnotationMigrateSnapshot]
	if snapshotJSON == "" {
		return nil, nil, nil, nil
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return nil, nil, nil, fmt.Errorf("decode snapshot: %w", err)
	}

	spec, _, _ := unstructured.NestedMap(cr.Object, "spec")
	if spec == nil {
		spec = map[string]interface{}{}
	}

	// Start with every snapshot entry retained; delete only what is successfully
	// restored. Entries for fields no longer in the Catalog (removed declarations,
	// renamed fields) are preserved automatically — Catalog changes do not destroy
	// recovery records for fields this rollback does not touch.
	remainingSnapshot := make(map[string]interface{}, len(snapshot))
	for k, v := range snapshot {
		remainingSnapshot[k] = v
	}

	var restoredFrags []map[string]interface{}
	for _, f := range fields {
		raw, ok := snapshot[f.MigrateFrom]
		if !ok {
			continue
		}
		beforeFrag, afterFrag, ok := parseSnapshotEntry(raw)
		if !ok {
			continue
		}
		if fragmentDrifted(spec, afterFrag) {
			// Confirmed drifted: leave this entry in remainingSnapshot so the
			// recovery record is preserved for a future rollback attempt.
			drifted = append(drifted, f.Name)
			continue
		}
		for path, val := range beforeFrag {
			setNested(spec, path, val)
		}
		restoredFrags = append(restoredFrags, beforeFrag)
		restored = append(restored, f.Name)
		delete(remainingSnapshot, f.MigrateFrom)
	}

	if len(restored) == 0 {
		return nil, drifted, nil, nil
	}

	patch, patchBuildErr := buildRollbackPatch(cr, restoredFrags, remainingSnapshot)
	if patchBuildErr != nil {
		return nil, drifted, nil, patchBuildErr
	}
	if err := m.patchCR(ctx, resource, cr, patch); err != nil {
		return restored, drifted, spec, fmt.Errorf("apply rollback patch: %w", err)
	}

	return restored, drifted, spec, nil
}

// cleanCR removes the migration annotations from cr without touching the spec.
// Returns a non-nil fields slice when annotations were present (or dry-run would
// remove them); nil when the CR carries no migration annotations.
func (m *Migrator) cleanCR(
	ctx context.Context,
	cr *unstructured.Unstructured,
	resource dynamic.NamespaceableResourceInterface,
) ([]string, error) {
	ann := cr.GetAnnotations()
	_, hasSnapshot := ann[labels.AnnotationMigrateSnapshot]
	_, hasTimestamp := ann[labels.AnnotationMigrateTimestamp]
	if !hasSnapshot && !hasTimestamp {
		return nil, nil
	}

	patch := buildCleanPatch(cr)
	if err := m.patchCR(ctx, resource, cr, patch); err != nil {
		return []string{"annotations"}, fmt.Errorf("apply clean patch: %w", err)
	}

	return []string{"annotations"}, nil
}

// patchCR uses MergePatchType (RFC 7396) — does not type-validate the live object against
// the CRD schema, so it works when the existing CR is in the old (invalid) shape.
// SSA would fail here because it tries to reconcile the live object with the schema before merging.
func (m *Migrator) patchCR(
	ctx context.Context,
	resource dynamic.NamespaceableResourceInterface,
	cr *unstructured.Unstructured,
	patch interface{},
) error {
	patchBody, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}

	opts := metav1.PatchOptions{
		FieldManager: config.FieldManagerGateway,
	}
	if m.DryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}

	_, err = client.PatchUnstructured(ctx, resource, cr, k8stypes.MergePatchType, patchBody, opts)
	return err
}
