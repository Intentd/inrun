package inruncluster

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/tools/cluster"
	"github.com/intentd/tools/command"
)

const (
	healthCheckTimeout = 200 * time.Second
	consoleDeploy      = InrunConsole
	runtimeLogDir      = "/tmp/inrun"
	runtimeLogPath     = "/tmp/inrun/runtime.log"
	gatewayLogDir      = "/tmp/inrun"
	gatewayLogPath     = "/tmp/inrun/gateway.log"
	consoleLogPath     = "/tmp/inrun/console.log"
)

// RuntimeInstalled reports whether the Inrun runtime Deployment exists.
func RuntimeInstalled() bool {
	return cluster.ResourceExists("deploy", InrunRuntime, InrunNamespace)
}

// GatewayInstalled reports whether the Inrun gateway Deployment exists.
func GatewayInstalled() bool {
	return cluster.ResourceExists("deploy", InrunGateway, InrunNamespace)
}

// CheckRuntimeHealth waits up to healthCheckTimeout for the Inrun runtime
// Deployment to have at least one ready replica. It polls every 2 seconds.
// Returns immediately if pods are in CrashLoopBackOff.
var (
	runtimeChecker = cluster.DeploymentHealthChecker{Name: InrunRuntime, Namespace: InrunNamespace}
	gatewayChecker = cluster.DeploymentHealthChecker{Name: InrunGateway, Namespace: InrunNamespace}
	consoleChecker = cluster.DeploymentHealthChecker{Name: consoleDeploy, Namespace: InrunNamespace}
)

// CheckRuntimeHealth waits up to healthCheckTimeout for the Inrun runtime to be ready
func CheckRuntimeHealth() cluster.DeploymentStatus {
	return runtimeChecker.CheckHealth(healthCheckTimeout, func(ctx context.Context) string {
		return crashLoopReason(ctx)
	})
}

// CheckGatewayHealth waits up to healthCheckTimeout for the Inrun gateway to be ready
func CheckGatewayHealth() cluster.DeploymentStatus {
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
	return cluster.SyncDeployment(InrunRuntime, InrunNamespace, 3*time.Minute)
}

// SyncGateway restarts the Inrun gateway Deployment and waits for rollout.
func SyncGateway() error {
	return cluster.SyncDeployment(InrunGateway, InrunNamespace, 3*time.Minute)
}

// BlueprintChanged returns true when .inrun/blueprint.yaml has uncommitted
// changes or was touched by the most recent commit.
func BlueprintChanged(dir string) bool {
	blueprintPath := filepath.Join(".inrun", "blueprint.yaml")
	ctx := context.Background()
	if out, err := command.Output(ctx, command.Args("git", "-C", dir, "diff", "HEAD", "--", blueprintPath)); err == nil && out != "" {
		return true
	}
	if out, err := command.Output(ctx, command.Args("git", "-C", dir, "diff", "HEAD~1", "HEAD", "--", blueprintPath)); err == nil && out != "" {
		return true
	}
	return false
}

// crashLoopReason names a runtime pod in CrashLoopBackOff, "" when none is.
func crashLoopReason(ctx context.Context) string {
	out, err := command.Output(ctx, command.Args("kubectl", "get", "pods",
		"-n", InrunNamespace,
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\t"}{range .status.containerStatuses[*]}{.state.waiting.reason}{end}{"\n"}{end}`))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(parts) == 2 && strings.Contains(parts[0], "runtime") && parts[1] == "CrashLoopBackOff" {
			return fmt.Sprintf("pod %s is in CrashLoopBackOff", parts[0])
		}
	}
	return ""
}
