//go:build !runtime && !gateway

package validate

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/migrate"

	"path/filepath"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/registry/e2e"
	modulepkg "github.com/intentd/inrun/pkg/registry/module"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/jig"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	rbacv1 "k8s.io/api/rbac/v1"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate an Inrun pattern (Blueprint, Stack, Module, E2E, Simulate)",
	Long: `Validates any Inrun pattern and reports errors.

The pattern kind is detected automatically from the 'kind' field:
  Blueprint   — operator definition with CRD declarations
  Stack  — multi-source blueprint composer
  Module     — reusable operator pattern
  E2E       — declarative end-to-end test spec
  Simulate  — declarative reconciler assertions

Reads blueprint.yaml or stack.yaml from the current directory by default.
Pass -f to validate a different file.

Examples:
  inrun validate
  inrun validate -f e2e.yaml
  inrun validate -f simulate.yaml
  inrun validate -f module.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, _ := cmd.Flags().GetStringSlice("file")
		expanded := cmdutil.ParseFilePaths(paths)
		if len(expanded) == 0 {
			expanded = cmdutil.DefaultFilePaths()
		}
		if len(expanded) == 0 {
			return fmt.Errorf(cmdutil.ErrNoBlueprint)
		}

		var docKind string
		for _, path := range expanded {
			kind, err := detectKindFromFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}

			// Validate document type
			if !config.IsValidPatternKind(kind) {
				if kind == "" {
					return fmt.Errorf(
						"not an Inrun pattern — expected a 'kind' field (allowed kinds: %s)",
						config.ValidKindsString(),
					)
				}
				return fmt.Errorf(
					"invalid Inrun pattern kind %q (allowed kinds: %s)",
					kind, config.ValidKindsString(),
				)
			}

			if config.IsModuleKind(kind) {
				return validateModuleFile(path)
			}

			if config.IsE2EKind(kind) {
				return ValidateE2EFile(path)
			}

			if config.IsSimulateKind(kind) {
				return ValidateSimulateFile(path)
			}

			docKind = kind
		}

		// Default path: Blueprint / Stack validation
		spin := cmdutil.StartSpinner("Resolving imports...")
		m, err := cmdutil.GenerateBlueprint(cmd)
		if err != nil {
			spin.Failure()
			return err
		}
		spin.Stop()

		bp, err := pipeline.BuildExpanded(cmdutil.Cfg, m.Merger)
		if err != nil {
			var typedErr *blueprint.TypedOperatorError
			if errors.As(err, &typedErr) {
				cmdutil.PrintTypedOperatorHint(typedErr, "inrun validate")
			}
			return err
		}
		entries := bp.EnabledCRDs()

		// The migrations, checked offline by KOME: identities and level,
		// schemas both ways, the transitional schema, the CR files.
		migrationWarnings, err := validateMigrations(bp)
		if err != nil {
			return err
		}

		kindLabel := "Blueprint"
		if config.IsStackKind(docKind) {
			kindLabel = "Stack"
		}

		if bp.IsStandaloneGateway() {
			kindLabel = "Gateway Standalone"
		}

		full, _ := cmd.Flags().GetBool("full")
		showFunctions, _ := cmd.Flags().GetBool("functions")
		showPresets, _ := cmd.Flags().GetBool("presets")

		// --functions / --presets: quiet mode — skip full validate output, print only that registry.
		if showFunctions || showPresets {
			if showFunctions {
				printValidateFunctions(bp.Functions)
			}
			if showPresets {
				printValidatePresets(bp.Presets)
			}
			return nil
		}

		var perCRDPerms map[string][]rbacv1.PolicyRule
		if full {
			perCRDPerms = bp.EnabledCRDsRBACRules()
		}

		// Sort entries by name for stable output across runs.
		sortedEntries := make(types.CRDEntries, 0, len(entries))
		for _, e := range entries {
			sortedEntries = append(sortedEntries, e)
		}
		sort.Slice(sortedEntries, func(i, j int) bool {
			return sortedEntries[i].Name < sortedEntries[j].Name
		})

		fmt.Println()
		fmt.Println(cmdutil.Bold("Validating " + kindLabel + "..."))
		fmt.Println()

		builtIn := 0
		custom := 0

		var deprecationHint string
		if config.IsStackKind(docKind) {
			deprecationHint = "To acknowledge this import, add it to lifecycle.accept.patterns in your Stack."
		}
		cmdutil.PrintBlueprintDeprecationWithHint(bp.Deprecation(), deprecationHint)

		// Print each CRD entry with enrichment info
		for _, entry := range sortedEntries {
			printCRDValidationLine(bp, entry)
			if full {
				printCRDPermissions(perCRDPerms[entry.Name])
				printCRDPresets(entry)
			}
			fmt.Println()

			if entry.IsBuiltIn {
				builtIn++
			} else {
				custom++
			}
		}

		// Summary
		crdText := "CRDs"
		if (builtIn + custom) == 1 {
			crdText = "CRD"
		}

		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("%d %s valid (%d built-in, %d custom)\n", len(entries), crdText, builtIn, custom)

		for _, w := range bp.CronConditionWarnings() {
			fmt.Printf("\n%s  %s\n", cmdutil.Yellow("⚠"), w)
		}
		for _, w := range migrationWarnings {
			fmt.Printf("\n%s  migrate %s\n", cmdutil.Yellow("⚠"), w)
		}

		if full {
			if dd := bp.DependencyDisplayData(); dd != nil {
				printValidateDependencyGraph(dd)
			}
			printRuntimePermissionsSection(bp.RuntimeRBACRules())
			printGatewayPermissionsSection(bp.GatewayRBACRules())
			printRuntimeContext()
			fmt.Println()
		}

		// Always print non-empty registries inline.
		printValidateFunctions(bp.Functions)
		printValidatePresets(bp.Presets)

		return nil
	},
}

// detectKindFromFile peeks at a YAML file to read its kind field.
// validateMigrations runs KOME's offline checks on the Blueprint's
// migrations. Errors are printed and fail; warnings are returned.
func validateMigrations(bp *blueprint.Blueprint) ([]string, error) {
	f, err := migrate.Declaration(bp, "")
	if err != nil {
		return nil, err
	}
	if len(f.Migrations) == 0 {
		return nil, nil
	}
	rep := f.Validate()
	var warnings []string
	for _, w := range rep.Warnings() {
		warnings = append(warnings, w.String())
	}
	if errs := rep.Errors(); len(errs) > 0 {
		fmt.Println()
		for _, e := range errs {
			fmt.Printf("%s migrate %s\n", cmdutil.FailureMark(), e)
		}
		return nil, fmt.Errorf("%d migration error(s)", len(errs))
	}
	return warnings, nil
}

func detectKindFromFile(path string) (string, error) {
	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return "", err
	}
	var doc struct {
		Kind string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", err
	}
	return doc.Kind, nil
}

// ValidateE2EFile validates an E2E spec file and prints a summary.
func ValidateE2EFile(path string) error {
	return validateE2EFileOpts(path, false)
}

// ValidateE2EFileQuiet validates without printing; the returned error
// lists every problem.
func ValidateE2EFileQuiet(path string) error {
	return validateE2EFileOpts(path, true)
}

func validateE2EFileOpts(path string, quiet bool) error {
	if !quiet {
		fmt.Println()
		fmt.Println(cmdutil.Bold("Validating E2E..."))
		fmt.Println()
	}

	// An Inrun e2e test: Inrun checks its own fields, maps the test to jig
	// offline, and jig checks the rest. A plain jig test: jig checks it all.
	var t *jig.Test
	var problems []error
	if e2e.IsE2EDoc(path) {
		t, problems = e2e.Validate(path)
	} else {
		var err error
		if t, err = jig.FromFile(path); err != nil {
			return err
		}
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Printf("  %s %s\n", cmdutil.FailureMark(), p)
		}
		fmt.Println()
		return fmt.Errorf("%d validation error(s) in %s", len(problems), path)
	}
	if !quiet {
		jig.Plan(os.Stdout, t)
	}
	return nil
}

// ValidateSimulateFile validates a Simulate spec file and prints a summary.
func ValidateSimulateFile(path string) error {
	return validateSimulateFileOpts(path, false)
}

// ValidateSimulateFileQuiet validates without printing; the returned error
// lists every problem.
func ValidateSimulateFileQuiet(path string) error {
	return validateSimulateFileOpts(path, true)
}

func validateSimulateFileOpts(path string, quiet bool) error {
	if !quiet {
		fmt.Println()
		fmt.Println(cmdutil.Bold("Validating Simulate..."))
		fmt.Println()
	}

	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var doc types.Simulate
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	baseDir := filepath.Dir(path)
	isAggregator := len(doc.Imports) > 0 && doc.Spec == nil

	var errs []string

	if doc.Metadata.Name == "" {
		errs = append(errs, "metadata.name is required")
	}
	if !isAggregator && doc.Spec == nil {
		errs = append(errs, "spec or imports is required")
	}
	if isAggregator {
		for _, f := range doc.Imports {
			p := f
			if !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}
			if !cmdutil.FileExists(p) {
				errs = append(errs, "import not found: "+f)
			}
		}
	}
	if doc.Spec != nil {
		if doc.Spec.Blueprint == "" {
			errs = append(errs, "spec.blueprint is required")
		} else if !cmdutil.FileExists(filepath.Join(baseDir, doc.Spec.Blueprint)) {
			errs = append(errs, "spec.blueprint not found: "+doc.Spec.Blueprint)
		}
		switch {
		case doc.Spec.Intent != "" && len(doc.Spec.AllCRPaths()) > 0:
			errs = append(errs, "set spec.intent or spec.cr/crFiles, not both")
		case doc.Spec.Intent != "":
			if !cmdutil.FileExists(filepath.Join(baseDir, doc.Spec.Intent)) {
				errs = append(errs, "intent file not found: "+doc.Spec.Intent)
			}
			if doc.Spec.Token == "" {
				errs = append(errs, "spec.token is required with spec.intent: the gateway token the intent is sent with")
			}
		case len(doc.Spec.AllCRPaths()) == 0:
			errs = append(errs, "spec.cr, spec.crFiles or spec.intent is required")
		default:
			for _, p := range doc.Spec.AllCRPaths() {
				if !cmdutil.FileExists(filepath.Join(baseDir, p)) {
					errs = append(errs, "cr file not found: "+p)
				}
			}
		}
		for _, p := range doc.Spec.AllCRDPaths() {
			if !cmdutil.FileExists(filepath.Join(baseDir, p)) {
				errs = append(errs, "crd file not found: "+p)
			}
		}
		errs = append(errs, doc.Spec.ValidateSetup()...)
		for _, p := range doc.Spec.PreparePaths() {
			if !cmdutil.FileExists(filepath.Join(baseDir, p)) {
				errs = append(errs, "setup file not found: "+p)
			}
		}
		errs = append(errs, doc.Spec.ValidateExpect(baseDir)...)
	}

	if len(errs) > 0 {
		if !quiet {
			for _, e := range errs {
				fmt.Printf("  %s %s\n", cmdutil.FailureMark(), e)
			}
			fmt.Println()
			fmt.Println(strings.Repeat("─", 60))
		}
		if quiet {
			return fmt.Errorf("%d validation error(s) in %s:\n  - %s", len(errs), path, strings.Join(errs, "\n  - "))
		}
		return fmt.Errorf("%d validation error(s) in %s", len(errs), path)
	}

	if quiet {
		return nil
	}

	// Success — print structured summary matching the Blueprint/E2E style.
	fmt.Printf("%s %s\n", cmdutil.HealthIcon("ready"), cmdutil.Bold(doc.Metadata.Name))
	if doc.Metadata.Description != "" {
		fmt.Printf("    %s\n", cmdutil.Gray(doc.Metadata.Description))
	}
	fmt.Println()

	if isAggregator {
		fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("imports : %d file(s)", len(doc.Imports))))
		for _, f := range doc.Imports {
			fmt.Printf("      %s %s\n", cmdutil.HealthIcon("ready"), cmdutil.Gray(f))
		}
	} else {
		cycles := doc.Spec.Cycles
		if cycles <= 0 {
			cycles = 10
		}
		fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("blueprint : %s", doc.Spec.Blueprint)))
		if doc.Spec.Intent != "" {
			fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("intent  : %s (token %s)", doc.Spec.Intent, doc.Spec.Token)))
		} else {
			fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("cr      : %s", doc.Spec.CR)))
		}
		fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("cycles  : %d", cycles)))
		if doc.Spec.Expect != nil {
			totalOps := len(doc.Spec.Expect.Ops)
			totalAbsent := len(doc.Spec.Expect.Absent)
			for _, sub := range doc.Spec.Expect.CRDs {
				if sub == nil {
					continue
				}
				totalOps += len(sub.Ops)
				totalAbsent += len(sub.Absent)
			}
			fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("ops     : %d rule(s)", totalOps)))
			if totalAbsent > 0 {
				fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("absent  : %d rule(s)", totalAbsent)))
			}
			if len(doc.Spec.Expect.CRDs) > 0 {
				fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("crds    : %d scoped", len(doc.Spec.Expect.CRDs))))
			}
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	if isAggregator {
		fmt.Printf("%d import(s) valid\n", len(doc.Imports))
	} else {
		ops := 0
		if doc.Spec.Expect != nil {
			ops = len(doc.Spec.Expect.Ops)
			for _, sub := range doc.Spec.Expect.CRDs {
				if sub != nil {
					ops += len(sub.Ops)
				}
			}
		}
		fmt.Printf("Simulate is valid (%d op rule(s))\n", ops)
	}
	return nil
}

// validateModuleFile runs Module-specific validation and prints results.
func validateModuleFile(path string) error {
	fmt.Println()
	fmt.Println(cmdutil.Bold("Validating Module..."))
	fmt.Println()

	errs := blueprint.ValidateModule(path)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Printf("  %s %s\n", cmdutil.FailureMark(), e.Error())
		}
		fmt.Println()
		fmt.Println(strings.Repeat("─", 60))
		return fmt.Errorf("%d validation error(s) in %s", len(errs), path)
	}

	// Load the module to build the structured summary.
	m, err := modulepkg.Load(path)
	if err != nil {
		// ValidateModule already passed, so this is unexpected — degrade gracefully.
		fmt.Printf("%s %s\n", cmdutil.HealthIcon("ready"), cmdutil.Bold(path))
	} else {
		fmt.Printf("%s %s\n", cmdutil.HealthIcon("ready"), cmdutil.Bold(m.Metadata.Name))
		if m.Metadata.Description != "" {
			fmt.Printf("    %s\n", cmdutil.Gray(m.Metadata.Description))
		}
		if m.Metadata.Version != "" {
			fmt.Println()
			fmt.Printf("    %s\n", cmdutil.Gray("version : "+m.Metadata.Version))
		}
		if len(m.Inputs) > 0 {
			if m.Metadata.Version == "" {
				fmt.Println()
			}
			fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("inputs  : %d", len(m.Inputs))))
		}
		if summary := moduleResourceSummary(m); summary != "" {
			fmt.Printf("    %s\n", cmdutil.Gray("resources: "+summary))
		}
		if summary := modulePresetSummary(m); summary != "" {
			fmt.Printf("    %s\n", cmdutil.Gray("presets : "+summary))
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Module is valid")
	return nil
}

// modulePresetSummary returns a compact string listing non-empty preset classes
// and their counts, e.g. "networkPolicies(2) resourceQuotas(1)".
func modulePresetSummary(m *types.Module) string {
	reg := m.Presets
	if reg.Empty() {
		return ""
	}
	var parts []string
	add := func(kind string, n int) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%s(%d)", kind, n))
		}
	}
	add("networkPolicies", len(reg.NetworkPolicies))
	add("resourceQuotas", len(reg.ResourceQuotas))
	add("limitRanges", len(reg.LimitRanges))
	add("hpa", len(reg.HPAs))
	add("pdb", len(reg.PDBs))
	add("rollingUpdate", len(reg.RollingUpdates))
	return strings.Join(parts, " ")
}

// moduleResourceSummary returns a compact string listing non-empty resource types
// and their counts, e.g. "deployments(1) services(1) networkPolicies(2)".
func moduleResourceSummary(m *types.Module) string {
	if m.Resources == nil {
		return ""
	}
	ht := m.Resources.HookTemplates
	var parts []string
	add := func(kind string, n int) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%s(%d)", kind, n))
		}
	}
	add("deployments", len(ht.Deployments))
	add("statefulSets", len(ht.StatefulSets))
	add("services", len(ht.Services))
	add("ingresses", len(ht.Ingresses))
	add("networkPolicies", len(ht.NetworkPolicies))
	add("jobs", len(ht.Jobs))
	add("cronJobs", len(ht.CronJobs))
	add("secrets", len(ht.Secrets))
	add("configMaps", len(ht.ConfigMaps))
	add("serviceAccounts", len(ht.ServiceAccounts))
	add("roles", len(ht.Roles))
	add("roleBindings", len(ht.RoleBindings))
	add("clusterRoles", len(ht.ClusterRoles))
	add("clusterRoleBindings", len(ht.ClusterRoleBindings))
	add("resourceQuotas", len(ht.ResourceQuotas))
	add("limitRanges", len(ht.LimitRanges))
	add("namespaces", len(ht.Namespaces))
	add("persistentVolumeClaims", len(ht.PersistentVolumeClaims))
	add("horizontalPodAutoscalers", len(ht.HorizontalPodAutoscalers))
	add("podDisruptionBudgets", len(ht.PodDisruptionBudgets))
	return strings.Join(parts, " ")
}

func printValidatePresets(reg types.PresetRegistry) {
	if reg.Empty() {
		return
	}
	type entry struct{ kind, name string }
	var entries []entry
	for _, p := range reg.Resources {
		entries = append(entries, entry{"resources", p.Name})
	}
	for _, p := range reg.ContainerSecurities {
		entries = append(entries, entry{"containerSecurity", p.Name})
	}
	for _, p := range reg.PodSecurities {
		entries = append(entries, entry{"podSecurity", p.Name})
	}
	for _, p := range reg.HPAs {
		entries = append(entries, entry{"hpa", p.Name})
	}
	for _, p := range reg.PDBs {
		entries = append(entries, entry{"pdb", p.Name})
	}
	for _, p := range reg.RollingUpdates {
		entries = append(entries, entry{"rollingUpdate", p.Name})
	}
	for _, p := range reg.Reconciler {
		entries = append(entries, entry{"reconciler", p.Name})
	}
	for _, p := range reg.NetworkPolicies {
		entries = append(entries, entry{"networkPolicies", p.Name})
	}
	for _, p := range reg.ResourceQuotas {
		entries = append(entries, entry{"resourceQuotas", p.Name})
	}
	for _, p := range reg.LimitRanges {
		entries = append(entries, entry{"limitRanges", p.Name})
	}
	for _, p := range reg.Probes {
		entries = append(entries, entry{"probes", p.Name})
	}
	maxLen := 0
	for _, e := range entries {
		if l := len(e.kind); l > maxLen {
			maxLen = l
		}
	}
	fmt.Println()
	fmt.Printf("%s\n", cmdutil.Bold(fmt.Sprintf("Presets (%d)", len(entries))))
	for _, e := range entries {
		fmt.Printf("  %s   %s\n", cmdutil.PadRight(cmdutil.Cyan(e.kind), maxLen), cmdutil.Gray(e.name))
	}
}

func printRuntimeContext() {
	fmt.Println()
	fmt.Printf("%s\n", cmdutil.Bold("Runtime context (.inrun.*)"))
	fmt.Printf("  %s   %s\n", cmdutil.PadRight(cmdutil.Cyan(".inrun.inPod"), 16), cmdutil.Gray("true when Inrun is running inside a Kubernetes pod"))
	fmt.Printf("  %s   %s\n", cmdutil.PadRight(cmdutil.Cyan(".inrun.namespace"), 16), cmdutil.Gray("namespace Inrun is deployed in"))
	fmt.Printf("  %s   %s\n", cmdutil.PadRight(cmdutil.Cyan(".inrun.version"), 16), cmdutil.Gray("running Inrun version string"))
}

func printValidateFunctions(reg types.KELFunctionRegistry) {
	if reg.Empty() {
		return
	}
	maxLen := 0
	for _, n := range reg.Definitions {
		if l := len(n.Name); l > maxLen {
			maxLen = l
		}
	}
	fmt.Println()
	fmt.Printf("%s\n", cmdutil.Bold(fmt.Sprintf("Functions (%d)", len(reg.Definitions))))
	for _, n := range reg.Definitions {
		fmt.Printf("  %s   %s\n", cmdutil.PadRight(cmdutil.Cyan(n.Name), maxLen), cmdutil.Gray(n.Expression))
	}
}

func init() {
	cmdutil.RootCmd.AddCommand(validateCmd)

	validateCmd.Flags().StringSliceP("file", "f", nil, "Path to an Inrun pattern (repeatable or comma-separated)")
	validateCmd.Flags().Bool("full", false, "Show per-CRD permissions, dependency graph, and system-level RBAC")
	validateCmd.Flags().Bool("functions", false, "Quiet mode: print only the merged function registry, skip full validate output")
	validateCmd.Flags().Bool("presets", false, "Quiet mode: print only the merged preset registry, skip full validate output")

	// Shadow global flags so they don't appear under `inrun validate`
	cmdutil.ShadowGlobalCommandFlags(validateCmd)
}
