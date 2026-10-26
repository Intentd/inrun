package e2e

import (
	"github.com/intentd/inrun/pkg/registry/e2e/expect"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	defaultClusterName = "inrun-e2e"
	defaultProvider    = "kind"
)

// Runner executes a single E2E spec end-to-end.
type Runner struct {
	e2e                       types.E2E
	e2eDir                    string // directory of the e2e.yaml file — resolves relative paths
	keepCluster, needsCluster bool
	useCurrentCtx             bool   // Default (false) - means whether to use the current context, skip cluster creation
	clusterCtx                string // non-empty means use this context, skip cluster creation
	workers                   int    // number of kind worker nodes to provision (0 = control-plane only)
	kindVersion               string // kind binary version to use ("" = DefaultKindVersion)

	blueprintFile string
	crFiles       []string

	// Inrun installation options
	inrunVersion string
	// inrunChart is a local chart directory to install instead of the published chart.
	inrunChart string
	valueFiles []string
	helmArgs   []string

	// devServer deploys the mock dev server into the cluster as part of setup.
	devServer bool

	// reportFile is the path to write the markdown results report to.
	// Empty means no file is written (stdout only).
	reportFile string

	// kubernetesTarget skips bundle generation and Inrun helm install/uninstall.
	// Set when spec.custom.target == "kubernetes" — the file is the source of truth.
	kubernetesTarget bool

	// sharedInrun means Inrun is managed by the parent runImports coordinator.
	// The sub-runner must not delete the bundle from the cluster (the bundle contains
	// the inrun-system namespace; deleting it cascades to the Inrun deployment).
	// It also suppresses all sync/health-check output — only the coordinator's single
	// install and uninstall messages are visible.
	sharedInrun bool

	// noRuntime skips starting the Inrun runtime. Only the gateway is started
	// when the blueprint enables it. The CR lifecycle steps (AfterCRApplied, AfterCRDeleted)
	// become no-ops — the CR is optional and gateway intents drive the expectations instead.
	noRuntime bool

	// command is the invocation that produced this result,
	//  e.g. "inrun e2e --use-current".
	command        string
	e2eFile        string
	pushTriggered  bool
	maxFailures    int
	suppressReport bool

	// cs and cfg are the Go Kubernetes client and REST config, built once after the
	// cluster context is ready. Used for operations that don't need kubectl:
	// Lease reads, port-forward+HTTP, SubjectAccessReview.
	cs  kubernetes.Interface
	cfg *rest.Config
	// env is the parent aggregator's spec.env, inherited by every import.
	// The import's own spec.env takes precedence on collision.
	env map[string]string
}

// Options configures a Runner. All fields are optional — zero values produce
// the same behaviour as the previous positional defaults.
type Options struct {
	ClusterCtx    string   // use an existing kubectl context, skip cluster creation
	UseCurrentCtx bool     // use the current kubectl context as-is
	KeepCluster   bool     // do not delete the kind cluster after the run
	Workers       int      // number of kind worker nodes (0 = control-plane only)
	KindVersion   string   // kind binary version to download ("" = DefaultKindVersion)
	DevServer     bool     // deploy the mock dev server into the cluster
	InrunVersion  string   // Inrun helm chart version to install
	InrunChart    string   // local chart directory to install instead of the published chart
	ValueFiles    []string // additional Helm values files
	HelmArgs      []string // additional helm --set arguments
	ReportFile    string   // write results as markdown to this path (in addition to stdout)
	NoRuntime     bool     // skip the Inrun runtime; gateway only; CR is optional
	Command       string   // Command is the invocation that produced this result,
	MaxFailures   int      // Stop the run after N failed expectations
	// Env is the parent aggregator's spec.env, inherited by every import.
	// The import's own spec.env takes precedence on collision.
	Env map[string]string
	// PushTriggered is set by `inrun push` when it runs the E2E gate. It makes
	// the runner record the spec path relative to the pattern root, so the
	// report that ships in the artifact does not carry an absolute path.
	PushTriggered bool
}

type expectationResult struct {
	result   *expect.Result
	failures int
	stopped  error
}

type dependencyResult struct {
	appliedCRDPaths   []string
	appliedBundlePath string
	appliedSetupPaths []string
	expects           []types.E2EExpectation
	installedInrun    bool
}
