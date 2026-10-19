//go:build !runtime && !gateway

package validate

// printEnrichmentReport prints the enrichment summary as part of inrun validate output.
//
// Example output for a Blueprint with built-in and custom CRDs:
//
//   ✓ deployment-governance
//     kind: Deployment → enriched from built-in registry
//     group: apps / version: v1 / plural: deployments / scope: Namespaced
//
//   ✓ pod-governance
//     kind: Pod → enriched from built-in registry
//     group: core / version: v1 / plural: pods / scope: Namespaced
//
//   ✓ website
//     kind: Website / group: demo.inrun.dev / version: v1alpha1 / plural: websites
//
//   ✓ platform-namespace
//     kind: PlatformNamespace / group: platform.inrun.dev / version: v1alpha1

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
	rbacv1 "k8s.io/api/rbac/v1"
)

// printCRDValidationLine prints one CRD entry's validation result.
// Shows enrichment clearly when it occurred, and any warnings.
func printCRDValidationLine(bp *blueprint.Blueprint, entry types.CRDEntry) {
	printCRDHeader(entry)
	strictModeText := getStrictModeText(entry, bp.IsStrictModeEnabled())
	printKindInfo(entry)
	printModeResync(entry)
	printProtectionStatus(entry, bp.IsDeletionProtectionEnabled(), strictModeText)
	printInfo(entry.Info, bp.Info)
	printWarnings(entry.Warnings, bp.Warnings)
}

// printCRDHeader prints the CRD name with appropriate icon.
func printCRDHeader(entry types.CRDEntry) {
	icon := cmdutil.HealthIconReady()
	if entry.Warnings.HasWarnings() {
		icon = cmdutil.HealthIconWarn()
	}
	if entry.Info.HasInfo() {
		icon = cmdutil.HealthIconInfo()
	}
	fmt.Printf("%s %s\n", icon, cmdutil.Bold(entry.Name))
}

// getStrictModeText returns a formatted string indicating strict mode status,
// or an empty string if strict mode is not enforced for this CRD.
func getStrictModeText(entry types.CRDEntry, blueprintStrictMode bool) string {
	if blueprintStrictMode && entry.IsStrictDeletionProtection(blueprintStrictMode) {
		return cmdutil.InfoMark() + " (strict)"
	}
	return ""
}

// printKindInfo prints the kind/group/version/plural/scope line.
// For built‑in types, it shows enrichment info; for custom, the declared values.
func printKindInfo(entry types.CRDEntry) {
	scope := "Namespaced"
	if !entry.IsNamespaced() {
		scope = "ClusterScoped"
	}

	if entry.IsBuiltIn {
		fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf(
			"kind: %s → enriched from built-in registry", cmdutil.Bold(entry.APITypes.Kind),
		)))
		fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf(
			"group: %s / version: %s / plural: %s / scope: %s",
			entry.BuiltInGroup, entry.APITypes.Version, entry.APITypes.Plural, scope,
		)))
	} else {
		fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf(
			"kind: %s / group: %s / version: %s / plural: %s / scope: %s",
			entry.APITypes.Kind, entry.APITypes.Group, entry.APITypes.Version, entry.APITypes.Plural, scope,
		)))
	}
}

// printModeResync prints the mode, workers, and resync period.
func printModeResync(entry types.CRDEntry) {
	fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf(
		"mode: %s / workers: %v / resync: %v",
		entry.Mode, entry.SetWorkers(0), entry.SetResync(0).String(),
	)))
}

// printProtectionStatus prints the protection level for custom CRDs.
// For built‑ins, a simplified message is shown.
// printProtectionStatus prints the protection level for custom CRDs.
// For built‑ins, a simplified message is shown.
// When strict mode is active, the label changes from "protection:" to "strict‑protection:".
func printProtectionStatus(entry types.CRDEntry, blueprintProtected bool, strictModeText string) {
	if entry.IsBuiltInType() {
		fmt.Printf("    %s\n", cmdutil.Gray("protection: label-based (built-in)"))
		return
	}

	if !blueprintProtected {
		return
	}

	protectCRD := entry.ShouldProtectCRD()
	protectCRs := entry.ShouldProtectCRs()
	strictActive := strictModeText != "" // strictModeText already contains the arrow+text, but we only need a boolean

	var icon string
	var baseText string
	switch {
	case protectCRD && protectCRs:
		icon = cmdutil.SecureMark()
		baseText = " full (CRD + CRs)"
	case protectCRD && !protectCRs:
		icon = cmdutil.SomeSecureMark()
		baseText = "CRD only (CRs not protected)"
	case !protectCRD && protectCRs:
		icon = cmdutil.WarningMark()
		baseText = "CRs only (CRD not protected - see warning)"
	default:
		icon = cmdutil.NoSecurityMark()
		baseText = "none"
	}

	// Choose label based on strict mode
	label := "protection:"
	if strictActive {
		label = "strict-protection:"
	}
	fullText := fmt.Sprintf("%s %s %s", label, icon, baseText)
	fmt.Printf("    %s\n", cmdutil.Gray(fullText))
}

// printWarnings prints any warnings associated with the CRD or Blueprint,
// with proper indentation.
func printWarnings(crdWarnings, blueprintWarnings []string) {
	// Print CRD-level warnings
	for _, w := range crdWarnings {
		lines := strings.Split(w, "\n")
		for i, line := range lines {
			if i == 0 {
				fmt.Printf("    %s warning: %s\n", cmdutil.Yellow("⚠"), cmdutil.Gray(line))
			} else {
				fmt.Printf("             %s\n", cmdutil.Gray(line))
			}
		}
	}

	// Print Blueprint-level warnings
	for _, w := range blueprintWarnings {
		lines := strings.Split(w, "\n")
		for i, line := range lines {
			if i == 0 {
				fmt.Printf("    %s warning: %s\n", cmdutil.Yellow("⚠"), cmdutil.Gray(line))
			} else {
				fmt.Printf("             %s\n", cmdutil.Gray(line))
			}
		}
	}
}

// printInfo prints any informational messages associated with the CRD or Blueprint,
// with proper indentation.
func printInfo(crdInfo, blueprintInfo []string) {
	// Print CRD-level info
	for _, msg := range crdInfo {
		lines := strings.Split(msg, "\n")
		for i, line := range lines {
			if i == 0 {
				fmt.Printf("    %s info: %s\n", cmdutil.Blue(cmdutil.InfoMark()), cmdutil.Gray(line))
			} else {
				fmt.Printf("          %s\n", cmdutil.Gray(line))
			}
		}
	}

	// Print Blueprint-level info
	for _, msg := range blueprintInfo {
		lines := strings.Split(msg, "\n")
		for i, line := range lines {
			if i == 0 {
				fmt.Printf("    %s info: %s\n", cmdutil.Blue(cmdutil.InfoMark()), cmdutil.Gray(line))
			} else {
				fmt.Printf("          %s\n", cmdutil.Gray(line))
			}
		}
	}
}

// ── Full mode (--full) ────────────────────────────────────────────────────────

// printCRDPermissions prints RBAC rules attributed to a CRD under its header.
// No-op when rules is empty.
func printCRDPermissions(rules []rbacv1.PolicyRule) {
	if len(rules) == 0 {
		return
	}
	maxGroup, maxRes := 0, 0
	for _, r := range rules {
		if n := len(rbacGroup(r.APIGroups)); n > maxGroup {
			maxGroup = n
		}
		if n := len(rbacRes(r)); n > maxRes {
			maxRes = n
		}
	}
	fmt.Printf("    %s\n", cmdutil.Gray("permissions:"))
	for _, r := range rules {
		fmt.Printf("      %s\n", cmdutil.Gray(fmt.Sprintf(
			"%-*s  %-*s  %s",
			maxGroup, rbacGroup(r.APIGroups),
			maxRes, rbacRes(r),
			strings.Join(r.Verbs, " "),
		)))
	}
}

// printValidateDependencyGraph prints the startup-order dependency section for validate --full.
// Only called when there are dependencies (dd is never nil here).
func printValidateDependencyGraph(dd *blueprint.DependencyDisplay) {
	fmt.Println()
	fmt.Println(cmdutil.Bold("startup order"))
	maxName := 0
	for _, name := range dd.StartupOrder {
		if len(name) > maxName {
			maxName = len(name)
		}
	}
	for i, name := range dd.StartupOrder {
		depNames := dd.SortedDeps(name)
		suffix := ""
		if len(depNames) > 0 {
			parts := make([]string, 0, len(depNames))
			for _, dep := range depNames {
				parts = append(parts, fmt.Sprintf("%s [%s]", dep, dd.Conditions[name][dep]))
			}
			suffix = "   ← " + strings.Join(parts, " · ")
		}
		fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("%d  %-*s%s", i+1, maxName, name, suffix)))
	}
}

// printRuntimePermissionsSection prints the runtime system RBAC rules.
func printRuntimePermissionsSection(rules []rbacv1.PolicyRule) {
	if len(rules) == 0 {
		return
	}
	fmt.Println()
	fmt.Println(cmdutil.Bold("runtime"))
	printRuleBlock(rules, nil)
}

// printGatewayPermissionsSection prints the gateway system RBAC rules with
// contextual notes on secrets (TLS) and namespaces (deletion-protection).
func printGatewayPermissionsSection(rules []rbacv1.PolicyRule) {
	if len(rules) == 0 {
		return
	}
	fmt.Println()
	fmt.Println(cmdutil.Bold("gateway"))
	printRuleBlock(rules, map[string]string{
		// set TLS_CERT / TLS_KEY in inrun-deployment to bring your own
		"secrets":    "Inrun provisions and rotates certs",
		"namespaces": "labels inrun-system to activate the deletion-protection admission scope",
	})
}

// printRuleBlock prints RBAC policy rules with aligned columns and optional per-resource notes.
func printRuleBlock(rules []rbacv1.PolicyRule, notes map[string]string) {
	maxGroup, maxRes := 0, 0
	for _, r := range rules {
		if n := len(rbacGroup(r.APIGroups)); n > maxGroup {
			maxGroup = n
		}
		if n := len(rbacRes(r)); n > maxRes {
			maxRes = n
		}
	}
	for _, r := range rules {
		line := fmt.Sprintf("  %-*s  %-*s  %s",
			maxGroup, rbacGroup(r.APIGroups),
			maxRes, rbacRes(r),
			strings.Join(r.Verbs, " "),
		)
		if notes != nil && len(r.Resources) > 0 {
			if note, ok := notes[r.Resources[0]]; ok {
				line += "   ← " + note
			}
		}
		fmt.Println(cmdutil.Gray(line))
	}
}

// rbacGroup returns a display-friendly API group; empty group → "core".
func rbacGroup(groups []string) string {
	if len(groups) == 0 || groups[0] == "" {
		return "core"
	}
	return groups[0]
}

// rbacRes returns the resource display string, appending a bracketed resource name
// for narrowly-scoped rules (e.g. CA-bundle patch).
func rbacRes(r rbacv1.PolicyRule) string {
	if len(r.Resources) == 0 {
		return ""
	}
	res := r.Resources[0]
	if len(r.ResourceNames) > 0 {
		res += " [" + r.ResourceNames[0] + "]"
	}
	return res
}

// ── Presets display ─────────────────────────────────────────────────────────

type presetLine struct {
	typLabel string
	preset   string
	location string
	mixed    bool
}

// printCRDPresets prints named presets declared for a CRD under its header.
// No-op when the CRD declares no presets.
func printCRDPresets(entry types.CRDEntry) {
	lines := collectPresetLines(entry)
	if len(lines) == 0 {
		return
	}
	maxType, maxPreset := 0, 0
	for _, l := range lines {
		if n := len(l.typLabel); n > maxType {
			maxType = n
		}
		if n := len(l.preset); n > maxPreset {
			maxPreset = n
		}
	}
	fmt.Printf("    %s\n", cmdutil.Gray("presets:"))
	for _, l := range lines {
		suffix := ""
		if l.mixed {
			suffix = "   ← mixed with explicit fields"
		}
		fmt.Printf("      %s\n", cmdutil.Gray(fmt.Sprintf(
			"%-*s  %-*s  %s%s",
			maxType, l.typLabel,
			maxPreset, l.preset,
			l.location,
			suffix,
		)))
	}
}

func collectPresetLines(entry types.CRDEntry) []presetLine {
	var lines []presetLine

	for _, e := range entry.CollectSecurityPresetEntries() {
		label := "security (container)"
		if e.Kind == "pod" {
			label = "security (pod)"
		}
		lines = append(lines, presetLine{
			typLabel: label,
			preset:   e.Preset,
			location: presetLoc(e.Resource, e.ResourceName, e.Phase),
			mixed:    e.Mixed,
		})
	}

	for _, e := range entry.CollectResourcePresetEntries() {
		lines = append(lines, presetLine{
			typLabel: "resources",
			preset:   e.Preset,
			location: presetLoc(e.Resource, e.ResourceName, e.Phase),
			mixed:    e.Mixed,
		})
	}

	for _, e := range entry.CollectHPAPresetEntries() {
		lines = append(lines, presetLine{
			typLabel: "hpa",
			preset:   e.Preset,
			location: presetLoc("", e.ResourceName, e.Phase),
			mixed:    e.Mixed,
		})
	}

	for _, e := range entry.CollectPDBPresetEntries() {
		lines = append(lines, presetLine{
			typLabel: "pdb",
			preset:   e.Preset,
			location: presetLoc("", e.ResourceName, e.Phase),
			mixed:    e.Mixed,
		})
	}

	for _, e := range entry.CollectRollingUpdatePresetEntries() {
		lines = append(lines, presetLine{
			typLabel: "rolling",
			preset:   e.Preset,
			location: presetLoc("", e.ResourceName, e.Phase),
			mixed:    e.Mixed,
		})
	}

	for _, e := range entry.CollectProbePresetEntries() {
		lines = append(lines, presetLine{
			typLabel: "probes (" + e.ProbeType + ")",
			preset:   e.Preset,
			location: presetLoc(e.Resource, e.ResourceName, e.Phase),
			mixed:    e.Mixed,
		})
	}

	return lines
}

// presetLoc builds a compact location string from resource kind, name, and phase.
func presetLoc(resource, name, phase string) string {
	switch {
	case resource != "" && name != "":
		return resource + "/" + name + " [" + phase + "]"
	case resource != "":
		return resource + " [" + phase + "]"
	case name != "":
		return name + " [" + phase + "]"
	default:
		return "[" + phase + "]"
	}
}
