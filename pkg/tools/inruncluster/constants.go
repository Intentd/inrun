package inruncluster

import (
	"os"
	"path/filepath"

	"github.com/intentd/tools/cluster"
)

const (
	// Inrun is the Helm release name and the prefix for all Inrun resources.
	Inrun = "inrun"

	// InrunRuntime is the name of the runtime Deployment.
	InrunRuntime = "inrun-runtime"

	// InrunGateway is the name of the gateway Deployment.
	InrunGateway = "inrun-gateway"

	// InrunNamespace is the Kubernetes namespace Inrun deploys into.
	InrunNamespace = "inrun-system"

	// InrunChartRepo is the Helm repository URL for the Inrun chart.
	InrunChartRepo = "https://intentd.github.io/inrun"

	// InrunChartName is the chart name within the repository.
	InrunChartName = "inrun"

	// InrunConsole is the name of the Console Deployment.
	InrunConsole = "inrun-console"

	// InrunConsolePort is the default Console HTTP port.
	InrunConsolePort = "8081"
)

// KindClusterName is the kind cluster inrun --dev creates.
const KindClusterName = "inrun-playground"

// BinDir is ~/.inrun/bin, where Inrun keeps the tools it downloads.
func BinDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".inrun", "bin")
}

// Kind is the kind setup Inrun's commands use: workers and version as given,
// kind kept in BinDir.
func Kind(workers int, version string) cluster.KindOptions {
	return cluster.KindOptions{Workers: workers, Version: version, BinDir: BinDir()}
}
