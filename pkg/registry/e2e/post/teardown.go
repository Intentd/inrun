package post

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/tools/inruncluster"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/cluster"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/ptr"
)

// teardownTimeout bounds every delete in teardown. A delete that outlasts it
// is almost always a finalizer whose controller is already gone.
const teardownTimeout = "60s"

// Teardown cleans up every resource applied to an existing cluster.
// Called via defer when e2e runs against --current-context, --cluster or --keep-cluster, where
// deleting the cluster itself is not an option.
//
// The suite's CRs and the custom resources from setup files go first, while
// the controllers that finalize them are still running. The rest is the
// reverse of apply order: Inrun → bundle → setup helm → setup files → CRDs.
// TeardownConfig carries what teardown needs from a run. The two phases are
// kept separate because they are torn down in the reverse of the order they
// ran: prepare (the workload and its dependencies) before environment
// (cluster furniture).
type TeardownConfig struct {
	// CRPaths are the suite's CR files.
	CRPaths []string
	// CRDPaths are the CRD files.
	CRDPaths []string
	// BundlePath is the generated bundle.
	BundlePath string
	// PreparePaths are the manifests applied by prepare.apply.
	PreparePaths []string
	// EnvironmentPaths are the manifests applied by environment.apply.
	EnvironmentPaths []string
	// Prepare is the prepare phase, for its helm releases.
	Prepare *types.Prepare
	// Environment is the environment phase, for its helm releases.
	Environment *types.E2EEnvironment
	// SharedInrun: the Inrun install is shared, so do not uninstall it.
	SharedInrun bool
	// UninstallInrun: uninstall the Inrun release this run installed.
	UninstallInrun bool
}

func Teardown(ctx context.Context, cfg TeardownConfig) {
	fmt.Printf("\n→ Cleaning up resources...\n")

	// Custom resources first — a failed run leaves them behind, and once the
	// runtime is uninstalled nothing removes their finalizers.
	for _, path := range cfg.CRPaths {
		deleteStep(ctx, fmt.Sprintf("Deleting CR %s...", filepath.Base(path)), path)
	}
	// The prepare phase's custom resources, in reverse.
	for i := len(cfg.PreparePaths) - 1; i >= 0; i-- {
		deleteCustomResources(ctx, cfg.PreparePaths[i])
	}

	// Helm uninstall inrun — before bundle delete so the runtime is stopped
	// before its RBAC and ConfigMap are removed.
	if cfg.UninstallInrun && !cfg.SharedInrun {
		uninstallInrun(ctx)
	}

	// Bundle (RBAC, ConfigMap, Namespace created by inrun generate bundle).
	if cfg.BundlePath != "" {
		if !cfg.SharedInrun {
			deleteStep(ctx, "Deleting bundle resources...", cfg.BundlePath)
		}
		os.Remove(cfg.BundlePath)
	}

	// Prepare helm releases, reverse order: the workload's charts before the
	// cluster furniture they depend on.
	if cfg.Prepare != nil {
		uninstallHelm(ctx, "prepare", cfg.Prepare.Helm)
	}

	// Environment helm releases, reverse order: cluster furniture last.
	if cfg.Environment != nil {
		uninstallHelm(ctx, "environment", cfg.Environment.Helm)
	}

	// Prepare manifests in reverse order.
	for i := len(cfg.PreparePaths) - 1; i >= 0; i-- {
		path := cfg.PreparePaths[i]
		deleteStep(ctx, fmt.Sprintf("Deleting prepare %s...", filepath.Base(path)), path)
	}

	// Environment manifests in reverse order: cluster furniture last.
	for i := len(cfg.EnvironmentPaths) - 1; i >= 0; i-- {
		path := cfg.EnvironmentPaths[i]
		deleteStep(ctx, fmt.Sprintf("Deleting environment %s...", filepath.Base(path)), path)
	}

	// CRDs last — deleting a CRD cascades to all CRs of that type.
	for _, path := range cfg.CRDPaths {
		deleteStep(ctx, fmt.Sprintf("Deleting CRD %s...", filepath.Base(path)), path)
	}

	fmt.Printf("  %s Cleanup complete\n", successMark())
}

// uninstallInrun uninstalls the Inrun release this run installed.
func uninstallInrun(ctx context.Context) {
	sp := startSpinner("Uninstalling Inrun...")
	cmd := exec.CommandContext(ctx, "helm", "uninstall", inruncluster.Inrun,
		"--namespace", inruncluster.InrunNamespace, "--ignore-not-found")
	if out, err := cmd.CombinedOutput(); err != nil {
		sp.Failure()
		fmt.Printf("    %v\n%s\n", err, out)
	} else {
		sp.Success()
	}
}

// uninstallHelm uninstalls releases in reverse order, labelled with the phase
// they belong to so a failure names it.
func uninstallHelm(ctx context.Context, phase string, releases []types.ProvisionHelmInstall) {
	for i := len(releases) - 1; i >= 0; i-- {
		h := releases[i]
		sp := startSpinner(fmt.Sprintf("Uninstalling %s helm %s...", phase, h.ReleaseName()))
		if err := cluster.HelmUninstall(ctx, h.HelmRelease()); err != nil {
			sp.Failure()
			fmt.Printf("    %v\n", err)
		} else {
			sp.Success()
		}
	}
}

// deleteStep deletes path behind a spinner labelled msg.
func deleteStep(ctx context.Context, msg, path string) {
	sp := startSpinner(msg)
	onTimeout := func() { sp.Update(strings.TrimSuffix(msg, "...") + " (finalizers cleared)") }
	if err := deleteManifest(ctx, path, onTimeout); err != nil {
		sp.Failure()
		fmt.Printf("    %v\n", err)
		return
	}
	sp.Success()
}

// deleteCustomResources deletes the custom resources in path and leaves
// everything else for the later teardown steps.
func deleteCustomResources(ctx context.Context, path string) {
	docs, err := manifestDocs(path)
	if err != nil {
		return
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	n := 0
	for _, d := range docs {
		if isCustomGroup(d.apiVersion) {
			if err := enc.Encode(d.node); err != nil {
				return
			}
			n++
		}
	}
	if err := enc.Close(); err != nil || n == 0 {
		return
	}

	tmp, err := os.CreateTemp("", "inrun-e2e-crs-*.yaml")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return
	}
	tmp.Close()

	deleteStep(ctx, fmt.Sprintf("Deleting custom resources from %s...", filepath.Base(path)), tmp.Name())
}

// deleteManifest deletes everything in path. If the delete times out, it
// calls onTimeout, clears the finalizers on what is left, including custom
// resources of any CRD in path, and deletes again without waiting.
func deleteManifest(ctx context.Context, path string, onTimeout func()) error {
	out, err := kubectl.Delete(ctx, kubectl.DeleteOptions{
		File:           path,
		IgnoreNotFound: true,
		Timeout:        teardownTimeout,
	})
	if err == nil {
		return nil
	}
	if !strings.Contains(out, "timed out") {
		return fmt.Errorf("%w\n%s", err, out)
	}

	onTimeout()
	_, _ = kubectl.PatchFile(ctx, kubectl.PatchFileOptions{
		File:  path,
		Type:  "merge",
		Patch: `{"metadata":{"finalizers":null}}`,
	})
	if docs, err := manifestDocs(path); err == nil {
		for _, d := range docs {
			if d.kind == "CustomResourceDefinition" && d.name != "" {
				clearCustomResourceFinalizers(ctx, d.name)
			}
		}
	}

	if out, err := kubectl.Delete(ctx, kubectl.DeleteOptions{
		File:           path,
		IgnoreNotFound: true,
		Wait:           ptr.To(false),
	}); err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
}

// clearCustomResourceFinalizers clears the finalizers on every resource of
// crd (plural.group), so a CRD stuck deleting can finish.
func clearCustomResourceFinalizers(ctx context.Context, crd string) {
	out, err := kubectl.Get(ctx, kubectl.GetOptions{
		Kind:          crd,
		AllNamespaces: true,
		Format:        `jsonpath={range .items[*]}{.metadata.namespace}{"/"}{.metadata.name}{"\n"}{end}`,
	})
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		ns, name, ok := strings.Cut(strings.TrimSpace(line), "/")
		if !ok || name == "" {
			continue
		}
		_, _ = kubectl.Patch(ctx, kubectl.PatchOptions{
			Kind:      crd,
			Name:      name,
			Namespace: ns,
			Type:      "merge",
			Patch:     `{"metadata":{"finalizers":null}}`,
		})
	}
}

// manifestDoc is one YAML document of a manifest file.
type manifestDoc struct {
	node       *yaml.Node
	apiVersion string
	kind       string
	name       string
}

// manifestDocs splits the manifest at path into its documents.
func manifestDocs(path string) ([]manifestDoc, error) {
	data, err := readLocal(path)
	if err != nil {
		return nil, err
	}
	var docs []manifestDoc
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var node yaml.Node
		if err := dec.Decode(&node); err != nil {
			if errors.Is(err, io.EOF) {
				return docs, nil
			}
			return nil, err
		}
		var meta struct {
			APIVersion string `yaml:"apiVersion"`
			Kind       string `yaml:"kind"`
			Metadata   struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
		}
		if err := node.Decode(&meta); err != nil || meta.Kind == "" {
			continue
		}
		docs = append(docs, manifestDoc{node: &node, apiVersion: meta.APIVersion, kind: meta.Kind, name: meta.Metadata.Name})
	}
}

// isCustomGroup reports whether apiVersion belongs to a custom resource.
// Built-in groups are either undotted (apps, batch) or end in .k8s.io.
func isCustomGroup(apiVersion string) bool {
	group, _, ok := strings.Cut(apiVersion, "/")
	return ok && strings.Contains(group, ".") && !strings.HasSuffix(group, ".k8s.io")
}
