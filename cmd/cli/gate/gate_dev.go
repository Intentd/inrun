//go:build !runtime && !gateway

package gate

// gate_dev.go — Local admission evaluator for dev builds.
//
// In a gateway build (//go:build gateway), inrun gate starts the full gateway
// server (TLS, webhooks, cluster-required). In dev builds there is no gateway
// process to start, but operators still need to validate admission rules before
// deploying. This command fills that gap:
//
//	inrun gate -f blueprint.yaml --cr cr.yaml
//
// It evaluates validation.rules and previews mutation.rules in-process, using
// the same EvaluateConditions + EvaluateValidationRule logic as the webhook and
// reconciler. No cluster connection, no TLS, no webhook server.
//
// Limitations vs. the real webhook:
//   - operator: unique  — skipped; no informer cache without a cluster.
//   - external: calls   — skipped (both validation and mutation); no endpoint.
//   Both produce a note in the output so the user knows they weren't checked.

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/internal"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/gateway/app"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
)

var gateRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the gateway locally (HTTP only, no TLS, no admission webhooks)",
	Long: `Start the Inrun Gateway in local HTTP mode.

The Gateway API (POST /api/v1/apply, GET /api/v1/resources/, intake webhooks)
runs on the health port (default :8080). Admission and conversion webhooks are
disabled — TLS and a live cluster are required for those.

Use this to test serve routing, apply flows, and intake payloads without a
Helm deployment.

Example:
  inrun gate run -f blueprint.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, _ := cmd.Flags().GetStringSlice("file")
		if len(paths) == 0 {
			paths = cmdutil.DefaultFilePaths()
		}
		if len(paths) == 0 {
			return fmt.Errorf(cmdutil.ErrNoBlueprint)
		}

		migrate, _ := cmd.Flags().GetBool("migrate")

		m := merger.New(paths...)
		if err := m.Merge(); err != nil {
			return fmt.Errorf("merging blueprints: %w", err)
		}

		internal.RunGateway(cmdutil.Ctx, cmdutil.Cfg, m, app.GatewayOptions{Migrate: migrate})
		return nil
	},
}

func init() {
	// Wire admission evaluator as the default action for bare `inrun gate`.
	gatewayCmd.RunE = gateEvalRunE
	gatewayCmd.Long = `Evaluate admission rules locally against a CR.

Reads the validation.rules declared in the Blueprint and runs them against the
provided CR using the same evaluation logic as the admission webhook and the
reconciler. No cluster, no TLS, no webhook server required.

Limitations:
  unique: true  — skipped (no live informer cache)
  external: calls   — skipped (no real endpoint to call)
Both are noted in the output.

Example:
  inrun gate -f blueprint.yaml --cr cr.yaml`

	gatewayCmd.AddCommand(gateRunCmd)
	gatewayCmd.Flags().String("cr", "", "CR file to evaluate (default: cr.yaml)")
	gateRunCmd.Flags().StringSliceP("file", "f", nil, "Path(s) to blueprint.yaml (repeatable)")
	gateRunCmd.Flags().Bool("migrate", false, "Run schema migration before starting the gateway")
	cmdutil.ShadowGlobalCommandFlags(gateRunCmd)
}

func gateEvalRunE(cmd *cobra.Command, args []string) error {
	crFile, _ := cmd.Flags().GetString("cr")
	if crFile == "" {
		crFile = cmdutil.FindFile(cmdutil.FileCr, cmdutil.DirManifests)
	}

	paths, _ := cmd.Flags().GetStringSlice("file")
	if len(paths) == 0 {
		paths = cmdutil.DefaultFilePaths()
	}
	if len(paths) == 0 {
		return fmt.Errorf(cmdutil.ErrNoBlueprint)
	}

	m := merger.New(paths...)
	if err := m.Merge(); err != nil {
		return fmt.Errorf("merging Blueprint: %w", err)
	}
	bp, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
	if err != nil {
		return fmt.Errorf("parsing Blueprint: %w", err)
	}

	crData, err := cmdutil.ReadLocal(crFile)
	if err != nil {
		return fmt.Errorf("reading CR %q: %w", crFile, err)
	}
	crs := cmdutil.ParseMultiDocCRs(crData)
	if len(crs) == 0 {
		return fmt.Errorf("no valid CR documents found in %s", crFile)
	}

	fmt.Println()
	fmt.Printf("%s  inrun gate\n", cmdutil.Bold("▶"))
	fmt.Printf("  %s %s\n", cmdutil.Gray("cr:     "), cmdutil.Cyan(crFile))
	fmt.Printf("  %s %s\n", cmdutil.Gray("blueprint:"), cmdutil.Cyan(strings.Join(paths, ", ")))
	fmt.Println()

	var anyDenied bool
	for _, name := range bp.CRDNames() {
		crdEntry, ok := bp.CRDEntry(name)
		if !ok {
			continue
		}
		in, ok := cmdutil.ResolveCRInputs(crs, crdEntry.APITypes.Kind)
		if !ok {
			continue
		}
		anyDenied = gateEvalCRD(bp, &crdEntry, in.CR.Object) || anyDenied
	}

	if anyDenied {
		return fmt.Errorf("admission denied")
	}
	return nil
}

// gateEvalCRD evaluates validation rules and previews mutation rules for one
// CRD+CR pair. Returns true if any deny-action validation rule fired.
func gateEvalCRD(bp *blueprint.Blueprint, crd *types.CRDEntry, obj map[string]interface{}) bool {
	fmt.Printf("%s  %s  %s\n", cmdutil.Cyan("◆"), cmdutil.Bold(crd.Kind()), cmdutil.Gray(fmt.Sprintf("(%s)", crd.ServeTarget())))

	if !crd.HasValidationRules() && !crd.HasMutationRules() {
		fmt.Printf("  %s no admission rules declared\n\n", cmdutil.Dim("note:"))
		return false
	}

	// Limitation notes — printed once for the CRD.
	hasUnique := crd.EffectiveValidation().HasUniqueRule()
	hasValidationExternal := crd.EffectiveValidation() != nil && len(crd.EffectiveValidation().AdmissionExternal()) > 0
	hasMutationExternal := crd.EffectiveMutation() != nil && len(crd.EffectiveMutation().AdmissionExternal()) > 0
	if hasUnique {
		fmt.Printf("  %s unique: true — skipped (no live cluster)\n", cmdutil.Dim("note:"))
	}
	if hasValidationExternal || hasMutationExternal {
		fmt.Printf("  %s external: calls — skipped (no endpoint)\n", cmdutil.Dim("note:"))
	}

	resolver := template.NewResolverFromMap(obj).WithUserFunctions(bp.Functions)
	eval := resolver.TemplateEvaluator()

	// When mutateFirst is set the real webhook applies mutations before
	// validation runs. Mirror that here so local results match cluster behaviour.
	validationObj := obj
	var mutResult AdmissionMutationResult
	if crd.ShouldMutateFirst() && crd.HasMutationRules() {
		mutResult = EvalAdmissionMutation(obj, crd, resolver, eval)
		if len(mutResult.Previews) > 0 {
			validationObj = applyMutationPreviews(obj, mutResult.Previews)
			validationResolver := template.NewResolverFromMap(validationObj).WithUserFunctions(bp.Functions)
			resolver = validationResolver
			eval = resolver.TemplateEvaluator()
		}
	}

	// ── Validation rules ──────────────────────────────────────────────────────
	denied := gateValidate(validationObj, crd, resolver, eval)

	// ── Mutation rules ────────────────────────────────────────────────────────
	// If we already evaluated mutations for mutateFirst, print those results
	// directly rather than re-evaluating against the original object.
	if crd.ShouldMutateFirst() && crd.HasMutationRules() {
		printGateMutateResult(mutResult)
	} else {
		gateMutate(obj, crd, resolver, eval)
	}

	fmt.Println()
	return denied
}

func gateValidate(obj map[string]interface{}, crd *types.CRDEntry, resolver *template.Resolver, eval types.TemplateEvaluator) bool {
	if !crd.HasValidationRules() {
		return false
	}
	r := EvalAdmissionValidation(obj, crd, resolver, eval)
	for _, v := range r.Violations {
		if v.Deny {
			fmt.Printf("  %s %s  %s\n", cmdutil.FailureMark(), cmdutil.Red(v.Field), cmdutil.Red(v.Message))
		} else {
			fmt.Printf("  %s %s  %s\n", cmdutil.Yellow("⚠"), cmdutil.Yellow(v.Field), cmdutil.Yellow(v.Message))
		}
	}
	denied, warned := r.Denied(), r.Warned()
	denialTxt, warningTxt := "denials", "warnings"
	if denied == 1 {
		denialTxt = "denial"
	}
	if warned == 1 {
		warningTxt = "warning"
	}
	if denied > 0 {
		fmt.Printf("\n  %s %d/%d validation rules passed · %s %d %s\n",
			cmdutil.FailureMark(), r.Passed, r.Total, cmdutil.Red("✗"), denied, cmdutil.Red(denialTxt))
		return true
	}
	if warned > 0 {
		fmt.Printf("  %s %d/%d validation rules passed · %s %d %s\n",
			cmdutil.SuccessMark(), r.Passed, r.Total, cmdutil.Yellow("⚠"), warned, cmdutil.Yellow(warningTxt))
		return false
	}
	fmt.Printf("  %s %d/%d validation rules passed\n", cmdutil.SuccessMark(), r.Passed, r.Total)
	return false
}

func gateMutate(obj map[string]interface{}, crd *types.CRDEntry, resolver *template.Resolver, eval types.TemplateEvaluator) {
	if !crd.HasMutationRules() {
		return
	}
	printGateMutateResult(EvalAdmissionMutation(obj, crd, resolver, eval))
}

func printGateMutateResult(r AdmissionMutationResult) {
	for _, p := range r.Previews {
		fromStr := cmdutil.Gray("(absent)")
		if p.Found {
			fromStr = cmdutil.Gray(p.From)
		}
		fmt.Printf("  %s %s  %s → %s  %s\n",
			cmdutil.Cyan("↳"), cmdutil.Cyan(p.Field), fromStr, cmdutil.Cyan(fmt.Sprintf("%v", p.To)), cmdutil.Gray(p.MutType))
	}
	mutTxt := "mutations"
	if len(r.Previews) == 1 {
		mutTxt = "mutation"
	}
	if len(r.Previews) > 0 {
		fmt.Printf("  %s %d %s would be applied\n", cmdutil.SuccessMark(), len(r.Previews), mutTxt)
	} else {
		fmt.Printf("  %s no mutations apply\n", cmdutil.Dim("note:"))
	}
}
