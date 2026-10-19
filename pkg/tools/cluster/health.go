package cluster

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	healthCheckTimeout    = 200 * time.Second
	resourceExistsTimeout = 10 * time.Second
	consoleDeploy         = InrunConsole
	runtimeLogDir         = "/tmp/inrun"
	runtimeLogPath        = "/tmp/inrun/runtime.log"
	gatewayLogDir         = "/tmp/inrun"
	gatewayLogPath        = "/tmp/inrun/gateway.log"
	consoleLogPath        = "/tmp/inrun/console.log"
)

// RuntimeInstalled reports whether the Inrun runtime Deployment exists.
func RuntimeInstalled() bool {
	return ResourceExists("deploy", InrunRuntime, InrunNamespace)
}

// GatewayInstalled reports whether the Inrun gateway Deployment exists.
func GatewayInstalled() bool {
	return ResourceExists("deploy", InrunGateway, InrunNamespace)
}

// CheckRuntimeHealth waits up to healthCheckTimeout for the Inrun runtime
// Deployment to have at least one ready replica. It polls every 2 seconds.
// Returns immediately if pods are in CrashLoopBackOff.
var (
	runtimeChecker = DeploymentHealthChecker{Name: InrunRuntime, Namespace: InrunNamespace}
	gatewayChecker = DeploymentHealthChecker{Name: InrunGateway, Namespace: InrunNamespace}
	consoleChecker = DeploymentHealthChecker{Name: consoleDeploy, Namespace: InrunNamespace}
)

// CheckRuntimeHealth waits up to healthCheckTimeout for the Inrun runtime to be ready
func CheckRuntimeHealth() DeploymentStatus {
	return runtimeChecker.CheckHealth(healthCheckTimeout, func(ctx context.Context) string {
		return crashLoopReason(ctx)
	})
}

// CheckGatewayHealth waits up to healthCheckTimeout for the Inrun gateway to be ready
func CheckGatewayHealth() DeploymentStatus {
	return gatewayChecker.CheckHealth(healthCheckTimeout, nil)
}

// FetchGatewayLogs saves gateway logs and returns the last 10 lines
func FetchGatewayLogs() (tail string, err error) {
	return gatewayChecker.FetchLogs(100, gatewayLogDir, gatewayLogPath)
}

// FetchConsoleLogsIfNeeded fetches console logs only if the deployment exists but has no ready replicas
func FetchConsoleLogsIfNeeded() error {
	if !consoleChecker.Exists() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if !consoleChecker.HasReadyReplicas(ctx) {
		_, err := consoleChecker.FetchLogs(100, runtimeLogDir, consoleLogPath)
		return err
	}
	return nil
}

// FetchRuntimeLogs saves the last 100 log lines from the Inrun runtime to
// /tmp/inrun/runtime.log. If the Console Deployment exists but has
// no ready replicas, its logs are saved to /tmp/inrun/console.log.
// Returns the last 10 lines of the runtime log for inline display.
func FetchRuntimeLogs() (tail string, err error) {
	tail, err = runtimeChecker.FetchLogs(100, runtimeLogDir, runtimeLogPath)
	if err != nil {
		return "", err
	}

	// Optionally fetch console logs if needed
	_ = FetchConsoleLogsIfNeeded()

	return tail, nil
}

// SyncRuntime restarts the Inrun runtime Deployment and waits for rollout.
func SyncRuntime() error {
	return SyncDeployment(InrunRuntime, InrunNamespace, 3*time.Minute)
}

// SyncGateway restarts the Inrun gateway Deployment and waits for rollout.
func SyncGateway() error {
	return SyncDeployment(InrunGateway, InrunNamespace, 3*time.Minute)
}

// BlueprintChanged returns true when .inrun/blueprint.yaml has uncommitted
// changes or was touched by the most recent commit.
func BlueprintChanged(dir string) bool {
	blueprintPath := filepath.Join(".inrun", "blueprint.yaml")
	if out, err := exec.Command("git", "-C", dir, "diff", "HEAD", "--", blueprintPath).Output(); err == nil {
		if len(bytes.TrimSpace(out)) > 0 {
			return true
		}
	}
	if out, err := exec.Command("git", "-C", dir, "diff", "HEAD~1", "HEAD", "--", blueprintPath).Output(); err == nil {
		if len(bytes.TrimSpace(out)) > 0 {
			return true
		}
	}
	return false
}
