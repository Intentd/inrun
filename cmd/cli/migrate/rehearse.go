//go:build !runtime

package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/intentd/kome"
	"github.com/intentd/kome/declaration"
	"github.com/intentd/kome/rehearse"
)

var rehearseCmd = &cobra.Command{
	Use:   "rehearse",
	Short: "Run each migration from start to end in a throwaway cluster",
	Long: `Rehearse the Blueprint's migrations with their CRs, through KOME: apply the
schemas, seed the CRs, check migrate refuses without the transitional schema,
dry-run, migrate, edit one field and check rollback leaves it alone, migrate
again, apply the final schema, clean. A failing step stops its migration and
is named. What a CR would meet for real (a name collision, a destination that
drops fields) is a known issue: the CR is set aside, the rehearsal goes on,
and every issue is listed at the end.

The schemas and CRs come from migrate.from (crdFile, crFiles),
migrate.transitional.crdFile and migrate.to (crdFile, crFiles); the CRD's own
crdFile is the new schema at field level and the source at kind level. The
transitional schema is generated when not declared.

By default a kind cluster is created and deleted afterwards; --use-current or
--context rehearse in an existing cluster instead.

Example:
  inrun migrate rehearse -f blueprint.yaml
  inrun migrate rehearse -f blueprint.yaml --kind Website --keep
  inrun migrate rehearse -f blueprint.yaml --context kind-dev`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		f, err := migrateDeclaration(cmd)
		if err != nil {
			return err
		}
		ms, err := migrate.Migrations(f)
		if err != nil {
			return err
		}
		useCurrent, _ := cmd.Flags().GetBool("use-current")
		kubeContext, _ := cmd.Flags().GetString("context")
		keep, _ := cmd.Flags().GetBool("keep")
		output, _ := cmd.Flags().GetString("output")
		ctx := cmd.Context()
		kube, done, err := rehearse.Connect(ctx, rehearse.Where{Context: kubeContext, UseCurrent: useCurrent, Keep: keep, Log: cmd.ErrOrStderr()})
		if err != nil {
			return err
		}
		defer done()

		bp, err := cmdutil.BuildBlueprint(cmd)
		if err != nil {
			return err
		}
		r := &rehearse.Rehearsal{File: f, Engine: migrate.Engine(kube, bp.UserFunctions()), Cluster: rehearse.KubeCluster{Kube: kube}}
		if output != "json" {
			r.Step = func(s rehearse.Step) { fmt.Fprintln(cmd.OutOrStdout(), s) }
		}
		rep, err := r.Run(ctx, ms)
		if output == "json" {
			if jerr := rehearse.WriteJSON(cmd.OutOrStdout(), rep); jerr != nil {
				return jerr
			}
		} else {
			rehearse.WriteSummary(cmd.OutOrStdout(), rep)
		}
		return err
	},
}

var transitionalCmd = &cobra.Command{
	Use:   "transitional",
	Short: "Write each field migration's transitional schema",
	Long: `Write the CRD a field migration runs under: the CRD's crdFile, plus every
migrate.from path it lacks (types from migrate.from.crdFile), with the new
paths optional. Apply it before migrating, and the final CRD after.

Example:
  inrun migrate transitional -f blueprint.yaml
  inrun migrate transitional -f blueprint.yaml -o ./schemas`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		f, err := migrateDeclaration(cmd)
		if err != nil {
			return err
		}
		dir, _ := cmd.Flags().GetString("output")
		for _, name := range f.Names() {
			d := f.Migrations[name]
			if d.Level() != declaration.LevelField || d.Transitional != nil {
				continue
			}
			out, err := kome.Transitional(f, name)
			if err != nil {
				return err
			}
			if dir == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "# %s\n---\n%s", name, out)
				continue
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			path := filepath.Join(dir, name+"-transitional.yaml")
			if err := os.WriteFile(path, out, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
		}
		return nil
	},
}

// migrateDeclaration loads the Blueprint, turns its migrations into a KOME
// declaration, and checks it offline; findings are printed, errors fail.
func migrateDeclaration(cmd *cobra.Command) (*declaration.File, error) {
	bp, err := cmdutil.BuildBlueprint(cmd)
	if err != nil {
		return nil, err
	}
	kind, _ := cmd.Flags().GetString("kind")
	f, err := migrate.Declaration(bp, kind)
	if err != nil {
		return nil, err
	}
	rep := f.Validate()
	for _, finding := range rep.Findings {
		mark := "error"
		if finding.Warning {
			mark = "warning"
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", mark, finding)
	}
	if err := rep.Err(); err != nil {
		return nil, fmt.Errorf("the migrations have errors")
	}
	return f, nil
}

func init() {
	for _, c := range []*cobra.Command{rehearseCmd, transitionalCmd} {
		c.Flags().StringP("kind", "k", "", "only the CRD with this kind")
		migrateCmd.AddCommand(c)
	}
	rehearseCmd.Flags().StringP("output", "o", "text", "text or json")
	rehearseCmd.Flags().Bool("use-current", false, "rehearse in the current kubeconfig context; no cluster is created or deleted")
	rehearseCmd.Flags().StringP("context", "c", "", "rehearse in the given kubeconfig context; no cluster is created or deleted")
	rehearseCmd.Flags().Bool("keep", false, "keep the kind cluster rehearse created")
	transitionalCmd.Flags().StringP("output", "o", "", "write the files to this directory (default: print them)")
}
