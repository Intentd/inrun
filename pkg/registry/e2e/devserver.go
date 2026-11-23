package e2e

import (
	"fmt"
	"os"
)

const (
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

// writeDevServerManifest writes the dev server manifest to a temp file and
// returns its path.
func writeDevServerManifest() (string, error) {
	f, err := os.CreateTemp("", "inrun-e2e-devserver-*.yaml")
	if err != nil {
		return "", fmt.Errorf("creating dev server manifest: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(devServerManifest); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("writing dev server manifest: %w", err)
	}
	return f.Name(), nil
}
