//go:build !runtime && !gateway

package serve

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/types"
)

var (
	validServeOperations      = strings.Join(types.ValidServeOperations(), ", ")
	validServeEndpointClasses = strings.Join(types.ValidServeEndpointClasses(), ", ")
)

// ── Helpers ─────────────────────────────────────────────────────────────────

// readIntentFile reads an intent file, JSON or YAML (see intent.Decode).
func readIntentFile(path string) (map[string]interface{}, error) {
	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return nil, err
	}
	return intent.Decode(data)
}

func resolveDefaultIntentFile() string {
	for _, name := range []string{"intent.yaml", "intent.json"} {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return ""
}

func namespaceOrAny(ns string) string {
	if ns == "" {
		return "(all namespaces)"
	}
	return ns
}

func aliasOrNone(alias string) string {
	if alias == "" {
		return cmdutil.Gray("(none)")
	}
	return cmdutil.Cyan(alias)
}

// resolveCRD resolves a CRD by target, kind, or name.
func resolveCRD(cat *catalog.Catalog, target, kind, name string) (*types.CRDEntry, error) {
	var crd *types.CRDEntry

	switch {
	case target != "":
		crd = cat.LookupByTarget(target).Entry()
		if crd == nil {
			return nil, fmt.Errorf("%s target %q not found", cmdutil.FailureMark(), target)
		}
	case kind != "":
		crd = cat.LookupByKind(kind).Entry()
		if crd == nil {
			return nil, fmt.Errorf("%s kind %q not found", cmdutil.FailureMark(), kind)
		}
	case name != "":
		crd = cat.LookupByName(name).Entry()
		if crd == nil {
			return nil, fmt.Errorf("%s CRD %q not found", cmdutil.FailureMark(), name)
		}
	default:
		return nil, fmt.Errorf("one of --target, --kind, or --name is required")
	}

	return crd, nil
}

// resolveCRDByAnyTarget resolves a CRD by primary target or alias using
// LookupByTargetOrAlias. Returns the CRD and the alias name (empty when the
// primary target matched). Used by can-i so --target accepts alias names too.
func resolveCRDByAnyTarget(cat *catalog.Catalog, target string) (*types.CRDEntry, string, error) {
	resolution := cat.LookupByTargetOrAlias(target)
	if resolution == nil {
		return nil, "", fmt.Errorf("%s target or alias %q not found", cmdutil.FailureMark(), target)
	}
	return resolution.CRD, resolution.Alias, nil
}

// checkToken applies the gateway's token check to a named token: it must be
// declared in gateway.api.auth.tokens (the gateway authenticates nothing
// else), and the target's serve.tokens, when set, must allow op.
// returns error when token is not declared or denied
func checkToken(c *catalog.Catalog, crd *types.CRDEntry, alias, tokenName string, op types.ServeOperation, ns string) error {
	if declared := c.GatewayTokenNames(); !slices.Contains(declared, tokenName) {
		return fmt.Errorf("unknown token %q\n  declared in gateway.api.auth.tokens: %s", tokenName, strings.Join(declared, ", "))
	}
	if allowed, reason := crd.TokenAllowedFor(alias, tokenName, string(op), ns, types.ServeClassResources); !allowed {
		return fmt.Errorf("token %q denied: %s", tokenName, reason.Message(tokenName, string(op), crd.Kind(), ns))
	}
	return nil
}

// printCanIResult prints the permission check result.
// alias is the serve alias used for the check — empty when the primary target was used.
func printCanIResult(allowed bool, token, op string, crd *types.CRDEntry, namespace, alias, reason string, details []string) {
	if op == "*" {
		op = "perform all operations"
	}

	subject := fmt.Sprintf("%q", crd.ServeTarget())
	if alias != "" {
		subject = fmt.Sprintf("%q (alias: %s)", crd.ServeTarget(), alias)
	}

	fmt.Println()
	if allowed {
		fmt.Printf("%s %s can %s on %s", cmdutil.SuccessMark(), token, op, subject)
		if namespace != "" {
			fmt.Printf(" in namespace %q", namespace)
		}
		fmt.Println()
	} else {
		fmt.Printf("%s %s cannot %s on %s", cmdutil.FailureMark(), token, op, subject)
		if namespace != "" {
			fmt.Printf(" in namespace %q", namespace)
		}
		fmt.Println()
		fmt.Printf("  Reason: %s\n", reason)
		if len(details) > 0 {
			fmt.Printf("  Available:\n")
			for _, detail := range details {
				fmt.Printf("    - %s\n", detail)
			}
		}
	}
	fmt.Println()
}

// ── Field sorting helper ─────────────────────────────────────────────────────

type fieldEntry struct {
	Name     string
	Config   types.ServeFieldConfig
	SpecPath string
	Source   string
}

// sortedFieldEntries returns a sorted list of field entries for a CRD.
// sortBy: "name" (default) or "order"
func sortedFieldEntries(crd *types.CRDEntry, sortBy string) []fieldEntry {
	fields := crd.AllServeFields()
	entries := make([]fieldEntry, 0, len(fields))

	for name, config := range fields {
		specPath := config.SpecPath(name)
		source := "spec"
		if _, ok := crd.ServeLabels()[name]; ok {
			source = "label"
		} else if _, ok := crd.ServeAnnotations()[name]; ok {
			source = "annotation"
		}
		entries = append(entries, fieldEntry{
			Name:     name,
			Config:   config,
			SpecPath: specPath,
			Source:   source,
		})
	}

	switch sortBy {
	case "order":
		sort.Slice(entries, func(i, j int) bool {
			// Fields with order 0 (unset) go after explicitly ordered fields
			oi, oj := entries[i].Config.Order, entries[j].Config.Order
			if oi == 0 && oj == 0 {
				return entries[i].Name < entries[j].Name
			}
			if oi == 0 {
				return false
			}
			if oj == 0 {
				return true
			}
			if oi != oj {
				return oi < oj
			}
			return entries[i].Name < entries[j].Name
		})
	default: // "name"
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name < entries[j].Name
		})
	}

	return entries
}

// ── printAliasesForCRD ──────────────────────────────────────────────

// printAliasesForCRD prints the alias table for one CRD.
func printAliasesForCRD(crd *types.CRDEntry) {
	aliases := crd.ServeAliases()
	if len(aliases) == 0 {
		fmt.Printf("\nCRD: %s (target: %s) — no aliases\n", crd.Name, crd.ServeTargetOrEmpty())
		return
	}

	fmt.Printf("\nCRD: %s (target: %s)\n", crd.Name, crd.ServeTargetOrEmpty())

	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  ALIAS\tTOKENS\tRESPONSE")
	for _, name := range names {
		cfg := aliases[name]
		hasTokens := "no"
		if cfg != nil && cfg.HasTokenRestrictions() {
			hasTokens = "yes"
		}
		hasResponse := "no"
		if cfg != nil && cfg.ResponseConfig() != nil {
			hasResponse = "yes"
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\n", name, hasTokens, hasResponse)
	}
	w.Flush()
}

// ── printServeValidationSummary ──────────────────────────────────────────────

// printServeValidationSummary prints a detailed breakdown of the Serve configuration.
func printServeValidationSummary(cat *catalog.Catalog) {
	enabledCRDs := cat.ServeEnabledCRDs()

	if len(enabledCRDs) == 0 {
		fmt.Println("\nNo Serve-enabled CRDs found.")
		return
	}

	fmt.Println()
	fmt.Println(cmdutil.Bold("Serve Configuration Summary"))
	fmt.Println(strings.Repeat("─", 70))

	for _, crd := range enabledCRDs {
		if crd == nil {
			continue
		}

		fmt.Printf("\n%s %s\n", cmdutil.HealthIconReady(), cmdutil.Bold(crd.Name))

		// ── Target ──────────────────────────────────────────────────────
		target := crd.ServeTarget()
		kind := crd.Kind()
		fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("target: %s  /  kind: %s", target, kind)))

		// ── Name/Namespace ──────────────────────────────────────────────
		if crd.HasServeName() {
			fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("name:      %q", crd.Serve.Name)))
		} else {
			fmt.Printf("  %s\n", cmdutil.Gray("name:      (caller must supply)"))
		}
		if crd.HasServeNamespace() {
			fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("namespace: %q", crd.Serve.Namespace)))
		} else {
			fmt.Printf("  %s\n", cmdutil.Gray("namespace: (cluster-scoped or not set)"))
		}

		// ── Fields ──────────────────────────────────────────────────────
		fields := crd.AllServeFields()
		if len(fields) > 0 {
			var specFields, labelFields, annoFields int
			var nestedPaths int
			for name := range fields {
				if _, ok := crd.ServeLabels()[name]; ok {
					labelFields++
				} else if _, ok := crd.ServeAnnotations()[name]; ok {
					annoFields++
				} else {
					specFields++
				}
				if config, ok := crd.Serve.Fields[name]; ok && config.HasSpecPath() && strings.Contains(config.SpecPath(name), ".") {
					nestedPaths++
				}
			}
			fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("fields:    %d total (spec: %d, labels: %d, annotations: %d)", len(fields), specFields, labelFields, annoFields)))
			if nestedPaths > 0 {
				fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("nested:    %d path(s)", nestedPaths)))
			}
		} else {
			fmt.Printf("  %s\n", cmdutil.Gray("fields:    none"))
		}

		// ── Tokens ──────────────────────────────────────────────────────
		tokenCount := len(crd.Serve.TokensMap())
		tokenTxt := "tokens"
		if tokenCount == 1 {
			tokenTxt = "token"
		}
		if crd.HasServeTokenRestrictions() {
			fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("tokens:    %d %s with restrictions", tokenCount, tokenTxt)))
		} else {
			fmt.Printf("  %s\n", cmdutil.Gray("tokens:    none (all tokens allowed)"))
		}

		// ── Aliases ─────────────────────────────────────────────────────
		if crd.HasServeAliases() {
			aliases := crd.ServeAliases()
			aliasNames := make([]string, 0, len(aliases))
			for name := range aliases {
				aliasNames = append(aliasNames, name)
			}
			sort.Strings(aliasNames)

			aliasTxt := "aliases"
			aliasNamesCount := len(aliasNames)
			if aliasNamesCount == 1 {
				aliasTxt = "alias"
			}

			fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("%s:   %d", aliasTxt, aliasNamesCount)))
			for _, aliasName := range aliasNames {
				alias := aliases[aliasName]
				var parts []string

				aliasTokenTxt := "tokens"
				aliasTokenCount := len(alias.Tokens)
				if aliasTokenCount == 1 {
					aliasTokenTxt = "token"
				}

				if alias != nil && alias.HasTokenRestrictions() {
					parts = append(parts, fmt.Sprintf("%d %s", aliasTokenCount, aliasTokenTxt))
				} else {
					parts = append(parts, "inherits CRD tokens")
				}
				if alias != nil && alias.ResponseConfig() != nil {
					parts = append(parts, "custom response")
				}
				fmt.Printf("    %s %s  %s\n", cmdutil.Gray("·"), cmdutil.Gray(aliasName), cmdutil.Gray("("+strings.Join(parts, ", ")+")"))
			}
		}

		// ── Response Config ─────────────────────────────────────────────
		cfg := crd.GetServeResponseConfig()
		if cfg != nil {
			payloadCount := len(cfg.Payload)
			excludeCount := len(cfg.Exclude)
			defaultVal := "true"
			if !cfg.UseDefault() {
				defaultVal = "false"
			}
			fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("response:  default: %s, payload: %d, exclude: %d", defaultVal, payloadCount, excludeCount)))
			if cfg.HasPoll() {
				pollParts := []string{}
				if cfg.Poll.URL != "" {
					pollParts = append(pollParts, "url")
				}
				if cfg.Poll.Field != "" {
					pollParts = append(pollParts, "field")
				}
				fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("poll:      %s", strings.Join(pollParts, ", "))))
			}
		} else {
			fmt.Printf("  %s\n", cmdutil.Gray("response:  none (default CR response)"))
		}

		// ── Warnings ─────────────────────────────────────────────────────
		if crd.Warnings.HasWarnings() {
			fmt.Printf("  %s\n", cmdutil.Gray("warnings:"))
			for _, w := range crd.Warnings {
				fmt.Printf("    %s %s\n", cmdutil.Yellow("⚠"), cmdutil.Gray(w))
			}
		}
	}

	// ── Summary ──────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println(strings.Repeat("─", 70))
	fmt.Printf("%s Serve configuration is valid\n", cmdutil.SuccessMark())
	fmt.Printf("  %d Serve-enabled CRD(s)\n", len(enabledCRDs))
	fmt.Println()
}

// plural returns one when n is 1, else many.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// relPath returns path relative to the working directory when it lies under
// it, else path unchanged.
func relPath(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(wd, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}
