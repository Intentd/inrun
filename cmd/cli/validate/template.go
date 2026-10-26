//go:build !runtime && !gateway

package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// templateRuntimeOutput is the serializable view of a fully-expanded Blueprint.
// It holds what the runtime receives after module expansion and validation.
type templateRuntimeOutput struct {
	APIVersion string             `yaml:"apiVersion" json:"apiVersion"`
	Kind       string             `yaml:"kind"       json:"kind"`
	Metadata   map[string]string  `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Spec       templateSpecOutput `yaml:"spec"       json:"spec"`
}

type templateSpecOutput struct {
	CRDs map[string]types.CRDEntry `yaml:"crds" json:"crds"`
}

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Render and inspect the fully-expanded runtime Blueprint",
	Long: `Loads one or more Blueprint or Stack files, fully expands all module imports,
resolves all template inputs, validates the configuration, and shows exactly
what the runtime will see — nothing more, nothing less.

By default prints a human-readable summary. Use flags to switch output format
or drill into a specific CRD.

Examples:
  inrun template -f blueprint.yaml
  inrun template -f blueprint.yaml --yaml
  inrun template -f blueprint.yaml --json
  inrun template -f blueprint.yaml --graph
  inrun template -f blueprint.yaml --crd pipeline
  inrun template -f blueprint.yaml --yaml -o runtime.yaml
  inrun template -f a.yaml -f b.yaml --graph`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		yamlOut, _ := cmd.Flags().GetBool("yaml")
		graphOut, _ := cmd.Flags().GetBool("graph")
		crdName, _ := cmd.Flags().GetString("crd")
		outFile, _ := cmd.Flags().GetString("output")
		noValidate, _ := cmd.Flags().GetBool("no-validate")

		// ── Load & expand ───────────────────────────────────────────────────────
		spin := cmdutil.StartSpinner("Resolving imports...")
		merged, err := cmdutil.GenerateBlueprint(cmd)
		if err != nil {
			spin.Failure()
			return err
		}
		spin.Stop()

		var bp *blueprint.Blueprint
		if noValidate {
			var raw blueprint.Blueprint
			if _, err = raw.BuildRuntimeBlueprint(cmdutil.Cfg, merged.Merger); err != nil {
				return err
			}
			bp = &raw
		} else {
			bp, err = pipeline.BuildExpanded(cmdutil.Cfg, merged.Merger)
			if err != nil {
				return fmt.Errorf("validation: %w", err)
			}
		}

		crds := bp.Enabled()
		depGraph := blueprint.NewDependencyGraph(bp)
		startupOrder := depGraph.StartupOrder()

		// ── Route to output mode ────────────────────────────────────────────────
		var out []byte
		var defaultOut bool
		if !yamlOut && !jsonOut && !graphOut {
			defaultOut = true
		}

		switch {

		// ── --crd <name>: drill into one CRD ───────────────────────────────────
		case crdName != "":
			crd, ok := crds[crdName]
			if !ok {
				available := make([]string, 0, len(crds))
				for n := range crds {
					available = append(available, n)
				}
				sort.Strings(available)
				return fmt.Errorf("CRD %q not found — available: %v", crdName, available)
			}
			if jsonOut {
				out, err = json.MarshalIndent(crd, "", "  ")
			} else if yamlOut || defaultOut {
				out, err = yaml.Marshal(crd)
			} else {
				cmdutil.PrintCRDDetail(crd, depGraph)
				return nil
			}

		// ── --graph: dependency tree ────────────────────────────────────────────
		case graphOut:
			cmdutil.PrintDependencyGraph(crds, depGraph, startupOrder)
			return nil

		// ── --yaml / --json: full runtime Blueprint ───────────────────────────────
		case yamlOut || jsonOut || defaultOut:
			view := buildRuntimeView(bp)
			if jsonOut {
				out, err = json.MarshalIndent(view, "", "  ")
			} else {
				pruned, err := pruneYamlFile(view)
				if err != nil {
					return err
				}
				out = append([]byte("---\n"), pruned...)
			}

		// ── default: human-readable summary ────────────────────────────────────
		default:
			cmdutil.PrintTemplateSummary(bp, crds, startupOrder)
			return nil
		}

		if err != nil {
			return fmt.Errorf("render: %w", err)
		}

		if outFile != "" {
			if yamlOut || defaultOut {
				if err := cmdutil.WriteFileAndFormat(outFile, out, 0644); err != nil {
					return fmt.Errorf("writing %s: %w", outFile, err)
				}
			} else {
				if err := os.WriteFile(outFile, out, 0644); err != nil {
					return fmt.Errorf("writing %s: %w", outFile, err)
				}
			}
			fmt.Printf("%s\n", cmdutil.Green("written to "+outFile))
			return nil
		}
		fmt.Println(string(out))
		return nil
	},
}

// buildRuntimeView constructs the serializable runtime view from the expanded Blueprint.
func buildRuntimeView(bp *blueprint.Blueprint) templateRuntimeOutput {
	meta := bp.Metadata()
	metaFields := map[string]string{}
	if meta.Name != "" {
		metaFields["name"] = meta.Name
	}
	if meta.Version != "" {
		metaFields["version"] = meta.Version
	}
	if meta.Author != "" {
		metaFields["author"] = meta.Author
	}
	if meta.Description != "" {
		metaFields["description"] = meta.Description
	}
	if len(metaFields) == 0 {
		metaFields = nil
	}

	return templateRuntimeOutput{
		APIVersion: bp.APIVersion,
		Kind:       bp.Kind,
		Metadata:   metaFields,
		Spec: templateSpecOutput{
			CRDs: bp.Enabled(),
		},
	}
}

func pruneYamlFile(view templateRuntimeOutput) ([]byte, error) {
	raw, merr := yaml.Marshal(view)
	if merr != nil {
		return nil, fmt.Errorf("marshal: %w", merr)
	}
	pruned, perr := cmdutil.PruneEmptyYAML(raw)
	if perr != nil {
		return nil, fmt.Errorf("prune: %w", perr)
	}
	return pruned, nil
}

func init() {
	cmdutil.RootCmd.AddCommand(templateCmd)

	templateCmd.Flags().BoolP("yaml", "y", false, "Output full expanded runtime Blueprint as YAML")
	templateCmd.Flags().BoolP("json", "j", false, "Output full expanded runtime Blueprint as JSON")
	templateCmd.Flags().BoolP("graph", "g", false, "Show dependency graph with startup order")
	templateCmd.Flags().StringP("crd", "c", "", "Drill into a specific CRD by name")
	templateCmd.Flags().StringP("output", "o", "", "Write output to file instead of stdout")
	templateCmd.Flags().Bool("no-validate", false, "Skip validation (show expanded state even if invalid)")

	// Shadow global flags so they don't appear under `inrun template`
	cmdutil.ShadowGlobalCommandFlags(templateCmd)
}
