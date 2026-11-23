//go:build !runtime && !gateway

package run

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/tools/inruncluster"
	"github.com/intentd/tools/cluster"
	"github.com/intentd/tools/command"
	"github.com/intentd/tools/logger"
)

// applyPreRuntimeResources applies all pre-runtime resources declared in the
// merged CRD entries. This includes:
//
//  1. CRD files      (crdFile)
//  2. Waiting for CRDs to establish
//  3. CR files       (crFiles) — dev mode only
//  4. Setup files    (setup)
//
// All resources are applied in the correct order before the operator runtime
// starts. Relative paths are resolved against the blueprint directory. This
// function is a no-op when running inside the cluster.
func applyPreRuntimeResources(ctx context.Context, blueprintPath string, m *merger.Merger) {
	if cmdutil.IsRunningInPod() {
		return
	}

	// The built Blueprint holds what the merged entries do not: expanded serve
	// includes and resolved schema paths, so it knows each field migration's
	// transitional schema. One that does not build is the runtime's to report.
	bp, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
	if err != nil {
		bp = nil
	}

	applyCRDFilesIfNeeded(ctx, blueprintPath, m, bp)
	waitForCRDsEstablished(ctx, m)
	applyCRFilesIfNeeded(ctx, blueprintPath, m)
	applySetupIfNeeded(ctx, blueprintPath, m)
}

// ensureClusterReady ensures that a Kubernetes cluster is reachable before
// starting the operator. In dev mode, this installs missing dependencies and
// creates a local Kind cluster if needed. Outside dev mode, this reports
// missing dependencies and unreachable cluster state to the user.
func ensureClusterReady(dev bool) error {
	if dev {
		if err := cluster.EnsureDependencies(); err != nil {
			return fmt.Errorf("installing dependencies: %w", err)
		}
		if cluster.ClusterReachable() {
			return nil
		}

		fmt.Println("\n  Cannot reach a Kubernetes cluster.")
		fmt.Printf("  Creating local Kind cluster '%s'...\n", inruncluster.KindClusterName)

		if err := cluster.EnsureKind(cmdutil.Ctx, inruncluster.KindClusterName, inruncluster.Kind(0, "")); err != nil {
			return fmt.Errorf("setting up kind cluster: %w", err)
		}

		return nil
	}

	// non --dev mode
	if cluster.ClusterReachable() {
		return nil
	}

	fmt.Println("\n  Cannot reach a Kubernetes cluster.")
	fmt.Println("  Check your kubeconfig, or run with --dev to deploy to a local kind cluster.")

	var missing []string
	helm := cluster.HelmAvailable()
	kubectl := cluster.KubectlAvailable()

	if !kubectl {
		missing = append(missing, "kubectl")
	}
	if !helm {
		missing = append(missing, "helm")
	}

	if len(missing) > 0 {
		text := "these missing dependencies"
		if len(missing) == 1 {
			text = "this missing dependency"
		}

		fmt.Printf("  This will install %s:\n", text)
		for _, m := range missing {
			fmt.Printf("    • %s\n", m)
		}
		fmt.Println()
	}

	return fmt.Errorf("cluster not reachable")
}

// applyCRFilesIfNeeded applies crFiles declarations via kubectl in order before
// the runtime starts. Only runs outside the cluster (dev mode).
func applyCRFilesIfNeeded(ctx context.Context, blueprintPath string, m *merger.Merger) {
	if cmdutil.IsRunningInPod() {
		return
	}

	blueprintDir := filepath.Dir(blueprintPath)

	for crdName, entry := range m.All() {
		if !entry.HasCRFiles() {
			continue
		}
		for _, crFile := range entry.CRFiles {
			path := crFile
			if !filepath.IsAbs(path) && !strings.HasPrefix(path, "http") {
				path = filepath.Join(blueprintDir, path)
			}

			out, err := kubectl(ctx, "", "apply", "-f", path)
			if err != nil {
				logger.Warn().
					Str("crd", crdName).
					Str("path", path).
					Str("output", out).
					Err(err).
					Msg("crFile pre-apply failed (continuing)")
			} else {
				logger.Info().
					Str("crd", crdName).
					Str("path", path).
					Msg("crFile applied")
			}
		}
	}
}

// waitForCRDsEstablished waits for any CRDs that have crFiles to be Established
// in the cluster before CRs are applied. Only blocks for CRDs that need it.
func waitForCRDsEstablished(ctx context.Context, m *merger.Merger) {
	if cmdutil.IsRunningInPod() {
		return
	}

	for crdName, entry := range m.All() {
		if len(entry.CRFiles) == 0 {
			continue
		}

		plural := entry.APITypes.Plural
		group := entry.APITypes.Group
		if plural == "" || group == "" {
			continue
		}

		crdFullName := plural + "." + group

		out, err := kubectl(ctx, "", "wait",
			"--for=condition=Established",
			"--timeout=30s",
			"crd/"+crdFullName,
		)
		if err != nil {
			logger.Warn().
				Str("crd", crdName).
				Str("name", crdFullName).
				Str("output", out).
				Err(err).
				Msg("CRD not established in time — applying CRs anyway")
		} else {
			logger.Info().
				Str("crd", crdName).
				Str("name", crdFullName).
				Msg("CRD established")
		}
	}
}

// applyCRDFilesIfNeeded applies any crdFile declarations via kubectl before the
// operator starts. A field migration's transitional schema, generated by the
// Blueprint when none is declared, is applied in crdFile's place, so CRs keep
// their old values until the gateway migrates them. Only runs outside the
// cluster (dev mode). In production, CRDs must be pre-applied by the platform
// operator.
func applyCRDFilesIfNeeded(ctx context.Context, blueprintPath string, m *merger.Merger, bp *blueprint.Blueprint) {
	if cmdutil.IsRunningInPod() {
		return
	}

	blueprintDir := filepath.Dir(blueprintPath)

	for crdName, entry := range m.All() {
		if bp != nil {
			schema, ok, err := migrate.Transitional(bp, crdName)
			if err != nil {
				logger.Warn().Str("crd", crdName).Err(err).Msg("transitional CRD not generated — applying crdFile")
			} else if ok {
				applyCRDSchema(ctx, crdName, schema)
				continue
			}
		}
		path := entry.AppliedCRDFile()
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) && !strings.HasPrefix(path, "http") {
			path = filepath.Join(blueprintDir, path)
		}

		out, err := kubectl(ctx, "", "apply", "-f", path)
		if err != nil {
			logger.Warn().
				Str("crd", crdName).
				Str("path", path).
				Str("output", out).
				Err(err).
				Msg("crdFile pre-apply failed (continuing)")
		} else {
			logger.Info().
				Str("crd", crdName).
				Str("path", path).
				Msg("crdFile applied")
		}
	}
}

// applyCRDSchema applies a CRD held in memory via kubectl.
func applyCRDSchema(ctx context.Context, crdName string, schema []byte) {
	if out, err := kubectl(ctx, string(schema), "apply", "-f", "-"); err != nil {
		logger.Warn().
			Str("crd", crdName).
			Str("output", out).
			Err(err).
			Msg("transitional CRD pre-apply failed (continuing)")
		return
	}
	logger.Info().Str("crd", crdName).Msg("transitional CRD applied")
}

// applyPatternExamples applies crd.yaml and cr.yaml from the pattern directory
// when --apply-cr is set. These are the example files shipped with the pattern —
// distinct from crdFile/crFiles declared in the blueprint itself. kubectl apply
// is idempotent so overlap with applyPreRuntimeResources is safe.
func applyPatternExamples(ctx context.Context, blueprintPath string, m *merger.Merger) {
	dir := filepath.Dir(blueprintPath)

	if rel := registry.FindPatternFile(dir, registry.FileCRD); rel != "" {
		crdPath := filepath.Join(dir, rel)
		out, err := kubectl(ctx, "", "apply", "-f", crdPath)
		if err != nil {
			logger.Warn().Str("path", crdPath).Str("output", out).Err(err).Msgf("%s apply failed", cmdutil.FileCrd)
		} else {
			logger.Info().Str("path", crdPath).Msgf("%s applied", cmdutil.FileCrd)
		}
		waitForCRDsEstablished(ctx, m)
	}

	if rel := registry.FindPatternFile(dir, registry.FileCR); rel != "" {
		crPath := filepath.Join(dir, rel)
		out, err := kubectl(ctx, "", "apply", "-f", crPath)
		if err != nil {
			logger.Warn().Str("path", crPath).Str("output", out).Err(err).Msgf("%s apply failed", cmdutil.FileCr)
		} else {
			logger.Info().Str("path", crPath).Msgf("%s applied", cmdutil.FileCr)
		}
	}
}

// applySetupIfNeeded applies setup YAML files via kubectl in order before
// Inrun starts. Only runs outside the cluster (dev mode).
func applySetupIfNeeded(ctx context.Context, blueprintPath string, m *merger.Merger) {
	if cmdutil.IsRunningInPod() {
		return
	}

	blueprintDir := filepath.Dir(blueprintPath)

	for crdName, entry := range m.All() {
		if !entry.HasPrepare() {
			continue
		}

		for _, prepFile := range entry.Prepare.Apply {
			path := prepFile.Path
			if !filepath.IsAbs(path) && !strings.HasPrefix(path, "http") {
				path = filepath.Join(blueprintDir, path)
			}

			out, err := kubectl(ctx, "", "apply", "-f", path)
			if err != nil {
				logger.Warn().
					Str("crd", crdName).
					Str("path", path).
					Str("output", out).
					Err(err).
					Msg("setup pre-apply failed (continuing)")
			} else {
				logger.Info().
					Str("crd", crdName).
					Str("path", path).
					Msg("setup applied")
			}
		}
	}
}

// kubectl runs kubectl with args (stdin, when not empty, on its input) and
// returns its output, trimmed; a non-zero exit is the error.
func kubectl(ctx context.Context, stdin string, args ...string) (string, error) {
	c := command.Args("kubectl", args...)
	c.Stdin = stdin
	r, err := command.Run(ctx, c)
	if err == nil {
		err = r.Err()
	}
	return strings.TrimSpace(r.Output), err
}
