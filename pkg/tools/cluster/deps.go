package cluster

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/intentd/inrun/pkg/utils"
)

const (
	getHelm               = "curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash"
	getKubectl            = "curl -LO https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl && "
	makeKubectlExecutable = "chmod +x kubectl && sudo mv kubectl /usr/local/bin/"
)

// KubectlAvailable reports whether kubectl is present in PATH.
func KubectlAvailable() bool {
	_, err := exec.LookPath("kubectl")
	return err == nil
}

// HelmAvailable reports whether helm is present in PATH.
func HelmAvailable() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}

// ClusterReachable reports whether kubectl can reach the cluster configured
// in the current kubeconfig. Times out after 5 seconds.
func ClusterReachable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "kubectl", "cluster-info", "--request-timeout=5s").Run() == nil
}

// EnsureDependencies installs kubectl and helm if they are missing.
func EnsureDependencies() error {
	if !KubectlAvailable() {
		spin := utils.StartSpinner("  → Installing kubectl...")
		if err := installKubectl(); err != nil {
			spin.Failure()
			return fmt.Errorf("failed to install kubectl: %w", err)
		}
		spin.Success()
	}

	if !HelmAvailable() {
		spin := utils.StartSpinner("  → Installing helm...")
		if err := installHelm(); err != nil {
			spin.Failure()
			return fmt.Errorf("failed to install helm: %w", err)
		}
		spin.Success()
	}
	return nil
}

func installKubectl() error {
	cmd := exec.Command("sh", "-c", getKubectl+makeKubectlExecutable)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func installHelm() error {
	cmd := exec.Command("sh", "-c", getHelm)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}
