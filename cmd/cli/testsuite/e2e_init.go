//go:build !runtime && !gateway

package testsuite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry/e2e"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/fileio"
	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

var e2eInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold an e2e.yaml from the current Blueprint",
	Long: `Reads the Blueprint in the current directory and generates a best-practice
e2e.yaml skeleton: CR created → resources created → cleanup verified.
No cluster is needed — it reads only the Blueprint for CRD names and kinds.

  inrun e2e init                         # auto-detect blueprint.yaml
  inrun e2e init -f blueprint.yaml         # explicit
  inrun e2e init --force                 # overwrite existing e2e.yaml
  inrun e2e init --dry-run               # preview without writing
  inrun e2e init --suite                 # aggregate all e2e.yaml files under .
  inrun e2e init --suite ./examples/     # aggregate under a specific dir`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if suite, _ := cmd.Flags().GetBool("suite"); suite {
			return e2eInitSuite(cmd, args)
		}
		return e2eInitScaffold(cmd)
	},
}

// crdInfo holds the name and kind of a CRD entry, used when scaffolding e2e.yaml.
type crdInfo struct {
	name string
	kind string
}

// e2eInitScaffold reads the Blueprint and writes an e2e.yaml scaffold.
func e2eInitScaffold(cmd *cobra.Command) error {
	blueprintFile, _ := cmd.Flags().GetString("file")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	var err error
	blueprintFile, err = cmdutil.ResolveBlueprintFile(blueprintFile)
	if err != nil {
		return err
	}

	bp, err := blueprint.ParseFile(blueprintFile)
	if err != nil {
		return fmt.Errorf("parsing Blueprint: %w", err)
	}

	outPath := filepath.Join(cmdutil.DirTest, cmdutil.FileE2e)
	if !dryRun && !force {
		if cmdutil.FileExists(outPath) {
			return fmt.Errorf("%s already exists — use --force to overwrite", outPath)
		}
	}

	// Paths in the spec are relative to the spec file, in test/.
	relBlueprint := fileio.RelFromDir(cmdutil.DirTest, blueprintFile)
	relCRD := fileio.RelFromDir(cmdutil.DirTest, projectFile(blueprintFile, cmdutil.FileCrd))
	relCR := fileio.RelFromDir(cmdutil.DirTest, projectFile(blueprintFile, cmdutil.FileCr))

	crdNames := bp.CRDNames()
	var crds []crdInfo
	for _, n := range crdNames {
		entry, ok := bp.CRDEntry(n)
		if !ok {
			continue
		}
		crds = append(crds, crdInfo{name: n, kind: entry.APITypes.Kind})
	}
	sort.Slice(crds, func(i, j int) bool { return crds[i].name < crds[j].name })

	output := buildE2EScaffoldYAML(bp.Metadata().Name, relBlueprint, relCRD, relCR, crds)

	if dryRun {
		fmt.Print(output)
		return nil
	}

	if err := os.MkdirAll(cmdutil.DirTest, 0755); err != nil {
		return fmt.Errorf("creating %s/: %w", cmdutil.DirTest, err)
	}
	if err := os.WriteFile(outPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	fmt.Printf("%s Generated %s\n", cmdutil.SuccessMark(), outPath)
	fmt.Printf("  %d CRD(s): %s\n", len(crds), crdKindList(crds))
	fmt.Printf("\n  Edit the placeholders, then run %s.\n", cmdutil.Bold("inrun e2e"))
	return nil
}

// e2eInitSuite discovers all e2e.yaml leaf files and writes a pure aggregator.
func e2eInitSuite(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	wait, _ := cmd.Flags().GetString("wait")
	skipRaw, _ := cmd.Flags().GetStringSlice("skip")

	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	var patterns []string
	for _, s := range skipRaw {
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				patterns = append(patterns, p)
			}
		}
	}

	paths, err := e2e.DiscoverE2EFiles(root, patterns)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	if len(paths) == 0 {
		fmt.Printf("No e2e files found under %s\n", root)
		return nil
	}

	outPath := cmdutil.FileE2e
	if !dryRun && !force {
		if cmdutil.FileExists(outPath) {
			return fmt.Errorf("%s already exists — use --force to overwrite", outPath)
		}
	}

	// Build relative import paths against CWD so the written file is portable.
	cwd, _ := os.Getwd()
	discovered := e2e.BuildDiscoveryE2E(paths, wait)
	type suiteImport struct {
		Path string `yaml:"path"`
		Wait string `yaml:"wait,omitempty"`
	}
	type suiteMeta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description,omitempty"`
	}
	// Pure aggregator — no spec, just imports. Use a local struct so the
	// empty E2ESpec fields are not serialized.
	type suiteDoc struct {
		APIVersion string        `yaml:"apiVersion"`
		Kind       string        `yaml:"kind"`
		Metadata   suiteMeta     `yaml:"metadata"`
		Imports    []suiteImport `yaml:"imports"`
	}
	doc := suiteDoc{
		APIVersion: "inrun.dev/v1",
		Kind:       "E2E",
		Metadata: suiteMeta{
			Name:        "suite",
			Description: fmt.Sprintf("Generated by inrun e2e init --suite — %d file(s) discovered", len(paths)),
		},
	}
	for _, imp := range discovered.Imports {
		rel, err := filepath.Rel(cwd, imp.Path)
		if err != nil {
			rel = imp.Path
		}
		doc.Imports = append(doc.Imports, suiteImport{Path: "./" + rel, Wait: imp.Wait})
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encoding suite: %w", err)
	}

	output := append([]byte(types.SchemaLine("E2E")+"\n"), buf.Bytes()...)

	if dryRun {
		fmt.Print(string(output))
		return nil
	}

	if err := os.WriteFile(outPath, output, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	absRoot, _ := filepath.Abs(root)
	fmt.Printf("%s Generated %s\n", cmdutil.SuccessMark(), outPath)
	fmt.Printf("  %d file(s) discovered under %s\n", len(paths), absRoot)
	const maxShow = 10
	for i, imp := range doc.Imports {
		if i >= maxShow {
			fmt.Printf("  ... %d more\n", len(doc.Imports)-maxShow)
			break
		}
		fmt.Printf("    %s\n", cmdutil.Dim(imp.Path))
	}
	fmt.Printf("\n  Run %s to verify.\n", cmdutil.Bold("inrun e2e"))
	return nil
}

// buildE2EScaffoldYAML returns the scaffold YAML string for inrun e2e init.
// Comments are injected directly — yaml.Marshal cannot preserve them.
func buildE2EScaffoldYAML(blueprintName, blueprintFile, crdFile, crFile string, crds []crdInfo) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s); b.WriteString("\n") }

	primaryKind := "<Kind>"
	if len(crds) > 0 {
		primaryKind = crds[0].kind
	}

	w(types.SchemaLine("E2E"))
	w("apiVersion: inrun.dev/v1")
	w("kind: E2E")
	w("metadata:")
	w("  name: " + blueprintName + "-e2e")
	w(`  description: "Generated by inrun e2e init — edit to refine"`)
	w("")
	w("spec:")
	w("  blueprint: " + blueprintFile)
	w("  crd: " + crdFile)
	w("  cr: " + crFile)
	w("")
	w("  cluster:")
	w("    provider: kind")
	w("    name: " + blueprintName + "-e2e")
	w("    reuse: false")

	if len(crds) > 1 {
		w("")
		w("  # Multiple CRDs in this Blueprint. Apply the remaining CRDs and CRs via setup.")
		w("  setup:")
		w("    apply:")
		w("      - ./other-crds.yaml   # CRD definitions for the remaining kinds")
		w("      - ./other-crs.yaml    # CR instances for the remaining kinds")
		w("      # Additional CRDs in this Blueprint:")
		for _, c := range crds[1:] {
			w("      # - " + c.kind)
		}
	}

	w("")
	w("  expect:")
	w("    - name: CR created")
	w("      after: cr-applied")
	w("      timeout: 60s")
	w("      resources:")
	w("        - kind: " + primaryKind)
	w("          name: <name>")
	w("          namespace: <namespace>")
	w("")
	w("    # - name: Resources created")
	w("    #   after: cr-applied")
	w("    #   timeout: 60s")
	w("    #   resources:")
	w("    #     - kind: Deployment")
	w("    #       name: <resource-name>")
	w("    #       namespace: <namespace>")
	w("    #       ready: true")
	w("")
	w("    - name: Cleanup verified")
	w("      after: cr-deleted")
	w("      timeout: 30s")
	w("      resources:")
	w("        - kind: " + primaryKind)
	w("          name: <name>")
	w("          namespace: <namespace>")
	w("          count: 0")
	w("        # - kind: Deployment")
	w("        #   name: <resource-name>")
	w("        #   namespace: <namespace>")
	w("        #   count: 0")

	if len(crds) > 1 {
		w("      # Add cleanup assertions for other CRDs as needed.")
	}

	return b.String()
}

// crdKindList returns a comma-separated list of CRD kind names for output.
func crdKindList(crds []crdInfo) string {
	kinds := make([]string, 0, len(crds))
	for _, c := range crds {
		kinds = append(kinds, c.kind)
	}
	return strings.Join(kinds, ", ")
}
