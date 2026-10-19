// pkg/merger/file.go
package merger

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
)

// ── Merge rules ───────────────────────────────────────────────────────────────
//
// Blueprint  (kind: Blueprint)
//   Declares CRDs directly in spec.crds.
//   Must NOT declare imports — imports are a Stack concern.
//   Error if imports block is present.
//
// Stack (kind: Stack)
//   Composes Blueprints from multiple imports (files, registry, helm).
//   May declare inline spec.crds as overrides — merged last, win on conflict.
//   Imports are resolved recursively. Each import must be a Blueprint.
//   A Stack cannot import another Stack.
//
// Within one file's import tree:
//   localSeen catches duplicates across imports and within inline block.
//
// Across entry point files:
//   seen in Merge() catches duplicates.
//
// Inline overrides import:
//   valid — map key collision triggers mergeCRDEntry.
//
// Inline duplicates inline:
//   always an error.

// loadBlueprintFile parses one file and dispatches to the correct loader
// based on its kind. Returns the deduplicated CRD map for this file tree.
func (m *Merger) loadBlueprintFile(path string) (map[string]types.CRDEntry, error) {
	data, err := loadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}

	doc, err := parseBlueprintDoc(data, path)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		if kind := sniffDocumentKind(data); kind != "" {
			return nil, fmt.Errorf("%q: kind %q cannot be used here — expected kind: Blueprint or Stack", path, kind)
		}
		logger.Debug().
			Str("path", path).
			Msg("merger: skipping — not a valid Blueprint or Stack document")
		return nil, nil
	}

	// Dispatch on Kind — Blueprint and Stack are handled differently
	switch doc.Kind {
	case config.BlueprintKind():
		return m.loadBlueprint(path, doc)
	case config.StackKind():
		return m.loadStack(path, doc)
	default:
		// Should not reach here — parseBlueprintDoc already validates Kind
		return nil, fmt.Errorf("%q: unexpected kind %q", path, doc.Kind)
	}
}

// loadBlueprint reads CRD definitions from a Blueprint file.
// Map keys are the CRD names; Name is injected from the key.
func (m *Merger) loadBlueprint(path string, doc *types.BlueprintFile) (map[string]types.CRDEntry, error) {
	// Guard — imports in a Blueprint is a mistake
	if doc.LooksLikeStack() {
		return nil, fmt.Errorf(
			"%q: kind Blueprint cannot declare imports — "+
				"use kind: Stack to compose multiple Blueprints",
			path,
		)
	}

	result := make(map[string]types.CRDEntry, len(doc.Spec.CRDs))

	// Resolve the blueprint namespace — "default" when not declared.
	blueprintNamespace := doc.Metadata.Namespace
	if blueprintNamespace == "" {
		blueprintNamespace = "default"
	}

	// blueprintDir is used to resolve relative crdFile and crFiles paths.
	// We resolve them here — while we still have the blueprint file's path —
	// so they become absolute before being merged into the top-level map.
	// This allows inrun/validate -f /any/path/blueprint.yaml to work from
	// any working directory, even when the blueprint is imported by a Stack.
	//
	// blueprintDir must be made genuinely absolute (not just joined once) —
	// downstream resolution (e.g. PopulateAPITypesFromCRDFile in
	// pkg/blueprint/crdfile.go) re-checks filepath.IsAbs() on the already-
	// joined path and re-joins it against blueprintDir again if it's still
	// relative, doubling the directory. A relative -f path with a real
	// subdirectory (e.g. -f a/b/blueprint.yaml run from elsewhere) used to
	// silently double into a/b/a/b/crd.yaml — invisible when blueprintDir
	// happened to be "." (running from inside the blueprint's own directory).
	blueprintDir := filepath.Dir(path)
	if abs, err := filepath.Abs(blueprintDir); err == nil {
		blueprintDir = abs
	}

	for name, crd := range doc.Spec.CRDs {
		if name == "" {
			return nil, fmt.Errorf("%q spec.crds: CRD with empty key", path)
		}
		// Duplicate within the same file is impossible — map keys are unique.
		crd.Name = name

		// Resolve crdFile and crFiles to absolute paths relative to this blueprint.
		if crd.CRDFile != "" && !filepath.IsAbs(crd.CRDFile) && !strings.HasPrefix(crd.CRDFile, "http") {
			crd.CRDFile = filepath.Join(blueprintDir, crd.CRDFile)
		}
		for i, cf := range crd.CRFiles {
			if !filepath.IsAbs(cf) && !strings.HasPrefix(cf, "http") {
				crd.CRFiles[i] = filepath.Join(blueprintDir, cf)
			}
		}
		if crd.Setup != nil {
			for i, entry := range crd.Setup.Apply {
				if !filepath.IsAbs(entry.Path) && !strings.HasPrefix(entry.Path, "http") {
					crd.Setup.Apply[i].Path = filepath.Join(blueprintDir, entry.Path)
				}
			}
		}
		// Resolve module file paths in imports to absolute so they work regardless
		// of the working directory when expandModuleImports runs.
		if crd.Box().Reconcile != nil {
			for i, imp := range crd.Box().Reconcile.Imports {
				if isFileModule(imp.Module) && !filepath.IsAbs(imp.Module) {
					crd.Box().Reconcile.Imports[i].Module = filepath.Join(blueprintDir, imp.Module)
				}
			}
		}

		// Stamp blueprint metadata — only when not already set so that values
		// deserialized from an expanded ConfigMap YAML are preserved.
		if crd.BlueprintNamespace == "" {
			crd.BlueprintNamespace = blueprintNamespace
		}
		if crd.BlueprintDescription == "" {
			crd.BlueprintDescription = doc.Metadata.Description
		}
		if crd.BlueprintVersion == "" {
			crd.BlueprintVersion = doc.Metadata.Version
		}

		// Apply blueprint-level CrossAccess as the default for every CRD that
		// does not declare its own crossAccess field.
		if crd.CrossAccess == nil && doc.CrossAccess != nil {
			v := *doc.CrossAccess
			crd.CrossAccess = &v
		}

		// Merge spec-level restrictions into each CRD (additive).
		protect := doc.Security.NamespaceProtection
		if protect != nil {
			if len(protect.RestrictedNamespaces) > 0 || len(protect.AllowedNamespaces) > 0 {
				if crd.Box().Runtime == nil {
					rt := types.RuntimeConfig{}
					crd.Box().Runtime = &rt
				}
				if len(protect.RestrictedNamespaces) > 0 {
					crd.Box().Runtime.RestrictedNamespaces = protect.RestrictedNamespaces.Merge(crd.Box().Runtime.RestrictedNamespaces)
				}
				if len(protect.AllowedNamespaces) > 0 {
					crd.Box().Runtime.AllowedNamespaces = protect.AllowedNamespaces.Merge(crd.Box().Runtime.AllowedNamespaces)
				}
			}
		}

		result[name] = crd

		logger.Debug().
			Str("crd", name).
			Str("source", path).
			Msg("merger: CRD loaded from Blueprint")
	}

	logger.Debug().
		Str("path", path).
		Int("crds", len(result)).
		Msg("merger: Blueprint loaded")

	// This is a blueprint
	apiMetadata := apiMetadata{
		APIVersion: doc.APIVersion,
		Kind:       doc.Kind,
		Metadata:   doc.Metadata,
	}
	m.apiMetadata = apiMetadata
	m.lifecycle = doc.Lifecycle
	m.policy = doc.Policy
	m.security = doc.Security
	m.gateway = doc.Gateway
	m.publish = doc.Publish
	m.presets = doc.Presets
	// Expand functions.include against this Blueprint's own directory, so a Blueprint
	// imported by a Stack brings its included functions with it.
	functions := doc.Functions
	if err := types.ExpandFunctionsInclude(&functions, blueprintDir); err != nil {
		return nil, fmt.Errorf("%q: functions: %w", path, err)
	}
	m.functions = functions
	// Resolve spec.imports module file paths to absolute, same as CRD-level imports above.
	specImports := make([]types.ModuleImport, len(doc.Spec.Imports))
	copy(specImports, doc.Spec.Imports)
	for i, imp := range specImports {
		if isFileModule(imp.Module) && !filepath.IsAbs(imp.Module) {
			specImports[i].Module = filepath.Join(blueprintDir, imp.Module)
		}
	}
	m.specImports = specImports

	return result, nil
}

// loadStack resolves imports from a Stack file and merges all CRDs.
func (m *Merger) loadStack(path string, doc *types.BlueprintFile) (map[string]types.CRDEntry, error) {
	if !doc.WithImportsOrSpecOverrides() {
		logger.Warn().
			Str("path", path).
			Msg("merger: Stack has no imports and no inline CRDs — nothing to load")
		return nil, nil
	}

	localSeen := map[string]string{}
	allCRDs := make(map[string]types.CRDEntry)

	// settings from all imported Blueprints. Each import that calls loadBlueprint sets these
	// as side-effects on m; we capture and merge here so they are not discarded
	// when the Stack's own (possibly Empty() block is applied at the end.
	var accSecurity types.BlueprintSecurity
	var accPresets types.PresetRegistry
	var accSpecImports []types.ModuleImport
	var accFunctions types.KELFunctionRegistry
	functionsSeen := make(map[string]string) // function name → import label, for cross-Blueprint conflict detection

	// ── Step 1: registry imports ─────────────────────────────────────────────
	if doc.Imports != nil {
		for i, regSrc := range doc.Imports.Registry {
			crds, err := m.loadRegistrySource(regSrc)
			if err != nil {
				return nil, fmt.Errorf("%q imports.registry[%d]: %w", path, i, err)
			}

			for name, crd := range crds {
				srcName := fmt.Sprintf("registry:%d", i)
				if regSrc.URL != "" {
					srcName = "registry:" + regSrc.URL
				}
				if err := checkDuplicate(localSeen, name, srcName); err != nil {
					return nil, fmt.Errorf("%q: %w", path, err)
				}
				localSeen[name] = srcName
				allCRDs[name] = crd
			}

			// Accumulate security and presets from registry source Blueprint.
			accSecurity = mergeBlueprintSecurity(accSecurity, m.security)
			merged, err := accPresets.Merge(m.presets, fmt.Sprintf("registry:%d", i))
			if err != nil {
				return nil, fmt.Errorf("%q imports.registry[%d]: presets: %w", path, i, err)
			}
			accPresets = merged
			accSpecImports = append(accSpecImports, m.specImports...)
			mergedFunctions, err := accFunctions.MergeImport(m.functions, fmt.Sprintf("registry:%d", i), functionsSeen)
			if err != nil {
				return nil, fmt.Errorf("%q imports.registry[%d]: functions: %w", path, i, err)
			}
			accFunctions = mergedFunctions
			logger.Debug().
				Str("import", fmt.Sprintf("registry:%d", i)).
				Msg("merger: accumulated security from registry import")
		}
	}

	// ── Step 2: file imports ──────────────────────────────────────────────────
	if doc.Imports != nil {
		for _, fileSrc := range doc.Imports.Files {

			// Resolve environment variable in the URL if needed
			resolved, err := resolveEnvVar(fileSrc.URL)
			if err != nil {
				return nil, fmt.Errorf("%q imports.files: %w", path, err)
			}

			// Resolve relative paths against the Stack's directory so
			// inrun -f /any/path/stack.yaml works from any working directory.
			if !filepath.IsAbs(resolved) && !strings.HasPrefix(resolved, "http") {
				resolved = filepath.Join(filepath.Dir(path), resolved)
			}

			// Resolve authentication credentials from environment variables
			auth, err := fileSrc.Auth.Resolve()
			if err != nil {
				return nil, fmt.Errorf("%q imports.files[%q]: auth: %w", path, resolved, err)
			}

			// Load the file — must be a Blueprint, not another Stack
			crds, err := m.loadImportFileWithAuth(path, resolved, auth)
			if err != nil {
				return nil, fmt.Errorf("%q imports.files[%q]: %w", path, resolved, err)
			}

			for name, crd := range crds {
				if err := checkDuplicate(localSeen, name, "file:"+resolved); err != nil {
					return nil, fmt.Errorf("%q: %w", path, err)
				}
				localSeen[name] = "file:" + resolved
				allCRDs[name] = crd
			}

			// Accumulate security and presets from this Blueprint file import.
			accSecurity = mergeBlueprintSecurity(accSecurity, m.security)
			merged, err := accPresets.Merge(m.presets, "file:"+resolved)
			if err != nil {
				return nil, fmt.Errorf("%q imports.files[%q]: presets: %w", path, resolved, err)
			}
			accPresets = merged
			accSpecImports = append(accSpecImports, m.specImports...)
			mergedFunctions, err := accFunctions.MergeImport(m.functions, "file:"+resolved, functionsSeen)
			if err != nil {
				return nil, fmt.Errorf("%q imports.files[%q]: functions: %w", path, resolved, err)
			}
			accFunctions = mergedFunctions
			logger.Debug().
				Str("import", "file:"+resolved).
				Msg("merger: accumulated security from file import")
		}
		// ── Step 3: helm imports ──────────────────────────────────────────────
		for i, helmSrc := range doc.Imports.Helm {
			crds, err := m.loadHelmSource(helmSrc)
			if err != nil {
				return nil, fmt.Errorf("%q imports.helm[%d]: %w", path, i, err)
			}

			srcName := fmt.Sprintf("helm:%s/%s@%s", helmSrc.Repo, helmSrc.Chart, helmSrc.Version)
			for name, crd := range crds {
				if err := checkDuplicate(localSeen, name, srcName); err != nil {
					return nil, fmt.Errorf("%q: %w", path, err)
				}
				localSeen[name] = srcName
				allCRDs[name] = crd
			}

			// Accumulate security and presets from this Helm import.
			accSecurity = mergeBlueprintSecurity(accSecurity, m.security)
			merged, err := accPresets.Merge(m.presets, srcName)
			if err != nil {
				return nil, fmt.Errorf("%q imports.helm[%d]: presets: %w", path, i, err)
			}
			accPresets = merged
			accSpecImports = append(accSpecImports, m.specImports...)
			mergedFunctions, err := accFunctions.MergeImport(m.functions, srcName, functionsSeen)
			if err != nil {
				return nil, fmt.Errorf("%q imports.helm[%d]: functions: %w", path, i, err)
			}
			accFunctions = mergedFunctions
			logger.Debug().
				Str("import", srcName).
				Msg("merger: accumulated security from helm import")
		}
	}

	// ── Step 4: inline spec.crds — override any source CRD with same name ────
	inlineKey := "inline:" + path
	for name, crd := range doc.Spec.CRDs {
		if name == "" {
			return nil, fmt.Errorf("%q spec.crds: CRD with empty key", path)
		}

		// Duplicate within the same inline block is impossible (map keys).
		// But if same inline key was already recorded, it's a bug.
		if existing, ok := localSeen[name]; ok && existing == inlineKey {
			return nil, fmt.Errorf(
				"%q spec.crds: duplicate CRD %q — each CRD name must be unique",
				path, name,
			)
		}

		crd.Name = name

		// Merge onto source, don't replace
		if base, found := allCRDs[name]; found {
			allCRDs[name] = mergeCRDEntry(base, crd)
			logger.Debug().
				Str("crd", name).
				Str("source", inlineKey).
				Msg("merger: inline override merged onto source entry")
		} else {
			allCRDs[name] = crd
			logger.Debug().
				Str("crd", name).
				Str("source", inlineKey).
				Msg("merger: new CRD from inline spec.crds")
		}

		localSeen[name] = inlineKey
	}
	// Fill BlueprintDescription and BlueprintVersion fallbacks — if the sub-Blueprint had none, use the Stack's.
	for name, crd := range allCRDs {
		changed := false
		if crd.BlueprintDescription == "" && doc.Metadata.Description != "" {
			crd.BlueprintDescription = doc.Metadata.Description
			changed = true
		}
		if crd.BlueprintVersion == "" && doc.Metadata.Version != "" {
			crd.BlueprintVersion = doc.Metadata.Version
			changed = true
		}
		if changed {
			allCRDs[name] = crd
		}
	}

	// Merge Stack-level restrictions into every CRD (additive).
	protect := doc.Security.NamespaceProtection
	if protect != nil {
		if len(protect.RestrictedNamespaces) > 0 || len(protect.AllowedNamespaces) > 0 {
			for name, crd := range allCRDs {
				if crd.Box().Runtime == nil {
					rt := types.RuntimeConfig{}
					crd.Box().Runtime = &rt
				}
				crd.Box().Runtime.RestrictedNamespaces = protect.RestrictedNamespaces.Merge(crd.Box().Runtime.RestrictedNamespaces)
				crd.Box().Runtime.AllowedNamespaces = protect.AllowedNamespaces.Merge(crd.Box().Runtime.AllowedNamespaces)
				allCRDs[name] = crd
			}
		}
	}

	logger.Debug().
		Str("path", path).
		Int("crds", len(allCRDs)).
		Msg("merger: Stack loaded")

	// This is a stack
	apiMetadata := apiMetadata{
		APIVersion: doc.APIVersion,
		Kind:       doc.Kind,
		Metadata:   doc.Metadata,
	}

	m.apiMetadata = apiMetadata

	// Merge accumulated source fields with the Stack's own top-level blocks.
	// Stack-declared fields win on conflict (non-nil / non-empty override semantics).
	// This ensures all top-level Blueprint fields, such as security,
	// are visible when running `inrun generate rbac` or `inrun generate configmap`
	// against a Stack, identical to running against the source Blueprints directly.
	m.security = mergeBlueprintSecurity(accSecurity, doc.Security)
	if doc.Gateway != nil {
		m.gateway = doc.Gateway
	}
	if doc.Publish != nil {
		m.publish = doc.Publish
	}
	if doc.Lifecycle != nil {
		m.lifecycle = doc.Lifecycle
	}
	if doc.Policy != nil {
		m.policy = doc.Policy
	}

	mergedPresets, err := accPresets.Merge(doc.Presets, path)
	if err != nil {
		return nil, fmt.Errorf("%q: presets: %w", path, err)
	}
	m.presets = mergedPresets

	if len(doc.Spec.Imports) > 0 {
		return nil, fmt.Errorf("%q: Stack does not support spec.imports — declare functions: and presets: inline to override Blueprint-wide settings", path)
	}
	m.specImports = accSpecImports
	mergedFunctions, err := doc.Functions.Merge(accFunctions, "blueprint")
	if err != nil {
		return nil, fmt.Errorf("%q: functions: %w", path, err)
	}
	m.functions = mergedFunctions

	logger.Debug().
		Str("path", path).
		Msg("merger: Stack security merged from imports and inline")

	return allCRDs, nil
}
