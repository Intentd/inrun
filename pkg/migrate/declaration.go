package migrate

import (
	"fmt"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kome"
	"github.com/intentd/kome/pkg/declaration"
)

// Declaration is the KOME file bp's migrations amount to,
// with the schema and CR files they declare: migrate.from,
// migrate.transitional and migrate.to (crdFile, crFiles). The side that is
// the CRD itself defaults to its crdFile: to at field level, from at kind
// level. Paths are the resolved ones, so the file has no directory. Only CRDs
// of kind (when set) are kept.
func Declaration(bp *blueprint.Blueprint, kind string) (*declaration.File, error) {
	f := &declaration.File{Migrations: map[string]declaration.Migration{}}
	for _, name := range bp.CRDNames() {
		entry, ok := bp.CRDEntry(name)
		if !ok || !entry.HasAnyMigrate() {
			continue
		}
		if kind != "" && entry.GVK().Kind != kind {
			continue
		}
		f.Migrations[name] = declarationFor(&entry)
	}
	if kind != "" && len(f.Migrations) == 0 {
		return nil, fmt.Errorf("no CRD of kind %q declares a migration", kind)
	}
	return f, nil
}

// Transitional returns the transitional schema of bp's CRD name's field
// migration, generated in memory by KOME from its declaration: crdFile with
// the old path of every move still declared, so CRs keep their values while
// they move, and those of the migrations it needs (a value moved twice). ok is false when the CRD has no field migration, declares its
// migrate.transitional (applied from its file), or lacks migrate.from's
// crdFile or its own.
func Transitional(bp *blueprint.Blueprint, name string) (schema []byte, ok bool, err error) {
	entry, found := bp.CRDEntry(name)
	if !found || !entry.HasServeFieldMigrate() || entry.HasMigrate() {
		return nil, false, nil
	}
	if mig := entry.Migrate; mig != nil && mig.Transitional != nil && mig.Transitional.CRDFile != "" {
		return nil, false, nil
	}
	d := declarationFor(&entry)
	if d.From.Schema == "" || d.To.Schema == "" {
		return nil, false, nil
	}
	f, err := Declaration(bp, "")
	if err != nil {
		return nil, false, err
	}
	schema, err = kome.Transitional(f, name)
	if err != nil {
		return nil, false, fmt.Errorf("CRD %q: %w", name, err)
	}
	return schema, true, nil
}

func declarationFor(crd *types.CRDEntry) declaration.Migration {
	var d declaration.Migration
	mig := crd.Migrate
	if mig != nil {
		s := mig.Selection
		d.Selection = declaration.Selection{
			Namespaces: s.Namespaces, Name: s.Name, Selector: s.Selector,
			When: s.When.Unwrap(), Or: s.Or.Unwrap(),
		}
		if from := mig.From; from != nil {
			d.From = declaration.Shape{Schema: from.ResolvedCRDFile, Objects: from.ResolvedCRFiles}
		}
		d.To.Objects = mig.To.ResolvedCRFiles
		d.Needs = mig.Needs
	}

	if crd.HasMigrate() {
		to := mig.To
		if d.From.Schema == "" {
			d.From.Schema = crd.ResolvedCRDFile
		}
		if d.From.Schema == "" {
			d.From.APIVersion, d.From.Kind, d.From.Plural = crd.APIVersion(), crd.Kind(), crd.APITypes.Plural
		}
		d.To.Schema = to.ResolvedCRDFile
		if d.To.Schema == "" {
			d.To.APIVersion, d.To.Kind, d.To.Plural = to.APIVersion, to.Kind, to.Plural
		}
		d.To.Namespace, d.To.CopyStatus = to.Namespace, to.CopyStatus
		if l := mig.Lifecycle; l != nil {
			d.Lifecycle = &declaration.Lifecycle{Prune: l.Prune, Children: l.Children, Finalizers: l.Finalizers}
		}
		md := to.Metadata
		if md.PreserveSource != nil || len(md.Labels) > 0 || len(md.Annotations) > 0 {
			d.To.Metadata = &declaration.Metadata{PreserveSource: md.PreserveSource, Labels: md.Labels, Annotations: md.Annotations}
		}
		return d
	}

	// Field level: the CRD's own schema is the destination.
	d.To.Schema = crd.ResolvedCRDFile
	if d.To.Schema == "" {
		d.To.APIVersion, d.To.Kind, d.To.Plural = crd.APIVersion(), crd.Kind(), crd.APITypes.Plural
	}
	if d.From.Schema == "" {
		d.From.APIVersion, d.From.Kind, d.From.Plural = crd.APIVersion(), crd.Kind(), crd.APITypes.Plural
	}
	if mig != nil && mig.Transitional != nil {
		d.Transitional = &declaration.Transitional{Schema: mig.Transitional.ResolvedCRDFile}
	}
	d.Spec = map[string]declaration.Field{}
	for _, key := range crd.Serve.FieldNames() {
		cfg := crd.Serve.Fields[key]
		if !cfg.HasMigrate() {
			continue
		}
		fl := declaration.Field{From: cfg.Migrate.SpecPath(), Value: cfg.Value, Values: cfg.Values}
		if len(cfg.Values) == 0 {
			fl.To = cfg.SpecPath(key)
		}
		d.Spec[key] = fl
	}
	return d
}
