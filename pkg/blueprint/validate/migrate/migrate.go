package migrate

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/kube"
)

// validateMigrate checks every enabled CRD that declares a CR-level migration.
// It rejects declarations that mix field-level and CR-level migrations, fails
// on structurally invalid declarations, and validates template expressions in
// the destination metadata so they fail at blueprint load rather than at
// migration time.
//
// migrate.to.crdFile has already filled apiVersion, kind and plural at parse
// time. Prune warns because it deletes sources; a missing plural warns
// because RBAC falls back to a naive guess.
func (c *checker) validateMigrate() error {
	funcMap := buildFuncMapForValidation(c.Blueprint.Functions)

	for name, crd := range c.Blueprint.Enabled() {
		if conflict, field := crd.HasConflictingMigrate(); conflict {
			return fmt.Errorf(
				"%s CRD %s: cannot declare both a CR-level migration (migrate.to) and field-level migrations (serve.fields.%s.migrate.from); field migrations evolve a CRD in place, CR migration replaces it. Run the field migration first, then declare migrate.to.",
				failureMark(), crd.Name, field,
			)
		}
		if crd.Migrate == nil {
			continue
		}
		if err := validateSelection(crd.Migrate.Selection); err != nil {
			return fmt.Errorf("%s CRD %s: %w", failureMark(), crd.Name, err)
		}
		if !crd.HasMigrate() {
			// Selection-only block: it scopes field-level migrations.
			switch {
			case crd.Migrate.Prune:
				return fmt.Errorf("%s CRD %s: migrate.prune needs migrate.to", failureMark(), crd.Name)
			case !crd.HasServeFieldMigrate():
				return fmt.Errorf("%s CRD %s: migrate declares no migrate.to and no serve.fields migrate.from; there is nothing to migrate", failureMark(), crd.Name)
			}
			continue
		}

		to := crd.Migrate.To

		// Template values in metadata are evaluated when the migration runs.
		// Validate them here so a malformed expression fails at blueprint load
		// rather than midway through a migration.
		for key, value := range to.Metadata.Annotations {
			if isTemplate(value) {
				if err := validateTemplate("annotations", name, key, "value", value, funcMap); err != nil {
					return err
				}
			}
		}
		for key, value := range to.Metadata.Labels {
			if isTemplate(value) {
				if err := validateTemplate("labels", name, key, "value", value, funcMap); err != nil {
					return err
				}
			}
		}

		// validate the configuration
		if err := validateMigrateConfig(crd.Migrate); err != nil {
			return fmt.Errorf("%s %v", failureMark(), err)
		}

		src, dst := crd.GVK(), crd.Migrate.GVK()
		if src.Group == dst.Group && src.Kind == dst.Kind {
			return fmt.Errorf("%s CRD %s: migrate.to names the source itself (%s); a version change within one kind is a conversion, not a migration", failureMark(), crd.Name, dst.GroupKind())
		}

		if err := checkMigrateScope(&crd); err != nil {
			return fmt.Errorf("%s CRD %s: %w", failureMark(), crd.Name, err)
		}
		if err := checkMigrateSchemas(&crd); err != nil {
			return fmt.Errorf("%s CRD %s: %w", failureMark(), crd.Name, err)
		}

		if crd.Migrate.Prune {
			crd.Warnings.AddWarning(fmt.Sprintf(
				"CRD: %s - migrate.prune is true. Source CRs will be deleted after the destination CR is applied; this is destructive and cannot be undone.",
				crd.Name,
			))
		}

		if crd.Migrate.To.Plural == "" {
			crd.Warnings.AddWarning(fmt.Sprintf(
				"CRD: %s - migrate.to.plural is not declared and no migrate.to.crdFile was provided; RBAC generation will infer %q from kind %q. Set migrate.to.plural explicitly if the destination's plural differs (e.g. ingress → ingresses).",
				crd.Name, kube.PluralForKind(crd.Migrate.To.Kind), crd.Migrate.To.Kind,
			))
		}

		c.Blueprint.EnabledCRDs()[name] = crd
	}
	return nil
}

// validateMigrateConfig checks migrate.to.
func validateMigrateConfig(mig *types.CRMigrate) error {
	to := mig.To
	if to.APIVersion == "" {
		return fmt.Errorf("migrate.to.apiVersion is required")
	}
	if to.Kind == "" {
		return fmt.Errorf("migrate.to.kind is required")
	}
	if to.Namespace != "" {
		if err := isValidK8sName(to.Namespace); err != nil {
			return fmt.Errorf("migrate.to.namespace: %w", err)
		}
	}
	if err := checkReserved("labels", to.Metadata.Labels); err != nil {
		return fmt.Errorf("migrate.to.metadata: %w", err)
	}
	if err := checkReserved("annotations", to.Metadata.Annotations); err != nil {
		return fmt.Errorf("migrate.to.metadata: %w", err)
	}
	// Template values are checked by validateTemplate; check their keys only.
	if err := validateLabelMap(literalValues(to.Metadata.Labels)); err != nil {
		return fmt.Errorf("migrate.to.metadata.labels: %w", err)
	}
	if err := validateAnnotationKeys(to.Metadata.Annotations); err != nil {
		return fmt.Errorf("migrate.to.metadata.annotations: %w", err)
	}

	return nil
}

// literalValues returns m with template values replaced by "" so label checks
// cover their keys but not the unresolved values.
func literalValues(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if isTemplate(v) {
			v = ""
		}
		out[k] = v
	}
	return out
}

// checkMigrateScope checks the source and destination scopes when the
// destination's crdFile is declared. The source's scope comes from its
// crdFile when present, else from namespaced: (default true).
func checkMigrateScope(crd *types.CRDEntry) error {
	to := crd.Migrate.To
	if to.ResolvedCRDFile == "" {
		return nil
	}
	dstNamespaced, err := readCRDNamespaced(to.ResolvedCRDFile)
	if err != nil {
		return fmt.Errorf("migrate.to.crdFile %w", err)
	}
	srcNamespaced := crd.IsNamespaced()
	if crd.ResolvedCRDFile != "" {
		if srcNamespaced, err = readCRDNamespaced(crd.ResolvedCRDFile); err != nil {
			return fmt.Errorf("crdFile %w", err)
		}
	}

	switch {
	case !srcNamespaced && dstNamespaced && to.Namespace == "":
		return fmt.Errorf("migrate.to.namespace is required: the source is cluster-scoped and %s is namespaced", to.Kind)
	case srcNamespaced && !dstNamespaced:
		crd.Warnings.AddWarning(fmt.Sprintf(
			"CRD: %s - %s is cluster-scoped; source namespaces are dropped, and sources with the same name in different namespaces will fail as collisions.",
			crd.Name, to.Kind,
		))
	}
	if to.Namespace != "" && (srcNamespaced || !dstNamespaced) {
		crd.Warnings.AddWarning(fmt.Sprintf(
			"CRD: %s - migrate.to.namespace is ignored; it applies only when a cluster-scoped source moves to a namespaced destination.",
			crd.Name,
		))
	}
	return nil
}

// checkMigrateSchemas rejects a destination schema that cannot hold every
// source field, when both crdFiles are declared. The server would drop or
// reject those fields, so every CR would fail at migration time.
func checkMigrateSchemas(crd *types.CRDEntry) error {
	to := crd.Migrate.To
	if crd.ResolvedCRDFile == "" || to.ResolvedCRDFile == "" {
		return nil
	}
	src, err := readCRDSchema(crd.ResolvedCRDFile)
	if err != nil {
		return fmt.Errorf("crdFile %w", err)
	}
	dst, err := readCRDSchema(to.ResolvedCRDFile)
	if err != nil {
		return fmt.Errorf("migrate.to.crdFile %w", err)
	}
	if src == nil || dst == nil {
		return nil
	}
	if problems := compareSchemas(src, dst); len(problems) > 0 {
		return fmt.Errorf("%s cannot hold every source field:\n  spec.%s", to.Kind, strings.Join(problems, "\n  spec."))
	}
	return nil
}

// validateSelection checks migrate.selection, used at either level.
func validateSelection(sel types.MigrateSelection) error {
	if sel.Selector != "" {
		if _, err := validateLabelSelector(sel.Selector); err != nil {
			return fmt.Errorf("migrate.selection.selector: %w", err)
		}
	}
	if sel.Name != "" {
		if err := isValidK8sName(sel.Name); err != nil {
			return fmt.Errorf("migrate.selection.name: %w", err)
		}
	}
	for i, ns := range sel.Namespaces {
		if err := isValidK8sName(ns); err != nil {
			return fmt.Errorf("migrate.selection.namespaces[%d]: %w", i, err)
		}
	}
	for _, cond := range append(append([]types.Condition{}, sel.When...), sel.Or...) {
		if err := cond.CheckComparison(); err != nil {
			return fmt.Errorf("migrate.selection condition on %s %v", cond.Field, err)
		}
	}
	return nil
}
