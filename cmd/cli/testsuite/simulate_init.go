//go:build !runtime && !gateway

package testsuite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ── inrun simulate init ──────────────────────────────────────────────────────────

type crdOps struct {
	name string
	ops  []simulate.Op
}

var simulateInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a simulate.yaml pre-filled with the observed cycle-1 ops",
	Long: `Runs the reconciler once and generates a simulate.yaml with the
observed cycle-1 create operations as expect: rules. Edit and refine from there.

  inrun simulate init
  inrun simulate init -f blueprint.yaml --cr cr.yaml
  inrun simulate init --force              # overwrite existing simulate.yaml
  inrun simulate init --suite              # aggregate all simulate.yaml files under .
  inrun simulate init --suite ./examples/  # aggregate under a specific dir`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if suite, _ := cmd.Flags().GetBool("suite"); suite {
			return simulateInitSuite(cmd, args)
		}

		blueprintFile, _ := cmd.Flags().GetString("file")
		crFile, _ := cmd.Flags().GetString("cr")
		force, _ := cmd.Flags().GetBool("force")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		var err error
		blueprintFile, err = cmdutil.ResolveBlueprintFile(blueprintFile)
		if err != nil {
			return err
		}
		if crFile == "" {
			crFile = projectFile(blueprintFile, cmdutil.FileCr)
		} else if abs, err := filepath.Abs(crFile); err == nil {
			crFile = abs
		}

		bp, err := blueprint.ParseFile(blueprintFile)
		if err != nil {
			return fmt.Errorf("parsing Blueprint: %w", err)
		}
		crData, err := cmdutil.ReadLocal(crFile)
		if err != nil {
			return fmt.Errorf("reading CR: %w", err)
		}
		crs, err := cmdutil.ParseMultiDocCRs(crData)
		if err != nil {
			return err
		}
		if len(crs) == 0 {
			return fmt.Errorf("no valid CR documents in %s", crFile)
		}

		outPath := filepath.Join(cmdutil.DirTest, cmdutil.FileSimulate)
		if !dryRun && !force {
			if cmdutil.FileExists(outPath) {
				return fmt.Errorf("%s already exists — use --force to overwrite", outPath)
			}
		}

		opts := simulate.RunOptions{}
		targets := bp.CRDNames()

		var results []crdOps

		for _, name := range targets {
			crdEntry, ok := bp.CRDEntry(name)
			if !ok {
				continue
			}
			in, ok := cmdutil.ResolveCRInputs(crs, crdEntry.APITypes.Kind)
			if !ok {
				continue
			}
			opts.Blueprint = bp
			opts.CRDName = name
			opts.CR = in.CR
			opts.MaxCycles = 10
			opts.Peers = in.Peers
			opts.ExistingInstances = in.Existing
			result, err := simulate.New(cmd.Context(), opts).Execute(simulate.InMemory)
			if err != nil && crdEntry.WithRemoteDecl() {
				fmt.Printf("  %s %s: remote reconciler unreachable — start it and re-run (%v)\n", cmdutil.Dim("note:"), name, err)
				continue
			}
			if err != nil {
				return fmt.Errorf("simulating %s: %w", name, err)
			}
			var cycle1Creates []simulate.Op
			for _, op := range result.AllOps {
				if op.Cycle == 1 && op.Verb == "create" && op.Resource != "namespaces" {
					cycle1Creates = append(cycle1Creates, op)
				}
			}
			results = append(results, crdOps{name: name, ops: cycle1Creates})
		}

		// Paths in the spec are relative to the spec file, so the file is portable.
		relBlueprint := fileio.RelFromDir(cmdutil.DirTest, blueprintFile)
		relCR := fileio.RelFromDir(cmdutil.DirTest, crFile)

		doc := generateSimulateDoc(relBlueprint, relCR, bp.Metadata().Name, results)

		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(doc); err != nil {
			return fmt.Errorf("encoding simulate.yaml: %w", err)
		}

		output := injectAbsentComment(buf.Bytes(), results)
		output = append([]byte(types.SchemaLine("Simulate")+"\n"), output...)

		if dryRun {
			fmt.Print(string(output))
			return nil
		}

		if err := os.MkdirAll(cmdutil.DirTest, 0755); err != nil {
			return fmt.Errorf("creating %s/: %w", cmdutil.DirTest, err)
		}
		if err := os.WriteFile(outPath, output, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		}

		fmt.Printf("%s Generated %s\n", cmdutil.SuccessMark(), outPath)
		fmt.Printf("  %d CRD(s), %d op rule(s)\n", len(results), countRules(results))
		fmt.Printf("\n  Run %s to verify.\n", cmdutil.Bold("inrun simulate"))
		return nil
	},
}

// simulateInitSuite discovers all simulate.yaml leaf files under root, builds a
// pure aggregator, and writes (or prints) simulate.yaml in the current directory.
func simulateInitSuite(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	skip, _ := cmd.Flags().GetStringSlice("skip")

	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	paths, err := simulate.DiscoverSimulateFiles(root, structured.SplitCommaList(skip))
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	if len(paths) == 0 {
		fmt.Printf("No simulate.yaml files found under %s\n", root)
		return nil
	}

	cwd, _ := os.Getwd()
	relPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, err := filepath.Rel(cwd, p)
		if err != nil {
			rel = p
		}
		relPaths = append(relPaths, "./"+rel)
	}

	outPath := cmdutil.FileSimulate
	if !dryRun && !force {
		if cmdutil.FileExists(outPath) {
			return fmt.Errorf("%s already exists — use --force to overwrite", outPath)
		}
	}

	type suiteMeta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description,omitempty"`
	}
	type suiteDoc struct {
		APIVersion string    `yaml:"apiVersion"`
		Kind       string    `yaml:"kind"`
		Metadata   suiteMeta `yaml:"metadata"`
		Imports    []string  `yaml:"imports"`
	}

	doc := suiteDoc{
		APIVersion: "inrun.dev/v1",
		Kind:       "Simulate",
		Metadata: suiteMeta{
			Name:        "suite",
			Description: fmt.Sprintf("Generated by inrun simulate init --suite — %d file(s) discovered", len(relPaths)),
		},
		Imports: relPaths,
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encoding suite: %w", err)
	}

	output := append([]byte(types.SchemaLine("Simulate")+"\n"), buf.Bytes()...)

	if dryRun {
		fmt.Print(string(output))
		return nil
	}

	if err := os.WriteFile(outPath, output, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	absRoot, _ := filepath.Abs(root)
	fmt.Printf("%s Generated %s\n", cmdutil.SuccessMark(), outPath)
	fmt.Printf("  %d file(s) discovered under %s\n", len(relPaths), absRoot)
	const maxShow = 10
	for i, p := range relPaths {
		if i >= maxShow {
			fmt.Printf("  ... %d more\n", len(relPaths)-maxShow)
			break
		}
		fmt.Printf("    %s\n", cmdutil.Dim(p))
	}
	fmt.Printf("\n  Run %s to verify.\n", cmdutil.Bold("inrun simulate"))
	return nil
}

// generateSimulateDoc builds a Simulate document from observed cycle-1 creates.
func generateSimulateDoc(blueprintPath, crPath, blueprintName string, results []crdOps) map[string]interface{} {
	trueVal := true

	makeOps := func(ops []simulate.Op) []map[string]interface{} {
		var rules []map[string]interface{}
		for _, op := range ops {
			rule := map[string]interface{}{
				"cycle":    1,
				"verb":     op.Verb,
				"resource": op.Resource,
			}
			if op.Name != "" {
				rule["name"] = op.Name
			}
			rules = append(rules, rule)
		}
		return rules
	}

	spec := map[string]interface{}{
		"blueprint": blueprintPath,
		"cr":        crPath,
		"cycles":    5,
	}

	if len(results) == 1 {
		expect := map[string]interface{}{
			"steady":   trueVal,
			"noErrors": trueVal,
		}
		if ops := makeOps(results[0].ops); len(ops) > 0 {
			expect["ops"] = ops
		}
		spec["expect"] = expect
	} else {
		crds := map[string]interface{}{}
		for _, r := range results {
			crdExpect := map[string]interface{}{
				"steady":   trueVal,
				"noErrors": trueVal,
			}
			if ops := makeOps(r.ops); len(ops) > 0 {
				crdExpect["ops"] = ops
			}
			crds[r.name] = crdExpect
		}
		spec["expect"] = map[string]interface{}{
			"noErrors": trueVal,
			"crds":     crds,
		}
	}

	return map[string]interface{}{
		"apiVersion": "inrun.dev/v1",
		"kind":       "Simulate",
		"metadata": map[string]interface{}{
			"name":        blueprintName + "-sim",
			"description": "Generated by inrun simulate init — edit to refine",
		},
		"spec": spec,
	}
}

func countRules(results []crdOps) int {
	n := 0
	for _, r := range results {
		n += len(r.ops)
	}
	return n
}

// injectAbsentComment parses the encoded YAML, adds a HeadComment on every
// "steady" key hinting at the absent: block, then re-encodes with 2-space indent.
func injectAbsentComment(data []byte, results []crdOps) []byte {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return data
	}

	resource := "deployments"
	if len(results) > 0 && len(results[0].ops) > 0 {
		resource = results[0].ops[0].Resource
	}

	comment := "# absent:   # ops that must NOT appear — fill in for failure-path coverage\n" +
		"#   - cycle: 1\n" +
		"#     verb: create\n" +
		"#     resource: " + resource

	addHeadComment(&root, "steady", comment)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return data
	}
	return buf.Bytes()
}

// addHeadComment recursively sets HeadComment on every mapping key matching name.
func addHeadComment(node *yaml.Node, name, comment string) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content)-1; i += 2 {
			if node.Content[i].Value == name {
				node.Content[i].HeadComment = comment
			}
		}
	}
	for _, child := range node.Content {
		addHeadComment(child, name, comment)
	}
}

// projectFile returns where name is in the Blueprint's project: next to it,
// else its place in the standard layout. Next to it is the fallback.
func projectFile(blueprintFile, name string) string {
	base := filepath.Dir(blueprintFile)
	if rel := registry.FindPatternFile(base, name); rel != "" {
		return filepath.Join(base, rel)
	}
	return filepath.Join(base, name)
}
