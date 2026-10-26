//go:build integration

// Package testenv provides a shared envtest lifecycle for integration tests.
// Each integration package starts its own environment in TestMain — envtest is
// cheap to start (~1s) and isolation between packages avoids CRD/namespace
// collisions without needing coordination.
//
// Usage in a test package:
//
//	func TestMain(m *testing.M) {
//	    env := testenv.Start([]string{"path/to/crds"})
//	    code := m.Run()
//	    env.Stop()
//	    os.Exit(code)
//	}
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// Env wraps an envtest.Environment and the derived clients.
type Env struct {
	inner  *envtest.Environment
	Config *rest.Config
	// Dynamic is a dynamic client backed by the envtest API server.
	Dynamic dynamic.Interface
}

// Start launches the envtest API server and installs any CRDs at the given
// paths. Exits the process on failure — integration tests cannot run without
// a working API server.
func Start(crdPaths []string) *Env {
	assetsDir := os.Getenv("KUBEBUILDER_ASSETS")
	if assetsDir == "" {
		// Fallback: walk up from this file's directory to find a known location
		_, file, _, _ := runtime.Caller(0)
		assetsDir = filepath.Join(filepath.Dir(file), "../../../../.envtest-bins")
	}

	inner := &envtest.Environment{
		BinaryAssetsDirectory: assetsDir,
		CRDDirectoryPaths:     crdPaths,
		ErrorIfCRDPathMissing: len(crdPaths) > 0,
	}

	cfg, err := inner.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "envtest: failed to start API server: %v\n", err)
		os.Exit(1)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		inner.Stop() //nolint:errcheck
		fmt.Fprintf(os.Stderr, "envtest: failed to build dynamic client: %v\n", err)
		os.Exit(1)
	}

	return &Env{
		inner:   inner,
		Config:  cfg,
		Dynamic: dyn,
	}
}

// Stop tears down the envtest API server. Call in TestMain after m.Run().
func (e *Env) Stop() {
	if err := e.inner.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "envtest: stop failed: %v\n", err)
	}
}
