package e2e

import (
	"text/template"

	"github.com/intentd/inrun/pkg/types"
)

// Runner maps one Inrun e2e test to jig and runs it.
type Runner struct {
	e2e           types.E2E
	e2eDir        string // directory of the e2e.yaml file — resolves relative paths
	e2eFile       string
	blueprintFile string
	crFiles       []string
	inrunVersion  string
	inrunChart    string // a local chart directory instead of the published chart
	valueFiles    []string
	helmArgs      []string
	devServer     bool // deploy the mock dev server before Inrun
	noRuntime     bool // Inrun without its runtime: gateway only, no CR steps
	// opts are the options the runner was made with: the run's, and where
	// imports start from.
	opts Options
}

// Options configures a Runner. All fields are optional — zero values produce
// the same behaviour as the previous positional defaults.
type Options struct {
	Context       string   // use an existing kubectl context, skip cluster creation
	UseCurrentCtx bool     // use the current kubectl context as-is
	KeepCluster   bool     // do not delete the kind cluster after the run
	Keep          bool     // leave everything as the run left it: no teardown, the cluster kept
	Workers       int      // number of kind worker nodes (0 = control-plane only)
	KindVersion   string   // kind binary version to download ("" = DefaultKindVersion)
	DevServer     bool     // deploy the mock dev server into the cluster
	InrunVersion  string   // Inrun helm chart version to install
	InrunChart    string   // local chart directory to install instead of the published chart
	ValueFiles    []string // additional Helm values files
	HelmArgs      []string // additional helm --set arguments
	ReportFile    string   // write results to this path too: JSON for .json, else Markdown
	NoRuntime     bool     // skip the Inrun runtime; gateway only; CR is optional
	Command       string   // Command is the invocation that produced this result,
	DryRun        bool     // DryRun makes mutating kubectl invocations server-dry-run.
	MaxFailures   int      // Stop the run after N failed expectations
	// Env is the parent aggregator's spec.env, inherited by every import.
	// The import's own spec.env takes precedence on collision.
	Env map[string]string
	// PushTriggered is set by `inrun push` when it runs the E2E gate. It makes
	// the runner record the spec path relative to the pattern root, so the
	// report that ships in the artifact does not carry an absolute path.
	PushTriggered bool
	// Functions are the template functions expectations and post blocks
	// start from; nil is KEL's. The spec's functions are added to them.
	Functions template.FuncMap
	// Images are container images to load into the cluster before the test.
	// Narrows setup.images when both are set; standalone when the schema
	// declares none (eg. CI case).
	Images []string
}
