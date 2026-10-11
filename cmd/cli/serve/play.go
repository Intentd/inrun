//go:build !runtime && !gateway

package serve

import (
	"context"
	"fmt"
	"github.com/intentd/inrun/cmd/cli/testsuite"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
)

// servePlayCmd runs the full gateway apply chain locally from a flat intent
// file — no cluster, no HTTP server. Useful for testing token permissions,
// field routing, provenance stamping, and response payload shaping before
// wiring up a real delivery surface.
var servePlayCmd = &cobra.Command{
	Use:   "play",
	Short: "Run a serve intent locally through the full gateway chain",
	Long: `Run a serve intent locally through the full gateway chain.

For create/update: reads a flat YAML or JSON intent file (default: intent.yaml
or intent.json in the current directory) and runs all five chain stages.

For get/list/delete: no intent file needed — pass --target, --namespace, and
(for get/delete) --name. Runs target resolution, token check, and shows the
response config that would apply.

The intent may be JSON or YAML, whatever the file is called; the gateway
accepts either as a request body too.

No cluster connection is required. No CR is applied or fetched.

Stages (create/update):
  1. Target resolution    — resolve target/alias from catalog
  2. Token check          — verify the named token can perform the operation
  3. CR construction      — build the full CR from field declarations
  4. Provenance           — stamp serve-target and serve-alias annotations
  5. Admission validation — evaluate validation.rules (deny/warn) against the built CR
  6. Response payload     — evaluate serve.config.response.payload expressions

Stages (get/list/delete):
  1. Target resolution  — resolve target/alias from catalog
  2. Token check        — verify the named token can perform the operation
  3. Response config    — show what the caller would receive

Example intent.yaml:
  target: apifixture
  name: my-payment-service
  workloadType: app
  team: platform
  environment: staging
  repoURL: https://github.com/myorg/payments
  productionApproval: JIRA-1234

Example intent.json:
  {
	"target":"apifixture",
	"name":"my-payment-service",
	"workloadType":"app",
	"team":"platform",
	"environment":"staging",
	"repoURL":"https://github.com/myorg/payments",
	"productionApproval":"JIRA-1234"
}
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		intentFile, _ := cmd.Flags().GetString("intent")
		tokenName, _ := cmd.Flags().GetString("token")
		targetOverride, _ := cmd.Flags().GetString("target")
		operation, _ := cmd.Flags().GetString("operation")
		namespace, _ := cmd.Flags().GetString("namespace")
		name, _ := cmd.Flags().GetString("name")
		simulateRaw, _ := cmd.Flags().GetString("simulate")
		simulateFlag := cmd.Flags().Changed("simulate")
		simulateConfig := simulateRaw
		if simulateConfig == "-" {
			simulateConfig = "" // op-print mode — no spec file
		}
		var simOpts testsuite.CliSimulateOptions
		simOpts.UseEnvtest, _ = cmd.Flags().GetBool("envtest")
		simOpts.K8sVersion, _ = cmd.Flags().GetString("k8s-version")

		if tokenName == "" {
			return fmt.Errorf("%s --token is required", cmdutil.FailureMark())
		}

		k, err := cmdutil.BuildCatalog(cmd)
		if err != nil {
			return err
		}

		op := types.ServeOperation(operation)

		if simulateFlag && op != types.ServeOpCreate && op != types.ServeOpUpdate {
			return fmt.Errorf("%s --simulate is only valid for create and update operations", cmdutil.FailureMark())
		}

		if op == types.ServeOpCreate || op == types.ServeOpUpdate {
			// Resolve first catalog path for simulate handoff (--file is a StringSlice on serve commands)
			catalogFile := ""
			paths, _ := cmd.Flags().GetStringSlice("file")
			if len(paths) == 0 {
				paths = cmdutil.DefaultFilePaths()
			}
			if len(paths) == 0 {
				return fmt.Errorf(cmdutil.ErrNoCatalog)
			}
			catalogFile = paths[0]

			return playWrite(cmd.Context(), k, catalogFile, intentFile, targetOverride, tokenName, op, simulateFlag, simulateConfig, simOpts)
		}
		return playRead(k, targetOverride, tokenName, op, namespace, name)
	},
}

// ── create / update path ─────────────────────────────────────────────────────

func playWrite(ctx context.Context, k *catalog.Catalog, catalogFile, intentFile, targetOverride, tokenName string, op types.ServeOperation, doSimulate bool, simulateConfig string, simOpts testsuite.CliSimulateOptions) error {
	if intentFile == "" {
		intentFile = resolveDefaultIntentFile()
		if intentFile == "" {
			return fmt.Errorf("%s no intent file found — create intent.yaml or intent.json, or pass --intent <file>", cmdutil.FailureMark())
		}
	}

	raw, err := readIntentFile(intentFile)
	if err != nil {
		return fmt.Errorf("%s reading intent file %q: %w", cmdutil.FailureMark(), intentFile, err)
	}
	if targetOverride != "" {
		raw["target"] = targetOverride
	}

	target, _ := raw["target"].(string)
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf(`%s "target" must be set in the intent file or via --target`, cmdutil.FailureMark())
	}

	printPlayHeader(intentFile, target, tokenName, op)

	obj, crd, alias, err := planChain(k, raw, tokenName, op, stageNarrator{})
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("%s Intent would be accepted\n", cmdutil.SuccessMark())
	fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("POST /api/v1/apply  →  %s/%s in %s", crd.Kind(), obj.GetName(), obj.GetNamespace())))
	if alias != "" {
		fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("surface: %s (alias of %s)", alias, crd.ServeTarget())))
	}
	fmt.Println()

	if doSimulate {
		return playRunSimulate(ctx, catalogFile, obj, simulateConfig, simOpts)
	}
	return nil
}

// ── get / list / delete path ─────────────────────────────────────────────────

func playRead(k *catalog.Catalog, target, tokenName string, op types.ServeOperation, namespace, name string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("%s --target is required for %s", cmdutil.FailureMark(), op)
	}
	if op != types.ServeOpList && strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s --name is required for %s", cmdutil.FailureMark(), op)
	}

	printPlayHeaderRead(target, tokenName, op, namespace, name)

	// Stage 1: Target resolution
	printStage(1, "Target resolution")
	resolution := k.LookupByTargetOrAlias(target)
	if resolution == nil {
		printStageError(fmt.Sprintf("unknown target %q\n  available: %s", target, strings.Join(k.AvailableTargets(), ", ")))
		return fmt.Errorf("target %q not found", target)
	}
	crd, alias := resolution.CRD, resolution.Alias
	printStageOK(fmt.Sprintf("kind=%s  target=%s  alias=%s", cmdutil.Bold(crd.Kind()), cmdutil.Green(crd.ServeTarget()), aliasOrNone(alias)))

	// Stage 2: Token check
	printStage(2, "Token check")
	if err := checkToken(k, crd, alias, tokenName, op, namespace); err != nil {
		printStageError(err.Error())
		return err
	}
	printStageOK(fmt.Sprintf("token %s can %s on %s", cmdutil.Bold(tokenName), op, cmdutil.Bold(crd.Kind())))

	// Stage 3: Response config
	printStage(3, "Response config")
	if op == types.ServeOpDelete {
		printStageDetail(cmdutil.Gray(fmt.Sprintf("DELETE %s/%s in %s", crd.Kind(), name, namespaceOrAny(namespace))))
		printStageOK("would be deleted")
	} else {
		cfg := crd.ServeResponseConfigFor(alias)
		if cfg == nil {
			printStageDetail(cmdutil.Gray("no serve.config.response declared — full CR returned"))
		} else {
			if !cfg.UseDefault() {
				printStageDetail(cmdutil.Gray("default: false — raw CR omitted from response"))
			} else {
				printStageDetail(cmdutil.Gray("default: true — full CR included in response"))
			}
			if cfg.HasPayload() {
				printStageDetail(cmdutil.Gray(fmt.Sprintf("payload fields: %s", strings.Join(cmdutil.SortedKeys(cfg.Payload), ", "))))
			}
			if len(cfg.Exclude) > 0 {
				printStageDetail(cmdutil.Gray(fmt.Sprintf("excluded paths: %s", strings.Join(cfg.Exclude, ", "))))
			}
		}
		if op == types.ServeOpGet {
			printStageOK(fmt.Sprintf("GET %s/%s in %s — response config above applies", crd.Kind(), name, namespaceOrAny(namespace)))
		} else {
			printStageOK(fmt.Sprintf("LIST %s in %s — response config applies to all items", crd.Kind(), namespaceOrAny(namespace)))
		}
	}

	fmt.Println()
	fmt.Printf("%s Request would be allowed\n", cmdutil.SuccessMark())
	fmt.Println()
	return nil
}

func init() {
	servePlayCmd.Flags().StringP("intent", "i", "", "Intent file to play (YAML or JSON; default: intent.yaml or intent.json in cwd)")
	servePlayCmd.Flags().StringP("token", "t", "", "Token name to authenticate with")
	servePlayCmd.Flags().StringP("target", "T", "", "Target or alias name (required for get/list/delete; overrides intent file for create/update)")
	servePlayCmd.Flags().StringP("operation", "o", string(types.ServeOpCreate), "Operation to simulate ("+validServeOperations+")")
	servePlayCmd.Flags().StringP("namespace", "N", "", "Namespace (for get/list/delete)")
	servePlayCmd.Flags().StringP("name", "n", "", "Resource name (for get/delete)")
	servePlayCmd.Flags().StringP("simulate", "S", "", "After play, hand the built CR to inrun simulate; pass a simulate.yaml path to use assert mode")
	servePlayCmd.Flags().Lookup("simulate").NoOptDefVal = "-"
	servePlayCmd.Flags().Bool("envtest", false, "With --simulate, run against a local kube-apiserver + etcd instead of fake clients")
	servePlayCmd.Flags().String("k8s-version", simulate.DefaultEnvtestK8sVersion, "Kubernetes version for envtest binaries; only used with --envtest")

	_ = servePlayCmd.MarkFlagRequired("token")

	serveCmd.AddCommand(servePlayCmd)
}
