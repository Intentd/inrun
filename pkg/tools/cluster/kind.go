package cluster

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/util/term"
)

const (
	// KindClusterName is the default kind cluster name used by inrun --dev.
	KindClusterName    = "inrun-playground"
	DefaultKindVersion = "v0.27.0"
)

// EnsureKindCluster creates a kind cluster named `name` if it does not already
// exist, then switches kubectl to its context. Downloads the kind binary from
// GitHub releases if not found in PATH or ~/.inrun/bin.
// workers > 0 provisions that many worker nodes in addition to the control-plane.
// version selects the kind release to download; empty string uses DefaultKindVersion.
func EnsureKindCluster(name string, workers int, version string) error {
	kindBin, err := resolveKind(version)
	if err != nil {
		return err
	}

	out, _ := exec.Command(kindBin, "get", "clusters").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == name {
			fmt.Printf("  ✓ Cluster '%s' already exists\n", name)
			return useKindContext(name)
		}
	}

	label := fmt.Sprintf("'%s'", name)
	workerText := "workers"
	if workers == 1 {
		workerText = "worker"
	}
	if workers > 0 {
		label = fmt.Sprintf("'%s' (%d %s)", name, workers, workerText)
	}
	fmt.Printf("  → Creating local cluster %s...\n", label)

	args := []string{"create", "cluster", "--name", name}
	if workers > 0 {
		cfg, cfgErr := writeKindConfig(workers)
		if cfgErr != nil {
			return fmt.Errorf("kind config: %w", cfgErr)
		}
		defer os.Remove(cfg)
		args = append(args, "--config", cfg)
	}

	create := exec.Command(kindBin, args...)
	create.Stdout = os.Stdout
	create.Stderr = os.Stderr
	if err := create.Run(); err != nil {
		return fmt.Errorf("kind create cluster: %w", err)
	}
	fmt.Printf("  %s Cluster '%s' ready\n", term.SuccessMark(), name)

	total := workers + 1 // workers + control-plane
	ready := 0
	spin := term.StartSpinner("   → Waiting for nodes to be ready...")
	defer spin.Failure()
	if err := waitForNodesReady(5*time.Minute, func(_ string) {
		ready++
		spin.Update(fmt.Sprintf("   → Waiting for nodes to be ready... (%d/%d)", ready, total))
	}); err != nil {
		return err
	}
	spin.Update(fmt.Sprintf("Nodes ready (%d/%d)", total, total))
	spin.Success()
	return useKindContext(name)
}

// DeleteKindCluster deletes a kind cluster by name. Safe to call when the
// cluster does not exist.
func DeleteKindCluster(name string) error {
	kindBin, err := resolveKind(DefaultKindVersion)
	if err != nil {
		return err
	}
	cmd := exec.Command(kindBin, "delete", "cluster", "--name", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ── private helpers ───────────────────────────────────────────────────────────

func useKindContext(name string) error {
	ctxName := "kind-" + name
	use := exec.Command("kubectl", "config", "use-context", ctxName)
	use.Stdout = os.Stdout
	use.Stderr = os.Stderr
	if err := use.Run(); err != nil {
		return fmt.Errorf("switching kubectl context to %s: %w", ctxName, err)
	}
	return nil
}

func resolveKind(version string) (string, error) {
	if version == "" {
		// No explicit version: prefer whatever is already in PATH.
		if p, err := exec.LookPath("kind"); err == nil {
			return p, nil
		}
		version = DefaultKindVersion
	}
	// Explicit version: use the versioned cache entry, downloading if needed.
	dest := filepath.Join(inrunBinDir(), "kind-"+version)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	return downloadKind(dest, version)
}

func downloadKind(dest, version string) (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	url := fmt.Sprintf(
		"https://github.com/kubernetes-sigs/kind/releases/download/%s/kind-%s-%s",
		version, goos, goarch,
	)

	fmt.Printf("  → Downloading kind %s...\n", version)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("kind: cannot create bin dir: %w", err)
	}

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("kind: download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kind: download returned %s for %s", resp.Status, url)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", fmt.Errorf("kind: writing binary: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(dest)
		return "", fmt.Errorf("kind: writing binary: %w", err)
	}
	f.Close()

	fmt.Printf("  %s kind %s installed (%s)\n", term.SuccessMark(), version, dest)
	return dest, nil
}

func writeKindConfig(workers int) (string, error) {
	f, err := os.CreateTemp("", "inrun-kind-*.yaml")
	if err != nil {
		return "", err
	}
	defer f.Close()
	fmt.Fprintf(f, "kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\nnodes:\n- role: control-plane\n")
	for range workers {
		fmt.Fprintf(f, "- role: worker\n")
	}
	return f.Name(), nil
}

func waitForNodesReady(timeout time.Duration, onNodeReady func(nodeName string)) error {
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("pipe: %w", err)
	}

	cmd := exec.Command("kubectl", "wait", "--for=condition=Ready",
		"node", "--all", fmt.Sprintf("--timeout=%ds", int(timeout.Seconds())))
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("kubectl wait: %w", err)
	}

	// Read per-node "condition met" lines without writing to terminal.
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			// kubectl prints: "node/inrun-playground-worker condition met"
			if strings.Contains(line, "condition met") {
				parts := strings.Fields(line)
				if len(parts) > 0 && onNodeReady != nil {
					onNodeReady(strings.TrimPrefix(parts[0], "node/"))
				}
			}
		}
	}()

	runErr := cmd.Wait()
	pw.Close()
	<-done
	pr.Close()

	if runErr != nil {
		return fmt.Errorf("nodes not ready after %s", timeout)
	}
	return nil
}

// inrunBinDir returns ~/.inrun/bin — where Inrun stores downloaded tools.
func inrunBinDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".inrun", "bin")
}
