package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/intentd/inrun/pkg/types"
)

const (
	devServerImage     = "ghcr.io/intentd/inrun-dev-server:latest"
	devServerName      = "inrun-dev-server"
	devServerNamespace = "inrun-system"
	// DevServerAddr is the in-cluster DNS address CR specs should use.
	DevServerAddr = "http://inrun-dev-server.inrun-system.svc:9999"
)

// devServerManifest is the Deployment + Service applied into the cluster when
// --dev-server is set. Runs in inrun-system so CRs can reach it at
// http://inrun-dev-server.inrun-system.svc:9999.
const devServerManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: inrun-dev-server
  namespace: inrun-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: inrun-dev-server
  template:
    metadata:
      labels:
        app: inrun-dev-server
    spec:
      containers:
        - name: dev-server
          image: ghcr.io/intentd/inrun-dev-server:latest
          # Use a locally loaded image when there is one.
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 9999
          readinessProbe:
            httpGet:
              path: /health
              port: 9999
            initialDelaySeconds: 2
            periodSeconds: 2
---
apiVersion: v1
kind: Service
metadata:
  name: inrun-dev-server
  namespace: inrun-system
spec:
  selector:
    app: inrun-dev-server
  ports:
    - port: 9999
      targetPort: 9999
`

// applyDevServer writes the dev server manifest to a temp file, applies it,
// waits for the pod to be ready, and returns the temp file path so the caller
// can track it for teardown.
func applyDevServer(ctx context.Context) (string, error) {
	f, err := os.CreateTemp("", "inrun-e2e-devserver-*.yaml")
	if err != nil {
		return "", fmt.Errorf("creating dev server manifest: %w", err)
	}
	if _, err := f.WriteString(devServerManifest); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("writing dev server manifest: %w", err)
	}
	f.Close()

	fmt.Printf("→ Deploying dev server (%s)...\n", devServerImage)
	if out, err := runKubectl(ctx, "", types.NoopEnvResolver(), "apply", "-f", f.Name()); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("applying dev server manifest: %w\n%s", err, out)
	}
	fmt.Printf("  ✓ Dev server deployed\n")
	return f.Name(), nil
}

var devServerChecker = cluster.DeploymentHealthChecker{
	Name:      devServerName,
	Namespace: devServerNamespace,
}

// checkDevServerHealth waits up to 120s for the dev server Deployment to have
// ready replicas, using the same DeploymentHealthChecker pattern as the runtime
// and gateway health checks.
func checkDevServerHealth() error {
	status := devServerChecker.CheckHealth(120*time.Second, nil)
	if !status.Running {
		return fmt.Errorf("dev server not ready: %s", status.Reason)
	}
	return nil
}
