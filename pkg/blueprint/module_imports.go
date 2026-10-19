// pkg/blueprint/module_imports.go
//
// Two import paths:
//
//	spec.imports (Blueprint-wide) — expandBlueprintImports
//	  Merges profiles: and functions: from each Module into the Blueprint-wide registries.
//	  Resources, status, and admission in the Module are ignored at this level.
//
//	spec.crds[name].imports (CRD-scoped) — expandModuleImports
//	  Merges resources, status, and admission into the target CRD.
//	  Profiles and notes in the Module are ignored at this level.
package blueprint

import (
	"fmt"
	"path/filepath"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/registry/module"
	"github.com/intentd/inrun/pkg/types"
)

// expandBlueprintImports resolves spec.imports entries on the Blueprint.
// For each import, profiles: and functions: from the expanded Module are merged
// into the Blueprint-wide ProfileRegistry and KELFunctionRegistry respectively.
// Resources, status, and admission are ignored at this level.
func (c *Blueprint) expandBlueprintImports() error {
	seen := make(map[string]string) // note name → module label, for conflict detection
	for i, imp := range c.Spec.Imports {
		expanded, err := c.loadAndExpandImport(&imp)
		if err != nil {
			return fmt.Errorf("spec.imports[%d]: %w", i, err)
		}
		label := fmt.Sprintf("spec.imports[%d] module %q", i, expanded.Name)
		if !expanded.Profiles.Empty() {
			merged, err := c.Profiles.Merge(expanded.Profiles, label)
			if err != nil {
				return fmt.Errorf("spec.imports[%d]: merging profiles from module %q: %w", i, expanded.Name, err)
			}
			c.Profiles = merged
		}
		if !expanded.Functions.Empty() {
			merged, err := c.Functions.MergeImport(expanded.Functions, label, seen)
			if err != nil {
				return fmt.Errorf("spec.imports[%d]: merging notes from module %q: %w", i, expanded.Name, err)
			}
			c.Functions = merged
		}
	}
	return nil
}

// expandModuleImports resolves all imports entries across enabled CRDs.
// For each import, it expands the module and merges the result into the CRD entry.
// After expansion, the imports list is cleared (they have been inlined).
//
// Note: Imports are defined at the CRD level (spec.crds[].imports),
// not inside operatorBox. This allows modules to contribute to multiple
// aspects of the CRD (resources, status, admission rules) without being
// tied to the operatorbox configuration.
func (c *Blueprint) expandModuleImports() error {
	for name, entry := range c.enabledCRDs {
		if len(entry.EffectiveImports()) == 0 {
			continue
		}

		for i, imp := range entry.EffectiveImports() {
			expanded, err := c.loadAndExpandImport(&imp)
			if err != nil {
				return fmt.Errorf("CRD %q: operatorBox.imports[%d]: %w", name, i, err)
			}
			if err := c.mergeExpandedModule(&entry, expanded); err != nil {
				return fmt.Errorf("CRD %q: operatorBox.imports[%d]: merging module %q: %w",
					name, i, imp.Module, err)
			}
			if !expanded.Profiles.Empty() {
				warning := fmt.Sprintf("CRD %q: import[%d] module %q: profiles: are ignored at CRD-level imports — use spec.imports to apply profiles Blueprint-wide",
					name, i, expanded.Name)
				entry.Warnings.AddWarning(warning)
				c.Warnings.AddWarning(warning)
			}
			if !expanded.Functions.Empty() {
				warning := fmt.Sprintf("CRD %q: import[%d] module %q: functions: are ignored at CRD-level imports — use spec.imports to apply notes Blueprint-wide",
					name, i, expanded.Name)
				entry.Warnings.AddWarning(warning)
				c.Warnings.AddWarning(warning)
			}
		}

		// Clear imports after successful expansion
		if entry.OperatorBox.Reconcile != nil {
			entry.OperatorBox.Reconcile.Imports = nil
		}
		c.enabledCRDs[name] = entry
	}
	return nil
}

// loadAndExpandImport loads the module from its source (file, OCI, Git) and expands
// it using the provided bindings. Returns the expanded module or an error.
//
// Relative file paths (./foo, ../foo, foo.yaml) are resolved against k.blueprintDir
// so the path is stable regardless of which directory inrun simulate / inrun is
// invoked from. This mirrors how crdFile paths are resolved in crdfile.go.
func (c *Blueprint) loadAndExpandImport(imp *types.ModuleImport) (*module.ExpandedModule, error) {
	resolved := imp
	if c.blueprintDir != "" && registry.IsFilePath(imp.Module) && !filepath.IsAbs(imp.Module) {
		copy := *imp
		copy.Module = filepath.Join(c.blueprintDir, imp.Module)
		resolved = &copy
	}
	m, err := module.LoadImport(resolved)
	if err != nil {
		return nil, fmt.Errorf("loading module %q: %w", imp.Module, err)
	}
	expanded, err := module.Expand(m, imp.With)
	if err != nil {
		return nil, fmt.Errorf("expanding module %q: %w", imp.Module, err)
	}
	return expanded, nil
}

// mergeExpandedModule merges the resources, status, and admission rules from an
// expanded module into the target CRD entry. It respects the existing fields
// (appending rules, preserving order, and merging condition flags sensibly).
func (c *Blueprint) mergeExpandedModule(entry *types.CRDEntry, expanded *module.ExpandedModule) error {
	// resources.onCreate: → CRD onCreate (update=false, preserves once: true guard)
	if expanded.OnCreate != nil {
		if entry.OperatorBox.Reconcile == nil {
			entry.OperatorBox.Reconcile = &types.ReconcileConfig{}
		}
		if entry.OperatorBox.Reconcile.OnCreate == nil {
			entry.OperatorBox.Reconcile.OnCreate = &types.HookTemplates{}
		}
		entry.OperatorBox.Reconcile.OnCreate.MergeFrom(expanded.OnCreate)
	}
	// resources flat fields → CRD onReconcile (drift correction)
	if expanded.OnReconcile != nil {
		if entry.OperatorBox.Reconcile == nil {
			entry.OperatorBox.Reconcile = &types.ReconcileConfig{}
		}
		if entry.OperatorBox.Reconcile.OnReconcile == nil {
			entry.OperatorBox.Reconcile.OnReconcile = &types.HookTemplates{}
		}
		entry.OperatorBox.Reconcile.OnReconcile.MergeFrom(expanded.OnReconcile)
	}

	// Merge status fields
	if expanded.HasStatus() {
		if entry.OperatorBox.Emit == nil {
			entry.OperatorBox.Emit = &types.EmitConfig{}
		}
		if entry.OperatorBox.Emit.Status == nil {
			entry.OperatorBox.Emit.Status = &types.StatusConfig{}
		}
		entry.OperatorBox.Emit.Status.Fields = append(entry.OperatorBox.Emit.Status.Fields, expanded.Status.Fields...)
		if entry.OperatorBox.Emit.Status.Conditions == nil && expanded.Status.Conditions != nil {
			entry.OperatorBox.Emit.Status.Conditions = expanded.Status.Conditions
		}
	}

	// Merge admission (validation + mutation) rules – these are at CRD level, not operatorBox
	if expanded.HasAdmission() {
		// Merge validation rules
		if expanded.Admission.HasValidationRules() {
			if entry.Admission == nil {
				entry.Admission = &types.AdmissionConfig{}
			}
			if entry.Admission.Validation == nil {
				entry.Admission.Validation = &types.ValidationConfig{}
			}
			entry.Admission.Validation.Rules = append(entry.Admission.Validation.Rules, expanded.Admission.Validation.Rules...)
		}
		// Merge mutation rules
		if expanded.Admission.HasMutationRules() {
			if entry.Admission == nil {
				entry.Admission = &types.AdmissionConfig{}
			}
			if entry.Admission.Mutation == nil {
				entry.Admission.Mutation = &types.MutationConfig{}
			}
			entry.Admission.Mutation.Rules = append(entry.Admission.Mutation.Rules, expanded.Admission.Mutation.Rules...)
		}
	}

	return nil
}
