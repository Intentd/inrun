// pkg/generate/registry_generator.go
package generate

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/types"
)

// TypeRegistry generates zz_generated_typeregistry.go from the merged Blueprint.
//
// When generation is needed (and why):
//
//	Typed CRDs (apiTypes.location set)
//	  The compiled Go type must be registered so the informer and REST client
//	  can decode API server responses. Without this, List/Watch calls fail with
//	  "not suitable for converting" errors.
//
//	Go hooks (reconciler.hooks declared)
//	  The hook function lives in an external package. The generator writes the
//	  import and wires it into HookRegistry so addHooks() can find it.
//
//	Custom constructors (reconciler.default: false)
//	  Same as Go hooks — external function needs import and ReconcilerRegistry entry.
//
// When generation is NOT needed:
//
//	Dynamic template CRDs (only onCreate/onReconcile/onDelete declared)
//	  generic.Reconciler.runTemplateReconcile() reads the Blueprint's operatorBox:Config
//	  directly at runtime and calls the InrunRegistry functions itself.
//	  No generated file. No inrun generate registry. Just inrun.
func TypeRegistry(crds map[string]types.CRDEntry, dryRun bool) (bool, error) {

	var (
		imports           []importEntry
		entries           []registryEntry   // typed CRDs → ObjectRegistry + ListRegistry + RegisterTypedScheme
		hookEntries       []hookEntry       // Go hooks → HookRegistry
		recEntries        []reconcilerEntry // custom constructors → ReconcilerRegistry
		targetHookEntries []targetHookEntry // per-target hooks → TargetHookRegistry
		targetRecEntries  []targetRecEntry  // per-target constructors → TargetReconcilerRegistry

		seenObjectAliases = map[string]string{}
		seenHookAliases   = map[string]string{}
		seenRecAliases    = map[string]string{}
	)

	for _, crd := range crds {
		if !crd.IsEnabled() {
			continue
		}

		// ── Typed CRDs ────────────────────────────────────────────────────────
		// apiTypes.location set means compiled Go types exist.
		// We need ObjectRegistry + ListRegistry entries so the informer can
		// create zero-value instances, and RegisterTypedScheme so the REST
		// client can decode API server responses.
		//
		// Dynamic CRDs (no location) skip this entirely — they use the dynamic
		// client which bypasses scheme decoding.
		if !crd.IsDynamic() && crd.APITypes.Location != "" {
			if err := validateAPITypes(crd); err != nil {
				return false, fmt.Errorf("CRD %q: %w", crd.Name, err)
			}

			alias := resolveAlias(crd.APITypes.Alias, crd.Name, crd.APITypes.Location)

			if err := dedupeImport(seenObjectAliases, alias, crd.APITypes.Location, crd.Name); err != nil {
				return false, err
			}
			if _, seen := seenObjectAliases[alias]; !seen {
				imports = append(imports, importEntry{Alias: alias, Location: crd.APITypes.Location})
				seenObjectAliases[alias] = crd.APITypes.Location
			}

			entries = append(entries, registryEntry{
				Alias:   alias,
				Object:  crd.APITypes.Object,
				List:    crd.APITypes.List,
				Group:   crd.APITypes.Group,
				Version: crd.APITypes.Version,
				Kind:    crd.APITypes.Kind,
			})
		}

		// ── Go hooks ──────────────────────────────────────────────────────────
		// reconciler.hooks.location + function declared means the user has
		// written a Go hook function in an external package.
		// We need to import that package and register a closure in HookRegistry
		// so addHooks() can wire it at startup.
		//
		// This is NOT needed for declarative template CRDs — those are handled
		// at runtime by generic.Reconciler.runTemplateReconcile() with no
		// registration required.
		if crd.DefaultReconcile() && crd.Box().Reconcile != nil && crd.Box().Reconcile.Hooks != nil {
			h := crd.Box().Reconcile.Hooks

			if err := validateHookEntry(h, crd.Name); err != nil {
				return false, err
			}

			hookAlias := resolveAlias(h.Alias, crd.Name+"hooks", h.Location)

			if err := dedupeImport(seenHookAliases, hookAlias, h.Location, crd.Name); err != nil {
				return false, err
			}
			if _, seen := seenHookAliases[hookAlias]; !seen {
				imports = append(imports, importEntry{Alias: hookAlias, Location: h.Location})
				seenHookAliases[hookAlias] = h.Location
			}

			hookEntries = append(hookEntries, hookEntry{
				Alias:    hookAlias,
				Function: h.Function,
				Group:    crd.APITypes.Group,
				Version:  crd.APITypes.Version,
				Kind:     crd.APITypes.Kind,
			})
		}

		// ── Custom constructors ───────────────────────────────────────────────
		// reconciler.default: false means the user owns the entire reconcile loop.
		// We need to import their constructor and register it in ReconcilerRegistry
		// so the runtime can look it up by GVK.
		if !crd.DefaultReconcile() {
			if crd.Box().Reconcile == nil || crd.Box().Reconcile.ConstructorDecl == nil {
				return false, fmt.Errorf(
					"CRD %q: reconciler.default is false but no constructor declared — "+
						"add reconciler.constructor with location and function, "+
						"then re-run inrun generate registry",
					crd.Name,
				)
			}

			c := crd.Box().Reconcile.ConstructorDecl

			if err := validateConstructorEntry(c, crd.Name); err != nil {
				return false, err
			}

			recAlias := resolveAlias(c.Alias, crd.Name+"rec", c.Location)

			if err := dedupeImport(seenRecAliases, recAlias, c.Location, crd.Name); err != nil {
				return false, err
			}
			if _, seen := seenRecAliases[recAlias]; !seen {
				imports = append(imports, importEntry{Alias: recAlias, Location: c.Location})
				seenRecAliases[recAlias] = c.Location
			}

			recEntries = append(recEntries, reconcilerEntry{
				Alias:    recAlias,
				Function: c.Function,
				Group:    crd.APITypes.Group,
				Version:  crd.APITypes.Version,
				Kind:     crd.APITypes.Kind,
			})
		}

		// ── Per-target hooks ──────────────────────────────────────────────────
		// A target that declares reconciler.hooks with a different location than
		// the CRD-level hooks binary needs its own import and a TargetHookRegistry
		// entry. Targets that only override args share the CRD-level binary and
		// are handled at runtime by mergeReconcilerConfig in EffectiveOperatorBox.
		if crd.Serve != nil && crd.Serve.Target.Entries != nil {
			crdLevelHookLocation := ""
			if crd.Box().Reconcile != nil && crd.Box().Reconcile.Hooks != nil {
				crdLevelHookLocation = crd.Box().Reconcile.Hooks.Location
			}
			for targetName, targetCfg := range crd.Serve.Target.Entries {
				if targetCfg.OperatorBox == nil || targetCfg.Box().Reconcile == nil {
					continue
				}
				h := targetCfg.Box().Reconcile.Hooks
				if h == nil || h.Location == "" || h.Location == crdLevelHookLocation {
					continue
				}
				if err := validateHookEntry(h, crd.Name+" target "+targetName); err != nil {
					return false, err
				}
				hookAlias := resolveAlias(h.Alias, crd.Name+"hooks", h.Location)
				if err := dedupeImport(seenHookAliases, hookAlias, h.Location, crd.Name); err != nil {
					return false, err
				}
				if _, seen := seenHookAliases[hookAlias]; !seen {
					imports = append(imports, importEntry{Alias: hookAlias, Location: h.Location})
					seenHookAliases[hookAlias] = h.Location
				}
				targetHookEntries = append(targetHookEntries, targetHookEntry{
					Alias:      hookAlias,
					Function:   h.Function,
					Group:      crd.APITypes.Group,
					Version:    crd.APITypes.Version,
					Kind:       crd.APITypes.Kind,
					TargetName: targetName,
				})
			}
		}

		// ── Per-target constructors ───────────────────────────────────────────
		// A target that declares reconciler.default: false with its own constructor
		// gets a TargetReconcilerRegistry entry so startCRDWorkers can build a
		// mux reconciler with the right sub-reconciler per target.
		if crd.Serve != nil && crd.Serve.Target.Entries != nil {
			for targetName, targetCfg := range crd.Serve.Target.Entries {
				if targetCfg.OperatorBox == nil || targetCfg.Box().Reconcile == nil {
					continue
				}
				rec := targetCfg.Box().Reconcile
				if rec.Default == nil || *rec.Default || rec.ConstructorDecl == nil {
					continue
				}
				c := rec.ConstructorDecl
				if err := validateConstructorEntry(c, crd.Name+" target "+targetName); err != nil {
					return false, err
				}
				recAlias := resolveAlias(c.Alias, crd.Name+"rec", c.Location)
				if err := dedupeImport(seenRecAliases, recAlias, c.Location, crd.Name); err != nil {
					return false, err
				}
				if _, seen := seenRecAliases[recAlias]; !seen {
					imports = append(imports, importEntry{Alias: recAlias, Location: c.Location})
					seenRecAliases[recAlias] = c.Location
				}
				targetRecEntries = append(targetRecEntries, targetRecEntry{
					Alias:      recAlias,
					Function:   c.Function,
					Group:      crd.APITypes.Group,
					Version:    crd.APITypes.Version,
					Kind:       crd.APITypes.Kind,
					TargetName: targetName,
				})
			}
		}
	}

	// ── Nothing to generate ───────────────────────────────────────────────────
	// Pure dynamic template Blueprints produce zero entries — this is correct.
	// generic.Reconciler handles them at runtime. Exit cleanly, no file written.
	if len(entries) == 0 && len(recEntries) == 0 && len(hookEntries) == 0 &&
		len(targetHookEntries) == 0 && len(targetRecEntries) == 0 {
		return false, nil
	}

	// ── Render registry file ──────────────────────────────────────────────────
	registryData := registryTemplateData{
		Timestamp:          time.Now().UTC().Format(time.RFC3339),
		Imports:            imports,
		Entries:            entries,
		HookEntries:        hookEntries,
		RecEntries:         recEntries,
		TargetHookEntries:  targetHookEntries,
		TargetRecEntries:   targetRecEntries,
		NeedsRecImports:    len(recEntries) > 0 || len(targetRecEntries) > 0,
		NeedsHookImports:   len(hookEntries) > 0 || len(targetHookEntries) > 0,
		NeedsSchemeImports: len(entries) > 0,
	}

	outPath := filepath.Join(TypeRegistryPackage, RegistryFile)
	return true, renderTemplateToFile(registryTemplate, registryData, outPath, true, dryRun)
}

// ── Validation helpers ────────────────────────────────────────────────────────

// validateAPITypes checks that all required fields are present for typed CRD registration.
// These fields are needed to generate correct ObjectRegistry entries and scheme imports.
func validateAPITypes(crd types.CRDEntry) error {
	t := crd.APITypes
	var missing []string
	if t.Object == "" {
		missing = append(missing, "apiTypes.object")
	}
	if t.List == "" {
		missing = append(missing, "apiTypes.list")
	}
	if t.Group == "" {
		missing = append(missing, "apiTypes.group")
	}
	if t.Version == "" {
		missing = append(missing, "apiTypes.version")
	}
	if t.Kind == "" {
		missing = append(missing, "apiTypes.kind")
	}
	if t.Location == "" {
		missing = append(missing, "apiTypes.location")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required apiTypes fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// validateHookEntry checks that location and function are both declared.
// Both are required to generate a valid import and HookRegistry closure.
func validateHookEntry(h *types.HookDeclaration, crdName string) error {
	var missing []string
	if h.Location == "" {
		missing = append(missing, "reconciler.hooks.location")
	}
	if h.Function == "" {
		missing = append(missing, "reconciler.hooks.function")
	}
	if len(missing) > 0 {
		return fmt.Errorf("CRD %q: missing required hook fields: %s", crdName, strings.Join(missing, ", "))
	}
	return nil
}

// validateConstructorEntry checks that location and function are both declared.
// Both are required to generate a valid import and ReconcilerRegistry closure.
func validateConstructorEntry(c *types.ConstructorDeclaration, crdName string) error {
	var missing []string
	if c.Location == "" {
		missing = append(missing, "reconciler.constructor.location")
	}
	if c.Function == "" {
		missing = append(missing, "reconciler.constructor.function")
	}
	if len(missing) > 0 {
		return fmt.Errorf("CRD %q: missing required constructor fields: %s", crdName, strings.Join(missing, ", "))
	}
	return nil
}

// dedupeImport returns an error if the same alias is used for two different
// import paths. This catches cases where two CRDs in the same Blueprint have
// conflicting aliases — the user must set an explicit alias to resolve.
func dedupeImport(seen map[string]string, alias, location, crdName string) error {
	if existing, ok := seen[alias]; ok && existing != location {
		return fmt.Errorf(
			"CRD %q: import alias %q is already used for %q — "+
				"set a unique alias via apiTypes.alias or reconciler.hooks.alias",
			crdName, alias, existing,
		)
	}
	return nil
}

// resolveAlias returns the import alias to use for a package.
// Priority: explicit alias from Blueprint → derived from the last two path segments.
//
// Examples:
//
//	location: github.com/myorg/apis/project/v1alpha1 → prefix + "project" + "v1alpha1"
//	location: github.com/myorg/hooks                 → prefix + "hooks"
func resolveAlias(explicitAlias, prefix, location string) string {
	if explicitAlias != "" {
		return explicitAlias
	}

	// Extract path parts and build base name
	parts := strings.Split(strings.TrimRight(location, "/"), "/")
	var base string
	if len(parts) >= 2 {
		base = parts[len(parts)-2] + parts[len(parts)-1]
	} else if len(parts) == 1 {
		base = parts[0]
	} else {
		base = "v1"
	}

	// Normalize: remove dots, dashes and any non-alphanumeric characters
	// then collapse repeated non-alnum (defensive)
	reNonAlnum := regexp.MustCompile(`[^A-Za-z0-9]+`)
	sanitized := reNonAlnum.ReplaceAllString(base, "")

	// Optional: lowercase for consistency
	sanitized = strings.ToLower(sanitized)

	// Truncate if too long, appending a short hash suffix to reduce collisions
	const maxLen = 30
	if len(sanitized) > maxLen {
		h := sha1.Sum([]byte(sanitized))
		suffix := fmt.Sprintf("%x", h)[:6] // 6 hex chars
		keep := maxLen - len(suffix)
		if keep <= 0 {
			// fallback: use only suffix if maxLen is very small
			sanitized = suffix
		} else {
			sanitized = sanitized[:keep] + suffix
		}
	}

	return prefix + sanitized
}
