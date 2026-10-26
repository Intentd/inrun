package cluster

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
