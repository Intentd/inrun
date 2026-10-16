package migrate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/utils"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	setNested     = utils.SetNestedPath
	getNested     = utils.GetNestedPath
	deletetNested = utils.DeleteNestedPath
)

// jsonNormalize round-trips a value through encoding/json, normalising all
// numeric types to float64. This ensures values from the Kubernetes API
// (int64 for integer CRD fields) compare correctly with values decoded from
// annotation JSON strings (float64 via encoding/json).
func jsonNormalize(v interface{}) interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out interface{}
	err = json.Unmarshal(b, &out)
	if err != nil {
		return v
	}
	return out
}

// parseSnapshotEntry decodes a snapshot map entry into its before and after
// flat maps. Both must be {"before": map, "after": map} — any other shape
// returns ok=false and the field is skipped during rollback.
func parseSnapshotEntry(raw interface{}) (before, after map[string]interface{}, ok bool) {
	m, isMap := raw.(map[string]interface{})
	if !isMap {
		return nil, nil, false
	}
	beforeRaw, hasBefore := m["before"]
	afterRaw, hasAfter := m["after"]
	if !hasBefore || !hasAfter {
		return nil, nil, false
	}
	beforeMap, bOk := beforeRaw.(map[string]interface{})
	afterMap, aOk := afterRaw.(map[string]interface{})
	if !bOk || !aOk {
		return nil, nil, false
	}
	return beforeMap, afterMap, true
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
	if crd.Serve == nil {
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

// sparseSpec builds a sparse spec map for a merge-patch containing only the
// named fields. writePath returns the key to copy from spec and write to the
// patch; nullPath returns the key to explicitly null (skipped when equal to
// writePath — same-key transforms don't need a null entry).
func sparseSpec(spec map[string]interface{}, fields []migratableField, names []string, writePath, nullPath func(migratableField) string) map[string]interface{} {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	out := map[string]interface{}{}
	for _, f := range fields {
		if !nameSet[f.Name] {
			continue
		}
		wp, np := writePath(f), nullPath(f)
		val, _ := getNested(spec, wp)
		setNested(out, wp, val)
		if wp != np {
			setNested(out, np, nil)
		}
	}
	return out
}

// specOf builds the sparse spec for a forward migration patch.
func specOf(spec map[string]interface{}, fields []migratableField, migrated []string) map[string]interface{} {
	return sparseSpec(spec, fields, migrated,
		func(f migratableField) string { return f.SpecPath },
		func(f migratableField) string { return f.MigrateFrom },
	)
}

// fragmentDrifted reports whether any non-nil path in afterFrag differs from
// the current spec value at that path. Nil-valued entries represent paths that
// were cleared by migration; their absence from the live spec is expected and
// not treated as drift.
func fragmentDrifted(currentSpec map[string]interface{}, afterFrag map[string]interface{}) bool {
	for path, expected := range afterFrag {
		if expected == nil {
			continue
		}
		current, found := getNested(currentSpec, path)
		if !found {
			// Path was written by migration but is now absent — drift.
			return true
		}
		// Round-trip through JSON to normalise numeric types: the Kubernetes API
		// returns integers as int64, but encoding/json decodes JSON numbers as
		// float64. Without normalisation, int64(5) != float64(5) under
		// reflect.DeepEqual and every integer field would falsely appear drifted.
		if !reflect.DeepEqual(jsonNormalize(current), jsonNormalize(expected)) {
			return true
		}
	}
	return false
}

// metaRef formats a namespace/name pair.
func metaRef(namespace, name string) string {
	if namespace != "" {
		return namespace + "/" + name
	}
	return name
}

// crRef formats a CR as "namespace/name", or just "name" when cluster-scoped.
func crRef(cr *unstructured.Unstructured) string {
	return metaRef(cr.GetNamespace(), cr.GetName())
}
