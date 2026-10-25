package field

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

func (m *migrator) migrateCR(
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
	snapshotJSON, err := mergeSnapshot(cr, snapshot)
	if err != nil {
		return nil, nil, err
	}

	patch := buildFieldMigrationPatch(cr, spec, fields, migrated, snapshotJSON, time.Now())

	if err := m.Patch(ctx, resource, cr, patch); err != nil {
		return migrated, spec, fmt.Errorf("apply migration patch: %w", err)
	}

	return migrated, spec, nil
}

// prepareField reads the old value from spec at f.MigrateFrom and stages the
// in-memory delete. Returns (oldVal, true) if the field needs migration, or
// (nil, false) to skip.
//
// The delete is done here — ApplyFieldTransform requires the target key to be
// clear before writing nested values, and reading + deleting are inseparable.
// prepareAndTransform reads the old value, applies the field transform, and
// nulls the old path when it differs from the write destination. Returns
// (oldVal, true, nil) if the field was migrated — oldVal is the raw value
// before any transform, used to build the rollback snapshot. Returns
// (nil, false, nil) if skipped (migrate.from absent or already in new shape).
func prepareAndTransform(resolver *template.Resolver, spec map[string]interface{}, f migratableField) (interface{}, bool, error) {
	oldVal, ok := prepareField(spec, f)
	if !ok {
		return nil, false, nil
	}
	if err := intent.ApplyFieldTransform(resolver, spec, f.Name, f.Config, oldVal); err != nil {
		return nil, false, fmt.Errorf("transform %q: %w", f.Name, err)
	}
	// merge patch only sets present keys; if the old path differs from the
	// actual write destination, explicitly null it so the server removes it.
	if f.MigrateFrom != f.SpecPath {
		setNested(spec, f.MigrateFrom, nil)
	}
	return oldVal, true, nil
}

func prepareField(spec map[string]interface{}, f migratableField) (interface{}, bool) {
	oldVal, found := getNested(spec, f.MigrateFrom)
	if !found {
		return nil, false
	}
	// When migrate.from equals the actual write destination, the transform
	// replaces the old scalar with a structured value at the same key (fanout).
	// If the value is already a map or slice, migration has already run — skip.
	if f.MigrateFrom == f.SpecPath {
		switch oldVal.(type) {
		case map[string]interface{}, []interface{}:
			return nil, false
		}
	}
	deletetNested(spec, f.MigrateFrom)
	return oldVal, true
}

// migratableField is a serve field that carries a migrate.from declaration.
type migratableField struct {
	Name        string // current field name in the serve spec
	SpecPath    string // actual write destination (Config.SpecPath(Name))
	MigrateFrom string // old spec path the value is read from (spec. prefix stripped)
	Config      types.ServeFieldConfig
}

// migratableFields returns all serve fields in crd that declare migrate.from.
func migratableFields(crd *types.CRDEntry) []migratableField {
	if !crd.HasServeFieldMigrate() {
		return nil
	}
	var out []migratableField
	for name, cfg := range crd.Serve.Fields {
		if cfg.HasMigrate() {
			from := strings.TrimPrefix(cfg.MigrateFrom(), "spec.")
			out = append(out, migratableField{
				Name:        name,
				SpecPath:    cfg.SpecPath(name),
				MigrateFrom: from,
				Config:      cfg,
			})
		}
	}
	return out
}
