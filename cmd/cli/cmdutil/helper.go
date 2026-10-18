//go:build !runtime && !gateway

package cmdutil

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/kube"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/intentd/inrun/pkg/util/term"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ── utils aliases ────────────────────────────────────────────────────────────
// Import utils once here. All other files in this package use these names
// directly — no per-file utils import needed.

var (
	// colors / styles
	Gray    = term.Gray
	Bold    = term.Bold
	Dim     = term.Dim
	Cyan    = term.Cyan
	Green   = term.Green
	Blue    = term.Blue
	White   = term.White
	Yellow  = term.Yellow
	Red     = term.Red
	magenta = term.Magenta

	// marks and icons
	SuccessMark     = term.SuccessMark
	FailureMark     = term.FailureMark
	WarningMark     = term.WarningMark
	InfoMark        = term.InfoMark
	SecureMark      = term.SecureMark
	SomeSecureMark  = term.SomeSecureMark
	NoSecurityMark  = term.NoSecurityMark
	HealthIcon      = term.HealthIcon
	HealthIconReady = term.HealthIconReady
	HealthIconWarn  = term.HealthIconWarning
	HealthIconInfo  = term.HealthIconInfo

	// other cli utilities
	IsRunningInPod      = kube.IsRunningInPod
	WriteFileAndFormat  = structured.WriteFileAndFormat
	SplitCommaSeparated = structured.SplitCommaSeparated
	ReadLocal           = fileio.ReadLocal
	StrictUnmarshal     = structured.StrictUnmarshal
	PruneEmptyYAML      = structured.PruneEmptyYAML
	CopyFile            = fileio.CopyFile
	CopyDir             = fileio.CopyDir
	JoinPath            = fileio.JoinRelative
	FormatSize          = term.FormatSize
	WordWrap            = term.WordWrap
	visibleLen          = term.VisibleLen
	PadRight            = term.PadRight
	OrDefault           = term.OrDefault
	ContainsTag         = term.ContainsFold
	IsTerminal          = term.IsTerminal
	HumanDuration       = term.FormatDuration
	DumpCRsJSON         = kube.DumpCRsJSON
	DumpCRsYAML         = kube.DumpCRsYAML
)

// SortedKeys returns a sorted slice of all keys from the given map.
func SortedKeys[V any](m map[string]V) []string {
	return structured.SortedKeys(m)
}

// StartSpinner starts a terminal progress spinner with the given message.
// Call Success, Failure, or Stop on the returned value when done.
func StartSpinner(msg string) *term.Spinner {
	return term.StartSpinner(msg)
}

// Shadow global command flags
func ShadowGlobalCommandFlags(cmd *cobra.Command, flags ...string) {
	// Shadow standard global flags for all commands
	if cmd.Parent() != nil {
		if cmd.Flags().Lookup("debug") == nil {
			cmd.Flags().Bool("debug", false, "")
			cmd.Flags().MarkHidden("debug")
		}
		if cmd.Flags().Lookup("kubeconfig") == nil {
			cmd.Flags().String("kubeconfig", "", "")
			cmd.Flags().MarkHidden("kubeconfig")
		}
		if cmd.Flags().Lookup("verbose") == nil {
			cmd.Flags().Bool("verbose", false, "")
			cmd.Flags().MarkHidden("verbose")
		}
	}

	// Shadow specific flags passed as arguments
	for _, f := range flags {
		if cmd.PersistentFlags().Lookup(f) == nil {
			switch f {
			case "file":
				cmd.PersistentFlags().StringSlice("file", nil, "")
				cmd.PersistentFlags().MarkHidden("file")
				// case "placeholder":
				// 	cmd.PersistentFlags().StringP("placeholder", "p", "", "")
				// 	cmd.PersistentFlags().MarkHidden("placeholder")
				// Add other flags as needed
			}
		}
	}

	// Process all subcommands
	for _, subCmd := range cmd.Commands() {
		ShadowGlobalCommandFlags(subCmd, flags...)
	}
}

// ── printTemplateSummary ──────────────────────────────────────────────────────

// PrintTemplateSummary prints the human-readable default output of `inrun template`.
// CRDs are listed in startup order so the user sees the dependency sequence.
func PrintTemplateSummary(cat *catalog.Catalog, crds map[string]types.CRDEntry, startupOrder []string) {
	meta := cat.Metadata()

	fmt.Printf("\n%s", Cyan(Bold("Catalog")))
	if meta.Name != "" {
		fmt.Printf(": %s", Bold(meta.Name))
	}
	if meta.Version != "" {
		fmt.Printf(" %s", Dim("("+meta.Version+")"))
	}
	if cat.APIVersion != "" {
		fmt.Printf("  %s", Dim(cat.APIVersion))
	}
	fmt.Println()

	fmt.Printf("  %d CRD(s) — startup order: %s\n\n",
		len(crds),
		Yellow(strings.Join(startupOrder, " → ")),
	)

	for i, name := range startupOrder {
		crd, ok := crds[name]
		if !ok {
			continue
		}
		isLast := i == len(startupOrder)-1
		connector := "├─"
		if isLast {
			connector = "└─"
		}

		gvk := fmt.Sprintf("%s/%s, Kind=%s", crd.APITypes.Group, crd.APITypes.Version, crd.APITypes.Kind)
		fmt.Printf("  %s %s  %s\n",
			connector,
			Bold(name),
			Dim(gvk),
		)

		indent := "  │   "
		if isLast {
			indent = "      "
		}

		// Workers / resync
		fmt.Printf("%sworkers:%s  resync:%s",
			indent,
			Green(fmt.Sprintf("%d", crd.SetWorkers(0))),
			Green(crd.SetResync(0).String()),
		)
		if d := crd.SetQueueDepth(0); d > 0 {
			fmt.Printf("  queue:%s", Green(fmt.Sprintf("%d", d)))
		}
		fmt.Println()

		// DependsOn
		if len(crd.DependsOn) > 0 {
			deps := make([]string, 0, len(crd.DependsOn))
			for depName, d := range crd.DependsOn {
				if d.Condition != "" && d.Condition != "started" {
					deps = append(deps, fmt.Sprintf("%s(%s)", depName, d.Condition))
				} else {
					deps = append(deps, depName)
				}
			}
			fmt.Printf("%s%s %s\n",
				indent,
				Yellow("dependsOn:"),
				strings.Join(deps, ", "),
			)
		}

		// Mode / reconciler
		mode := "default"
		if crd.Mode != "" {
			mode = string(crd.Mode)
		}
		if !crd.DefaultReconcile() {
			if crd.CustomHooksEnabled() {
				mode = "hooks"
			} else if crd.ConstructorEnabled() {
				mode = "constructor"
			}
		}
		fmt.Printf("%smode: %s\n", indent, mode)

		// onCreate resources
		if crd.Box().EffectiveOnCreate() != nil && !crd.Box().EffectiveOnCreate().Empty() {
			fmt.Printf("%s%s  %s\n", indent, Cyan("onCreate:"),
				summarizeHookTemplates(crd.Box().EffectiveOnCreate()))
		}

		// onReconcile resources
		if crd.Box().EffectiveOnReconcile() != nil && !crd.Box().EffectiveOnReconcile().Empty() {
			fmt.Printf("%s%s  %s\n", indent, Cyan("onReconcile:"),
				summarizeHookTemplates(crd.Box().EffectiveOnReconcile()))
		}

		// Status fields
		if s := crd.Box().EffectiveStatus(); s != nil && len(s.Fields) > 0 {
			fieldNames := make([]string, 0, len(s.Fields))
			for _, f := range s.Fields {
				fieldNames = append(fieldNames, f.Path)
			}
			fmt.Printf("%s%s  %s\n", indent, Cyan("status:"),
				strings.Join(fieldNames, ", "))
		}

		// Autoscale
		if crd.AutoscaleEnabled() {
			a := crd.Box().EffectiveAutoscale()
			if a.Profile != "" {
				fmt.Printf("%s%s  profile=%s\n", indent, magenta("autoscale:"), a.Profile)
			} else {
				parts := []string{}
				if len(a.Conditions.When) > 0 {
					triggers := make([]string, 0, len(a.Conditions.When))
					for _, c := range a.Conditions.When {
						triggers = append(triggers, c.Field)
					}
					parts = append(parts, "when("+strings.Join(triggers, ", ")+")")
				}
				if a.Do.Workers != nil {
					parts = append(parts, fmt.Sprintf("workers→%d", *a.Do.Workers))
				}
				if a.Do.Resync != nil {
					parts = append(parts, fmt.Sprintf("resync→%s", a.Do.Resync.String()))
				}
				fmt.Printf("%s%s  %s  interval:%s  cooldown:%s\n",
					indent, magenta("autoscale:"),
					strings.Join(parts, "  "),
					a.EffectiveInterval(), a.EffectiveCooldown(),
				)
			}
		}

		fmt.Println()
	}

	fmt.Printf("  %s\n\n", Green("✓ Catalog is valid"))
}

// ── printDependencyGraph ──────────────────────────────────────────────────────

// PrintDependencyGraph prints a two-part dependency view:
// 1. Ordered startup list (flat)
// 2. Tree view showing the dependency hierarchy
func PrintDependencyGraph(crds map[string]types.CRDEntry, g *catalog.DependencyGraph, startupOrder []string) {
	fmt.Printf("\n%s\n\n", Cyan(Bold("Dependency Graph")))

	// ── Part 1: startup order list ────────────────────────────────────────────
	fmt.Printf("  %s\n", Bold("Startup order:"))
	for i, name := range startupOrder {
		crd, ok := crds[name]
		if !ok {
			continue
		}
		deps := crd.DependsOn.Names()
		depStr := ""
		if len(deps) > 0 {
			formatted := make([]string, 0, len(deps))
			for depName, d := range crd.DependsOn {
				if d.Condition != "" && d.Condition != "started" {
					formatted = append(formatted, fmt.Sprintf("%s(%s)",
						Yellow(depName),
						Dim(d.Condition)))
				} else {
					formatted = append(formatted, Yellow(depName))
				}
			}
			depStr = fmt.Sprintf("  %s %s", Dim("←"), strings.Join(formatted, ", "))
		}
		gvk := fmt.Sprintf("%s/%s, Kind=%s", crd.APITypes.Group, crd.APITypes.Version, crd.APITypes.Kind)
		fmt.Printf("    %s %-20s %s%s\n",
			Bold(fmt.Sprintf("%d.", i+1)),
			name,
			Dim(gvk),
			depStr,
		)
	}

	fmt.Println()

	// ── Part 2: tree view ─────────────────────────────────────────────────────
	fmt.Printf("  %s\n", Bold("Tree view:"))

	// Find roots (CRDs with no dependencies)
	roots := []string{}
	for _, name := range startupOrder {
		crd, ok := crds[name]
		if !ok {
			continue
		}
		if len(crd.DependsOn) == 0 {
			roots = append(roots, name)
		}
	}

	printed := map[string]bool{}
	for _, root := range roots {
		printGraphNode(crds, g, root, "    ", "", printed)
	}
	fmt.Println()
}

// printGraphNode recursively prints one CRD node and its dependents in tree form.
func printGraphNode(crds map[string]types.CRDEntry, g *catalog.DependencyGraph, name, indent, connector string, printed map[string]bool) {
	crd, ok := crds[name]
	if !ok {
		return
	}

	gvk := fmt.Sprintf("%s/%s", crd.APITypes.Group, crd.APITypes.Version)
	fmt.Printf("%s%s%s  %s\n",
		indent, connector,
		Bold(name),
		Dim(gvk),
	)

	if printed[name] {
		return
	}
	printed[name] = true

	dependents := g.GetDependents(name)
	sort.Strings(dependents)

	for i, dep := range dependents {
		isLast := i == len(dependents)-1
		childConnector := "├── "
		childIndent := indent + "│   "
		if isLast {
			childConnector = "└── "
			childIndent = indent + "    "
		}

		// Show the condition for this edge
		depCrd, ok := crds[dep]
		if ok {
			if d, exists := depCrd.DependsOn[name]; exists && d.Condition != "" && d.Condition != "started" {
				childConnector += Dim("("+d.Condition+")") + " "
			}
		}
		printGraphNode(crds, g, dep, childIndent, childConnector, printed)
	}
}

// ── printCRDDetail ────────────────────────────────────────────────────────────

// PrintCRDDetail prints the full expanded state of a single CRD as a
// human-readable document — what the runtime will use for this CRD.
func PrintCRDDetail(crd types.CRDEntry, g *catalog.DependencyGraph) {
	fmt.Printf("\n%s\n", Bold(crd.Name))
	fmt.Printf("  %s %s/%s\n", Cyan("APIVersion:"), crd.APITypes.Group, crd.APITypes.Version)
	fmt.Printf("  %s %s  (plural: %s)\n", Cyan("Kind:     "), crd.APITypes.Kind, crd.APITypes.Plural)
	fmt.Printf("  %s %v", Cyan("Namespaced:"), crd.IsNamespaced())
	if crd.Namespace != "" {
		fmt.Printf("  (namespace: %s)", crd.Namespace)
	}
	fmt.Println()
	fmt.Printf("  %s %v\n", Cyan("Enabled:  "), crd.IsEnabled())
	fmt.Printf("  %s %s\n", Cyan("Mode:     "), crdModeLabel(crd))
	fmt.Println()

	// ── Runtime config ───────────────────────────────────────────────────────
	fmt.Printf("  %s\n", Cyan(Bold("Runtime")))
	fmt.Printf("    Workers:       %s\n", Green(fmt.Sprintf("%d", crd.SetWorkers(0))))
	fmt.Printf("    Resync:        %s\n", Green(crd.SetResync(0).String()))
	if d := crd.SetQueueDepth(0); d > 0 {
		fmt.Printf("    MaxDepth: %s\n", Green(fmt.Sprintf("%d", d)))
	}
	fmt.Println()

	// ── DependsOn ─────────────────────────────────────────────────────────────
	if len(crd.DependsOn) > 0 {
		fmt.Printf("  %s\n", Yellow(Bold("DependsOn")))
		for depName, dep := range crd.DependsOn {
			cond := dep.Condition
			if cond == "" {
				cond = "started"
			}
			fmt.Printf("    - %s  condition: %s\n", Yellow(depName), cond)
		}
		fmt.Println()
	}

	// ── OperatorBox.OnCreate ──────────────────────────────────────────────────
	if crd.Box().EffectiveOnCreate() != nil && !crd.Box().EffectiveOnCreate().Empty() {
		fmt.Printf("  %s\n", Cyan(Bold("onCreate")))
		printHookTemplateDetail("    ", crd.Box().EffectiveOnCreate())
		fmt.Println()
	}

	// ── OperatorBox.OnReconcile ───────────────────────────────────────────────
	if crd.Box().EffectiveOnReconcile() != nil && !crd.Box().EffectiveOnReconcile().Empty() {
		fmt.Printf("  %s\n", Cyan(Bold("onReconcile")))
		printHookTemplateDetail("    ", crd.Box().EffectiveOnReconcile())
		fmt.Println()
	}

	// ── Status ────────────────────────────────────────────────────────────────
	if s := crd.Box().EffectiveStatus(); s != nil && len(s.Fields) > 0 {
		fmt.Printf("  %s\n", Cyan(Bold("Status Fields")))
		for _, f := range s.Fields {
			fmt.Printf("    - %s: %s\n", Green(f.Path), Dim(f.Value))
		}
		fmt.Println()
	}

	// ── Autoscale ─────────────────────────────────────────────────────────────
	if crd.AutoscaleEnabled() {
		a := crd.Box().EffectiveAutoscale()
		fmt.Printf("  %s\n", magenta(Bold("Autoscale")))
		if a.Profile != "" {
			fmt.Printf("    Profile:  %s\n", magenta(a.Profile))
		} else {
			fmt.Printf("    Interval: %s\n", a.EffectiveInterval())
			fmt.Printf("    Cooldown: %s\n", a.EffectiveCooldown())
			if len(a.Conditions.When) > 0 {
				fmt.Printf("    When (AND):\n")
				for _, cond := range a.Conditions.When {
					printConditionLine("      ", cond)
				}
			}
			if len(a.Conditions.Or) > 0 {
				fmt.Printf("    Or (OR):\n")
				for _, cond := range a.Conditions.Or {
					printConditionLine("      ", cond)
				}
			}
			if a.Do.Workers != nil {
				fmt.Printf("    Do.Workers:    %s\n", magenta(fmt.Sprintf("%d", *a.Do.Workers)))
			}
			if a.Do.QueueDepth != nil {
				fmt.Printf("    Do.QueueDepth: %s\n", magenta(fmt.Sprintf("%d", *a.Do.QueueDepth)))
			}
			if a.Do.Resync != nil {
				fmt.Printf("    Do.Resync:     %s\n", magenta(a.Do.Resync.String()))
			}
		}
		fmt.Println()
	}

	// ── Finalizers ────────────────────────────────────────────────────────────
	if finals := crd.Box().EffectiveFinalizers(); len(finals) > 0 {
		fmt.Printf("  %s\n", Cyan(Bold("Finalizers")))
		for _, f := range finals {
			fmt.Printf("    - %s\n", f)
		}
		fmt.Println()
	}

	// ── Dependents (what depends on this CRD) ────────────────────────────────
	if g != nil {
		dependents := g.GetDependents(crd.Name)
		if len(dependents) > 0 {
			fmt.Printf("  %s\n", Yellow(Bold("Required by")))
			for _, dep := range dependents {
				fmt.Printf("    - %s\n", Yellow(dep))
			}
			fmt.Println()
		}
	}
}

// printHookTemplateDetail prints a detailed breakdown of resource declarations.
func printHookTemplateDetail(indent string, ht *types.HookTemplates) {
	if ht == nil {
		return
	}

	printResources := func(kind string, names []string) {
		if len(names) == 0 {
			return
		}
		fmt.Printf("%s%s\n", indent, Green(fmt.Sprintf("%s(%d):", kind, len(names))))
		for _, n := range names {
			fmt.Printf("%s  %s\n", indent, Dim("- "+n))
		}
	}

	printResources("deployments", deploymentNameList(ht.Deployments))
	printResources("statefulsets", statefulSetNameList(ht.StatefulSets))
	printResources("services", serviceNameList(ht.Services))
	printResources("configmaps", configMapNameList(ht.ConfigMaps))
	printResources("secrets", secretNameList(ht.Secrets))
	printResources("jobs", jobNameList(ht.Jobs))
	printResources("cronJobs", cronJobNameList(ht.CronJobs))
	printResources("serviceAccounts", serviceAccountNameList(ht.ServiceAccounts))
	printResources("ingresses", ingressNameList(ht.Ingresses))
	printResources("pvcs", pvcNameList(ht.PersistentVolumeClaims))
	printResources("namespaces", namespaceNameList(ht.Namespaces))
	printResources("roles", roleNameList(ht.Roles))
	printResources("clusterRoles", clusterRoleNameList(ht.ClusterRoles))

	if len(ht.CustomResource) > 0 {
		fmt.Printf("%s%s\n", indent, Green(fmt.Sprintf("custom(%d):", len(ht.CustomResource))))
		for _, cr := range ht.CustomResource {
			fmt.Printf("%s  %s\n", indent, Dim(fmt.Sprintf("- %s/%s  name: %s", cr.APIVersion, cr.Kind, cr.Metadata.Name)))
		}
	}
}

// ── summarizeHookTemplates ────────────────────────────────────────────────────

// summarizeHookTemplates returns a compact one-line resource summary.
func summarizeHookTemplates(ht *types.HookTemplates) string {
	if ht == nil {
		return ""
	}
	var parts []string
	add := func(n int, label string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, label))
		}
	}
	add(len(ht.Deployments), "deployment(s)")
	add(len(ht.StatefulSets), "statefulset(s)")
	add(len(ht.Services), "service(s)")
	add(len(ht.ConfigMaps), "configmap(s)")
	add(len(ht.Secrets), "secret(s)")
	add(len(ht.Jobs), "job(s)")
	add(len(ht.CronJobs), "cronjob(s)")
	add(len(ht.Ingresses), "ingress(es)")
	add(len(ht.PersistentVolumeClaims), "pvc(s)")
	add(len(ht.Namespaces), "namespace(s)")
	add(len(ht.ServiceAccounts), "serviceaccount(s)")
	add(len(ht.Roles), "role(s)")
	add(len(ht.ClusterRoles), "clusterrole(s)")
	if len(ht.CustomResource) > 0 {
		kinds := make([]string, 0, len(ht.CustomResource))
		for _, cr := range ht.CustomResource {
			kinds = append(kinds, cr.Kind)
		}
		parts = append(parts, fmt.Sprintf("custom(%s)", strings.Join(kinds, ", ")))
	}
	return strings.Join(parts, ", ")
}

// ── Condition printer ─────────────────────────────────────────────────────────

func printConditionLine(indent string, cond types.Condition) {
	if cond.Field != "" {
		line := fmt.Sprintf("%s- %s", indent, cond.Field)
		if cond.GreaterThan != "" {
			line += fmt.Sprintf(" > %s", Dim(cond.GreaterThan))
		}
		if cond.LessThan != "" {
			line += fmt.Sprintf(" < %s", Dim(cond.LessThan))
		}
		if cond.Equals != "" {
			line += fmt.Sprintf(" == %s", Dim(cond.Equals))
		}
		fmt.Println(line)
	}
}

// ── Name list helpers (for printHookTemplateDetail) ───────────────────────────

func deploymentNameList(srcs []types.DeploymentTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		if s.Name != "" {
			out = append(out, s.Name)
		} else {
			out = append(out, fmt.Sprintf("image:%s", s.Image))
		}
	}
	return out
}

func statefulSetNameList(srcs []types.StatefulSetTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "statefulset"))
	}
	return out
}

func serviceNameList(srcs []types.ServiceTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "service"))
	}
	return out
}

func configMapNameList(srcs []types.ConfigMapTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "configmap"))
	}
	return out
}

func secretNameList(srcs []types.SecretTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "secret"))
	}
	return out
}

func jobNameList(srcs []types.JobTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "job"))
	}
	return out
}

func cronJobNameList(srcs []types.CronJobTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "cronjob"))
	}
	return out
}

func serviceAccountNameList(srcs []types.ServiceAccountTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "serviceaccount"))
	}
	return out
}

func ingressNameList(srcs []types.IngressTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "ingress"))
	}
	return out
}

func pvcNameList(srcs []types.PVCTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "pvc"))
	}
	return out
}

func namespaceNameList(srcs []types.NamespaceTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "namespace"))
	}
	return out
}

func roleNameList(srcs []types.RoleTemplateSource) []string {
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, nameOrFallback(s.Name, "role"))
	}
	return out
}

func clusterRoleNameList(srcs []types.ClusterRoleTemplateSource) []string {
	out := make([]string, len(srcs))
	for i, s := range srcs {
		if s.Name != "" {
			out[i] = s.Name
		} else {
			out[i] = fmt.Sprintf("<clusterrole-%d>", i+1)
		}
	}
	return out
}

func nameOrFallback(name, fallback string) string {
	if name != "" {
		return name
	}
	return fmt.Sprintf("<%s>", fallback)
}

// ── printTypedOperatorHint ────────────────────────────────────────────────────

// PrintTypedBuildSteps prints the build steps for a custom runtime.
// hasMakefile=true shows the make path; false shows inrun generate + go build.
func PrintTypedBuildSteps(hasMakefile bool) {
	if hasMakefile {
		fmt.Printf("    make registry && make build\n")
	} else {
		fmt.Printf("    inrun generate registry\n")
		fmt.Printf("    go build .\n")
	}
}

// PrintTypedOperatorHint is called when a registry-sourced typed operator fails
// validation or simulate. Tells the user to pull the pattern, build the custom
// runtime, then re-run the same command.
func PrintTypedOperatorHint(err *catalog.TypedOperatorError, command string) {
	fmt.Printf("\n%s  This operator is typed — requires a custom runtime.\n\n", Yellow("⚠"))
	fmt.Printf("  Pull and build, then re-run:\n")
	fmt.Printf("    inrun pull %s -o .\n", err.Ref)
	PrintTypedBuildSteps(false) // Makefile presence unknown until pulled
	fmt.Printf("    %s\n\n", command)
}

// ── crdModeLabel ──────────────────────────────────────────────────────────────

// PrintCatalogDeprecation prints the deprecation notice for a locally validated
// catalog. Reads timeline state from the CatalogDeprecation methods.
// Prints nothing if the block is nil or today is before timeline.from.
func PrintCatalogDeprecation(d *types.CatalogDeprecation) {
	PrintCatalogDeprecationWithHint(d, "")
}

func PrintCatalogDeprecationWithHint(d *types.CatalogDeprecation, hint string) {
	if d == nil {
		return
	}
	today := time.Now()
	state := d.DeprecationState(today)
	if state == "none" {
		return
	}
	printDeprecationBlock(state, d.Message, d.MigratedTo, d.TimelineTo(), hint, d.DaysUntilEOL(today))
}

// PrintPatternDeprecation prints the deprecation notice for a registry pattern
// (inspect / pull). Reads timeline from PatternDeprecated fields.
func PrintPatternDeprecation(dep *registry.PatternDeprecated) {
	if dep == nil {
		return
	}
	d := &types.CatalogDeprecation{
		MigratedTo: dep.MigratedTo,
		Message:    dep.Message,
	}
	if dep.TimelineFrom != "" || dep.TimelineTo != "" {
		d.Timeline = &types.DeprecationTimeline{
			From: dep.TimelineFrom,
			To:   dep.TimelineTo,
		}
	}
	PrintCatalogDeprecation(d)
}

// printDeprecationBlock renders the deprecation block for a given state.
func printDeprecationBlock(state, message, migrateTo, eolDate, hint string, daysLeft int) {
	switch state {
	case "eol":
		fmt.Printf("\n%s  END OF LIFE\n", Red("✗"))
		if eolDate != "" {
			fmt.Printf("  This pattern reached end of life on %s.\n", Bold(eolDate))
		}
	default:
		fmt.Printf("\n%s  DEPRECATION WARNING\n", Yellow("⚠"))
		if daysLeft > 0 {
			fmt.Printf("  End of life in %s (%s).\n", Bold(fmt.Sprintf("%d days", daysLeft)), eolDate)
		} else if eolDate != "" {
			fmt.Printf("  End of life: %s.\n", Bold(eolDate))
		}
	}
	if message != "" {
		fmt.Printf("  %s\n", message)
	}
	if migrateTo != "" {
		fmt.Printf("  Migrate to:  %s\n", Bold(migrateTo))
	}
	if hint != "" {
		fmt.Printf("  %s\n", hint)
	}
	fmt.Println()
}

func crdModeLabel(crd types.CRDEntry) string {
	if crd.DefaultReconcile() {
		return "default"
	}
	if crd.CustomHooksEnabled() {
		return fmt.Sprintf("hooks(%s)", crd.Box().Reconcile.Hooks.Function)
	}
	if crd.ConstructorEnabled() {
		return fmt.Sprintf("constructor(%s)", crd.Box().Reconcile.ConstructorDecl.Function)
	}
	if crd.Mode != "" {
		return string(crd.Mode)
	}
	return "default"
}

// ValidateCRDFile checks that path is a valid YAML file with the required
// CustomResourceDefinition fields: apiVersion, kind, spec.group, spec.names.kind.
func ValidateCRDFile(path string) error {
	data, err := ReadLocal(path)
	if err != nil {
		return err
	}
	var crd struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Spec       struct {
			Group string `yaml:"group"`
			Names struct {
				Kind string `yaml:"kind"`
			} `yaml:"names"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	if crd.Kind != "CustomResourceDefinition" {
		return fmt.Errorf("kind must be CustomResourceDefinition, got %q", crd.Kind)
	}
	if crd.Spec.Group == "" {
		return fmt.Errorf("spec.group is required")
	}
	if crd.Spec.Names.Kind == "" {
		return fmt.Errorf("spec.names.kind is required")
	}
	return nil
}

// WriteOutput writes data to either a file, a directory, or stdout.
// If path is "-"  → prints to stdout.
// If path is ""   → writes <filename> in the current directory.
// If path is a directory → writes <path>/<filename>.
// If path is a file → writes directly to that file.
func WriteOutput(path, filename string, data []byte) error {
	if path == "-" {
		fmt.Println(string(data))
		return nil
	}

	var dest string
	if path == "" {
		dest = filename
	} else {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			dest = filepath.Join(path, filename)
		} else {
			dest = path
		}
	}

	log.Printf("%s generated successfully\n", filepath.Base(dest))
	return os.WriteFile(dest, data, 0644)
}

// PrintBanner prints the CLI's name line.
func PrintBanner() {
	fmt.Printf("\n%s\n\n", Bold("Inrun"))
}

// ExtractBinaryFromTarGz streams r (a .tar.gz) and writes the named binary entry to dst.
func ExtractBinaryFromTarGz(r io.Reader, binary string, dst io.Writer) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Match by base name so archives like "inrun/inrun" and flat "inrun" both work.
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == binary {
			if _, err := io.Copy(dst, tr); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("binary %q not found in archive", binary)
}

// CRKey is the key CRs are grouped and looked up by: the kind, lowercased.
func CRKey(kind string) string {
	return strings.ToLower(kind)
}
