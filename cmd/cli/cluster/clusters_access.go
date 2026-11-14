//go:build !runtime && !gateway

package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/tools/inruncluster"
	"github.com/intentd/tools/cluster/access"
	"github.com/intentd/tools/term"
)

var clustersAccessCmd = &cobra.Command{
	Use:   "access",
	Short: "Give the gateway least-privilege access to target clusters, or revoke it",
	Long: `Create the ServiceAccount, ClusterRole and token on a target cluster, then
store the credential in the gateway cluster (the current context) so the
gateway can route applies to it.

With --context, one cluster is given access, and the ClusterRole is scoped to
exactly the serve-enabled CRDs declared in the blueprint: no wildcard
resources, no cluster-admin. With --config, every cluster in the file is
granted (or revoked, with revoke: true) using the rules the file declares.

Revoking removes the ServiceAccount, its roles and token on the target and
the credential on the gateway cluster. It asks first; --yes skips the
question.

Examples:
  inrun clusters access --context kind-prod --name prod
  inrun clusters access --context kind-prod --name prod --token short --ttl 24h
  inrun clusters access --context kind-prod --name prod --dry-run
  inrun clusters access --context kind-prod --name prod --emit-rbac
  inrun clusters access --context kind-prod --name prod --revoke
  inrun clusters access --config access.yaml
  inrun clusters access --validate access.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if path, _ := cmd.Flags().GetString("validate"); path != "" {
			_, err := loadAccessConfig(path)
			return err
		}
		f := cmd.Flags()
		hubNS, _ := f.GetString("namespace")
		dryRun, _ := f.GetBool("dry-run")
		emitRBAC, _ := f.GetBool("emit-rbac")
		yes, _ := f.GetBool("yes")
		opts := access.Options{HubNamespace: hubNS, DryRun: dryRun, EmitRBACOnly: emitRBAC}

		if path, _ := f.GetString("config"); path != "" {
			cfg, err := loadAccessConfig(path)
			if err != nil {
				return err
			}
			out, _ := f.GetString("out")
			return runAccess(cmd.Context(), nil, cfg, opts, yes, out)
		}

		name, _ := f.GetString("name")
		kubectx, _ := f.GetString("context")
		saNS, _ := f.GetString("sa-namespace")
		token, _ := f.GetString("token")
		ttl, _ := f.GetString("ttl")
		revoke, _ := f.GetBool("revoke")
		cl := access.Cluster{Context: kubectx, Revoke: revoke, Settings: access.Settings{
			Namespace: saNS,
			Token:     access.Token{Type: access.TokenType(token), TTL: ttl},
		}}
		if f.Changed("rotate") {
			rotate, _ := f.GetBool("rotate")
			cl.Token.Rotate = &rotate
		}
		cfg := &access.Config{Clusters: map[string]access.Cluster{name: cl}}
		if err := cfg.Validate(inruncluster.Naming); err != nil {
			return fmt.Errorf("invalid cluster:\n%w", err)
		}
		bp, err := cmdutil.BuildBlueprint(cmd)
		if err != nil {
			return err
		}
		return runAccess(cmd.Context(), bp, cfg, opts, yes, "")
	},
}

// loadAccessConfig reads and validates an access config, applying Inrun's
// default token, and prints what it would do.
func loadAccessConfig(path string) (*access.Config, error) {
	cfg, err := access.LoadConfig(path)
	if err != nil {
		return nil, err
	}
	if cfg.Defaults.Token.Type == "" {
		cfg.Defaults.Token.Type = inruncluster.DefaultToken
	}
	if err := cfg.Validate(inruncluster.Naming); err != nil {
		return nil, fmt.Errorf("invalid access config:\n%w", err)
	}
	fmt.Printf("%s access config valid (%d cluster(s))\n", cmdutil.SuccessMark(), len(cfg.Clusters))
	for _, name := range cfg.Names() {
		cl, e := cfg.Clusters[name], cfg.Effective(name, inruncluster.Naming)
		action := fmt.Sprintf("grant (%s token)", e.Token)
		if cl.Revoke {
			action = cmdutil.Yellow("revoke")
		}
		fmt.Printf("  %-12s →  %s   %s\n", name, cl.Context, action)
	}
	return cfg, nil
}

// runAccess grants or revokes every cluster in cfg. Revoking asks first
// unless yes. With a Blueprint the rules are the gateway's; with out, the
// granted credentials are written there in gateway.clusters format.
func runAccess(ctx context.Context, bp *blueprint.Blueprint, cfg *access.Config, opts access.Options, yes bool, out string) error {
	if cfg.Defaults.Token.Type == "" {
		cfg.Defaults.Token.Type = inruncluster.DefaultToken
	}
	opts = inruncluster.GatewayAccess(bp, opts)
	var revoking []string
	for _, name := range cfg.Names() {
		if cfg.Clusters[name].Revoke {
			revoking = append(revoking, name)
		}
	}
	if len(revoking) > 0 && !opts.DryRun && !opts.EmitRBACOnly && !yes {
		ok, err := term.Confirm(fmt.Sprintf("Revoke the gateway's access to %v? This deletes its ServiceAccount, roles and token there", revoking))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("revoke cancelled")
		}
	}

	var granted []*access.Result
	for _, name := range cfg.Names() {
		cl := cfg.Clusters[name]
		fmt.Println()
		fmt.Printf("%s  inrun clusters access\n", cmdutil.Bold("⎈"))
		fmt.Printf("  %s cluster name:   %s\n", cmdutil.Gray("→"), cmdutil.Bold(name))
		fmt.Printf("  %s target context: %s\n", cmdutil.Gray("→"), cmdutil.Bold(cl.Context))
		fmt.Printf("  %s namespace:      %s\n", cmdutil.Gray("→"), cmdutil.Gray(opts.HubNamespace))
		if opts.DryRun {
			fmt.Printf("  %s\n", cmdutil.Yellow("dry-run: no changes will be made"))
		}
		fmt.Println()

		log := func(msg string) { fmt.Printf("   %s %s\n", cmdutil.SuccessMark(), msg) }
		var r *access.Result
		var err error
		if cl.Revoke {
			r, err = access.Revoke(ctx, cfg, name, opts, log)
		} else {
			r, err = access.Grant(ctx, cfg, name, opts, log)
		}
		if err != nil {
			return fmt.Errorf("%s cluster %s: %s", cmdutil.FailureMark(), name, err.Error())
		}

		switch {
		case opts.EmitRBACOnly:
			if err := printClusterRole(r); err != nil {
				return err
			}
		case opts.DryRun:
			fmt.Printf("  %s dry-run complete: no changes were made\n", cmdutil.Gray("○"))
		case r.Revoked:
			fmt.Printf("  %s access to %s revoked; remove it from gateway.clusters\n", cmdutil.SuccessMark(), name)
		default:
			if !r.ExpiresAt.IsZero() {
				fmt.Printf("  %s short token, expires %s\n", cmdutil.Gray("○"), r.ExpiresAt.UTC().Format(time.RFC3339))
			}
			fmt.Println()
			printBlueprintSnippet(r)
			granted = append(granted, r)
		}
	}

	if out != "" && len(granted) > 0 {
		if err := inruncluster.WriteClusterCredentials(out, granted); err != nil {
			return err
		}
		fmt.Printf("\n%s cluster credentials → %s\n", cmdutil.SuccessMark(), out)
		fmt.Printf("  %s inrun clusters check --config %s\n", cmdutil.Gray("→"), out)
	}
	return nil
}

// ── output helpers ────────────────────────────────────────────────────────────

func printBlueprintSnippet(r *access.Result) {
	fmt.Printf("%s  Add to your blueprint:\n\n", cmdutil.Bold("⎈"))
	b, err := yaml.Marshal(map[string]any{
		"gateway": map[string]any{"clusters": map[string]any{r.Name: inruncluster.GatewayClusterConfig(r)}},
	})
	if err != nil {
		return
	}
	fmt.Println(string(b))
}

func printClusterRole(r *access.Result) error {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: r.Settings.ServiceAccount, Labels: inruncluster.Naming.Labels},
		Rules:      r.Settings.Rules,
	}
	b, err := yaml.Marshal(cr)
	if err != nil {
		return fmt.Errorf("marshaling ClusterRole: %w", err)
	}
	fmt.Printf("# ── ClusterRole (%s) ──────────────────────────────────────────────\n", r.Settings.ServiceAccount)
	fmt.Printf("---\n%s\n", string(b))
	return nil
}

func init() {
	f := clustersAccessCmd.Flags()
	f.String("context", "", "kubectl context for the TARGET cluster")
	f.String("name", "", "name for this cluster in gateway.clusters")
	f.String("namespace", "default", "namespace in the gateway cluster for the credential Secret")
	f.String("sa-namespace", access.DefaultSANamespace, "namespace on the TARGET cluster for the ServiceAccount")
	f.String("token", string(inruncluster.DefaultToken), "token type: short (expires, renewed) or long (does not expire)")
	f.String("ttl", "", "short token lifetime, e.g. 24h (default 24h)")
	f.Bool("rotate", false, "issue a new long token instead of reusing the current one")
	f.Bool("revoke", false, "remove the access instead of granting it")
	f.Bool("yes", false, "revoke without asking")
	f.String("config", "", "path to an access config to grant or revoke several clusters")
	f.StringP("out", "o", "", "write granted credentials to this file (gateway.clusters format; use with check --config)")
	f.String("validate", "", "validate an access config without connecting to any cluster")
	f.Bool("dry-run", false, "print what would be done without making changes")
	f.Bool("emit-rbac", false, "print only the ClusterRole YAML for review")

	clustersCmd.AddCommand(clustersAccessCmd)
}
