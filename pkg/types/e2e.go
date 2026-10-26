package types

import (
	"fmt"
	"os"
	"strings"

	"github.com/intentd/kel/core"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SetupConfig declares prerequisite resources to apply before Inrun starts.
//
// Shorthand — a plain list of strings is equivalent to setup.apply:
//
//	setup:
//	  - ./prereqs/secret.yaml
//
// Struct form with per-entry waits:
//
//	setup:
//	  apply:
//	    - ./prereqs/namespace.yaml
//	    - path: ./prereqs/secret.yaml
//	      wait:
//	        - kind: Secret
//	          name: my-secret
//	          namespace: default
//	          timeout: 30s
//	  helm:
//	    - repo: https://charts.cert-manager.io
//	      chart: cert-manager
//	      version: v1.14.0
//	      wait:
//	        - kind: Deployment
//	          name: cert-manager
//	          namespace: cert-manager
//	          ready: true
//	          timeout: 120s
//	  wait:
//	    - kind: Deployment
//	      name: cert-manager-webhook
//	      namespace: cert-manager
//	      ready: true
//	      timeout: 120s
type SetupConfig struct {
	// Apply is an ordered list of manifests to kubectl-apply.
	// Each entry is either a plain path string or a {path, wait} struct.
	// Applied first, before helm installs, after the CRD is installed.
	Apply []SetupApplyEntry `yaml:"apply,omitempty"`

	// Helm is an ordered list of Helm charts to install before Inrun starts.
	// Executed as helm upgrade --install — not rendered for Blueprint extraction.
	Helm []SetupHelmInstall `yaml:"helm,omitempty"`

	// Wait blocks until all listed resources exist and satisfy conditions.
	// Runs after all apply and helm steps. Use per-entry wait for ordered checks.
	Wait []SetupWait `yaml:"wait,omitempty"`
}

// UnmarshalYAML allows setup: to be written as either a plain list of strings (setup.apply) or a full struct.
func (s *SetupConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				break
			}
			s.Apply = append(s.Apply, SetupApplyEntry{Path: item.Value})
		}
		if len(s.Apply) > 0 {
			return nil
		}
	}
	type plain SetupConfig
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return strictUnmarshal(raw, (*plain)(s))
}

// SetupApplyEntry is a single manifest to kubectl-apply during setup.
// It is either a plain path string or a struct with an optional per-entry wait.
//
//	# flat form
//	- ./prereqs/secret.yaml
//
//	# structured form
//	- path: ./prereqs/secret.yaml
//	  wait:
//	    - kind: Secret
//	      name: my-secret
//	      namespace: default
//	      timeout: 30s
type SetupApplyEntry struct {
	// Path is the YAML file path to kubectl-apply.
	Path string `yaml:"path"`
	// Wait blocks after this apply until all listed resources satisfy their conditions.
	Wait []SetupWait `yaml:"wait,omitempty"`
}

// UnmarshalYAML allows a SetupApplyEntry to be written as either a plain string
// (the path) or a full {path, wait} struct.
func (e *SetupApplyEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		e.Path = value.Value
		return nil
	}
	type plain SetupApplyEntry
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return strictUnmarshal(raw, (*plain)(e))
}

// SetupHelmInstall installs a Helm chart as a real release into the cluster.
// Unlike HelmSource (which renders charts to extract Blueprint documents),
// this runs helm upgrade --install for a prerequisite chart.
type SetupHelmInstall struct {
	// Repo is the Helm repository URL. Omit for local chart paths.
	Repo string `yaml:"repo,omitempty"`
	// Chart is the chart name within the repository, or a local path (e.g. ./).
	Chart string `yaml:"chart"`
	// Release is the Helm release name. Defaults to the chart name when empty.
	Release string `yaml:"release,omitempty"`
	// Namespace for the release. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Version pins the chart version. Leave empty for latest or local charts.
	Version string `yaml:"version,omitempty"`
	// ValueFiles is an ordered list of values files (local paths or URLs).
	ValueFiles []string `yaml:"valueFiles,omitempty"`
	// Values are inline key-value overrides, equivalent to helm --set.
	Values map[string]interface{} `yaml:"values,omitempty"`
	// CreateNamespace passes --create-namespace to helm.
	CreateNamespace bool `yaml:"createNamespace,omitempty"`
	// Wait blocks after this helm install until all listed resources satisfy conditions.
	Wait []SetupWait `yaml:"wait,omitempty"`
}

// ReleaseName returns the effective Helm release name.
func (h SetupHelmInstall) ReleaseName() string {
	if h.Release != "" {
		return h.Release
	}
	return h.Chart
}

// EffectiveNamespace returns the effective namespace, defaulting to "default".
func (h SetupHelmInstall) EffectiveNamespace() string {
	if h.Namespace != "" {
		return h.Namespace
	}
	return "default"
}

// IsLocalChart reports whether the chart field is a local filesystem path.
func (h SetupHelmInstall) IsLocalChart() bool {
	return strings.HasPrefix(h.Chart, "./") || strings.HasPrefix(h.Chart, "/") || h.Chart == "."
}

// Validate returns an error when required fields are missing.
func (h SetupHelmInstall) Validate() error {
	if h.Chart == "" {
		return fmt.Errorf("setup.helm: chart is required")
	}
	if !h.IsLocalChart() && h.Repo == "" {
		return fmt.Errorf("setup.helm: repo is required for remote charts")
	}
	return nil
}

// SetupWait describes a single resource to wait for before the operator starts.
type SetupWait struct {
	// Kind is the Kubernetes resource kind (e.g. Deployment, Secret, Namespace).
	Kind string `yaml:"kind"`
	// Name is the exact resource name.
	Name string `yaml:"name"`
	// Namespace to look in. Omit for cluster-scoped resources.
	Namespace string `yaml:"namespace,omitempty"`
	// Ready waits for an available/ready condition, not just existence.
	Ready bool `yaml:"ready,omitempty"`
	// Timeout is a Go duration string. Default: "30s".
	Timeout string `yaml:"timeout,omitempty"`
}

// E2E is the top-level document type for declarative end-to-end tests.
// Committed alongside the blueprint, it drives `inrun e2e` — the same command
// that runs locally, in CI, and inside the GitHub Action.
type E2E struct {
	// APIVersion — inrun.dev/v1.
	APIVersion string `yaml:"apiVersion"`
	// Kind — E2E.
	Kind string `yaml:"kind"`
	// Metadata — the test's name and description, shown in results.
	Metadata E2EMeta `yaml:"metadata"`
	// Spec — the Blueprint under test, the cluster, setup and expectations.
	Spec E2ESpec `yaml:"spec"`
	// Imports — other E2E files to run after this one, on the same cluster
	// unless an import sets freshCluster.
	Imports []E2EImport `yaml:"imports,omitempty"`
}

// E2EImport references another E2E file to run after this one completes.
// By default imports share the same cluster. Set freshCluster: true to
// provision a new cluster for that import instead.
//
// Shorthand — a plain path string is equivalent to {path: <string>}:
//
//	imports:
//	  - ./auth-e2e.yaml
//	  - ./rbac-e2e.yaml
//	  - path: ./infra-e2e.yaml
//	    freshCluster: true
type E2EImport struct {
	// Path is the path to another E2E spec file (must be kind: E2E).
	Path string `yaml:"path"`
	// FreshCluster provisions a new kind cluster for this import instead of
	// reusing the parent's cluster. Default: false (share parent cluster).
	FreshCluster bool `yaml:"freshCluster,omitempty"`
	// Wait is an optional duration to sleep before this import starts.
	// Useful when the previous test leaves cluster state that needs time to
	// clear — webhook deregistration, namespace termination, cert provisioning.
	// Must be a valid Go duration string (e.g. "10s", "1m30s").
	Wait string `yaml:"wait,omitempty"`
}

// UnmarshalYAML allows imports to be written as a plain string path.
func (i *E2EImport) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		i.Path = value.Value
		return nil
	}
	type plain E2EImport
	return value.Decode((*plain)(i))
}

type E2EMeta struct {
	// Name — shown in the results.
	Name string `yaml:"name"`
	// Description — what the test checks.
	Description string `yaml:"description,omitempty"`
}

// CustomTarget identifies the runtime environment being tested when Inrun
// is not the operator. Validation rejects values outside the known set.
type CustomTarget string

const (
	// CustomTargetKubernetes tests a workload that runs on Kubernetes — an operator,
	// Helm chart, or raw manifests. Inrun manages the cluster lifecycle and
	// assertions; bundle generation and Inrun helm install/uninstall are skipped.
	CustomTargetKubernetes CustomTarget = "kubernetes"

	// CustomTargetContainer tests a container image directly without a cluster.
	// Future: build image → run container gate → embed OCI annotations → push.
	// Not yet implemented; validation accepts the value but no runner path exists.
	CustomTargetContainer CustomTarget = "container"
)

// String implements fmt.Stringer.
func (t CustomTarget) String() string { return string(t) }

// E2ECustomConfig declares the target runtime when testing non-Inrun workloads.
type E2ECustomConfig struct {
	// Target is the runtime environment under test. Supported values: "kubernetes".
	// "container" is reserved for future use.
	Target CustomTarget `yaml:"target"`
}

type E2ESpec struct {
	// Core operator spec — the three files that define every Inrun operator.

	// Blueprint is the path to the blueprint.yaml file.
	// Optional when spec.custom.target is set.
	Blueprint string `yaml:"blueprint,omitempty"`
	// CRD is the path to the CRD YAML file for this operator.
	// Applied before the bundle and before Inrun starts.
	CRD string `yaml:"crd,omitempty"`
	// CRDFiles is a list of additional CRD YAML files. Applied in declaration order
	// after CRD. Use when the operator manages multiple custom resource
	CRDFiles []string `yaml:"crdFiles,omitempty"`
	// CR is the path to the CR YAML file to apply during the test.
	CR string `yaml:"cr,omitempty"`
	// CRFiles is a list of additional CR YAML files. Applied in declaration order
	// after CR. Use when the test needs multiple CR instances or kinds.
	CRFiles []string `yaml:"crFiles,omitempty"`

	// Custom declares the target runtime for this e2e test when it is not an
	// Inrun-managed operator. Inrun still owns the cluster lifecycle, setup,
	// CR apply, assertions, and cleanup — only the bundle generation and
	// Inrun helm install/uninstall are skipped.
	//
	// Use this when your operator, Helm chart, or any Kubernetes workload is
	// installed via setup.helm or is already present in the cluster.
	// See the custom target in docs/reference/schema/e2e/complete-example.md.
	Custom *E2ECustomConfig `yaml:"custom,omitempty"`

	// Init uses an example pack — for Inrun's own CI.
	Init *E2EInit `yaml:"init,omitempty"`

	// Cluster controls which cluster to use.
	Cluster E2ECluster `yaml:"cluster"`

	// Setup declares prerequisite resources to apply before Inrun starts.
	// Shorthand: a plain list of strings applies each file
	// Struct form adds helm installs and resource waiting.
	Setup *SetupConfig `yaml:"setup,omitempty"`

	// ValuesFiles is a list of Helm values files passed to the Inrun chart
	// installation during e2e. Paths are relative to the e2e.yaml file.
	// Use this to configure custom runtime images or any other Helm values
	// without requiring --values flags on the command line — useful when e2e
	// runs automatically during inrun push.
	ValuesFiles []string `yaml:"valuesFiles,omitempty"`

	// Functions declares user-defined functions available in when:/or: expressions.
	// Same syntax as a Blueprint functions block — same FuncMap registration, same functions.
	Functions KELFunctionRegistry `yaml:"functions,omitempty"`

	// Expect is the list of expectations to check after each lifecycle event.
	Expect []E2EExpectation `yaml:"expect"`

	// OnFailure declares kubectl operations and commands to run and print to the
	// terminal when any expectation fails. Uses the same kubectl: DSL as expect
	// entries — assertions are never evaluated; output is always printed.
	OnFailure *E2EOnFailure `yaml:"onFailure,omitempty"`

	// Runtime controls runtime-specific behavior during the e2e test.
	Runtime *E2ERuntimeConfig `yaml:"runtime,omitempty"`

	// Env declares environment variables available to all expectations in this
	// e2e spec. Values support variable expansion.
	Env map[string]string `yaml:"env,omitempty"`
}

// EnvResolver resolves ${VAR} references for one expectation and exposes the
// declared values for processes that need them as environment variables
// (sh -c). Built by E2ESpec.ResolveEnv.
type EnvResolver struct {
	lookup func(string) string
	values map[string]string
}

// NoopEnvResolver returns an EnvResolver that declares nothing: Lookup falls
// through to the process environment, and Pairs returns nil. Use it where an
// EnvResolver is required but no spec.env/expect.env applies.
func NoopEnvResolver() EnvResolver {
	return EnvResolver{}
}

// Lookup returns the value for key: expectation env, then spec env, then the
// process environment. Suitable for os.Expand.
func (e EnvResolver) Lookup(key string) string {
	if e.lookup != nil {
		return e.lookup(key)
	}
	return os.Getenv(key)
}

// Pairs returns the declared key=value entries (spec env overlaid by
// expectation env) for exec.Cmd.Env. Process-environment values are not
// included — append os.Environ() to preserve the rest.
func (e EnvResolver) Pairs() []string {
	pairs := make([]string, 0, len(e.values))
	for k, v := range e.values {
		pairs = append(pairs, k+"="+v)
	}
	return pairs
}

// ResolveEnv returns an EnvResolver for expect, scoped to this spec.
func (s *E2ESpec) ResolveEnv(expect E2EExpectation) EnvResolver {
	merged := make(map[string]string, len(expect.Env))
	if s != nil {
		for k, v := range s.Env {
			merged[k] = v
		}
	}
	for k, v := range expect.Env {
		merged[k] = v
	}

	return EnvResolver{
		values: merged,
		lookup: func(key string) string {
			if v, ok := merged[key]; ok {
				return v
			}
			return os.Getenv(key)
		},
	}
}

// InheritEnv merges parent into s.Env as defaults: keys already declared in
// s.Env keep their value, keys only the parent declares are added. Used when
// a suite (aggregator) passes its spec.env down to an import, so the suite
// can declare a shared value once and each leaf overrides it if needed.
func (s *E2ESpec) InheritEnv(parent map[string]string) {
	if len(parent) == 0 {
		return
	}
	if s.Env == nil {
		s.Env = make(map[string]string, len(parent))
	}
	for k, v := range parent {
		if _, ok := s.Env[k]; !ok {
			s.Env[k] = v
		}
	}
}

type E2ERuntimeConfig struct {
	// Enabled controls whether Inrun runtime health checks are performed.
	// Defaults to true.
	Enabled *bool `yaml:"enabled,omitempty"`
}

// RuntimeEnabled reports whether the runtime is enabled, defaulting to true.
func (s E2ESpec) RuntimeEnabled() bool {
	if s.Runtime == nil || s.Runtime.Enabled == nil {
		return true
	}
	return *s.Runtime.Enabled
}

// AllCRPaths returns all CR file paths in declaration order (cr then crFiles).
func (s *E2ESpec) AllCRPaths() []string {
	var paths []string
	if s.CR != "" {
		paths = append(paths, s.CR)
	}
	paths = append(paths, s.CRFiles...)
	return paths
}

// AllCRDPaths returns all CRD file paths in declaration order (crd then crdFiles).
func (s *E2ESpec) AllCRDPaths() []string {
	var paths []string
	if s.CRD != "" {
		paths = append(paths, s.CRD)
	}
	paths = append(paths, s.CRDFiles...)
	return paths
}

// E2EOnFailure declares operations to run when an expectation fails.
type E2EOnFailure struct {
	E2ESideEffects `yaml:",inline"`

	// Stop halts the run after the diagnostics for this failure. At spec
	// level it applies to any expectation that fails; at expectation level it
	// applies only to that one. The failure is still recorded and reported;
	// Stop only decides whether later expectations run.
	Stop *bool `yaml:"stop,omitempty"`
}

// Declares operations to run after an expectation completes —
// whether it passed, failed or was skipped — before the next expectation
// runs. Operations are never asserted on; failures are printed, not counted.
// Use it to undo mutations a step made to files or cluster state.
type E2EFinally struct {
	E2ESideEffects `yaml:",inline"`
}

// E2ESideEffects declares operations that run alongside an expectation
// without being asserted on. Assertions on these operations are ignored;
// failures are printed, not counted. Used by onFailure: (failure only)
// and finally: (always).
type E2ESideEffects struct {
	// Kubectl — kubectl operations to run, in the same form as expect
	// entries; their assertions are ignored.
	Kubectl *E2EKubectl `yaml:"kubectl,omitempty"`
	// Commands — shell commands to run, each with sh -c.
	Commands []string `yaml:"commands,omitempty"`
}

// Empty reports whether no operation is declared.
func (s *E2ESideEffects) Empty() bool {
	return s == nil || (s.Kubectl == nil && len(s.Commands) == 0)
}

// HasOnFailure reports whether this expectation declares failure diagnostics.
func (e *E2EExpectation) HasOnFailure() bool {
	return e.OnFailure != nil && !e.OnFailure.Empty()
}

// HasFinally reports whether this expectation declares post-step operations.
func (e *E2EExpectation) HasFinally() bool {
	return e.Finally != nil && !e.Finally.Empty()
}

// EffectiveStop reports whether a failure under this spec should stop the run
// after diagnostics. Spec-level onFailure.stop is the default; an expectation
// that sets its own onFailure.stop overrides it.
func (s *E2ESpec) EffectiveStop() bool {
	if s.OnFailure != nil && s.OnFailure.Stop != nil {
		return *s.OnFailure.Stop
	}
	return false
}

// EffectiveStop reports whether this expectation failing should stop the run
// after diagnostics. An expectation-level onFailure.stop wins; otherwise the
// spec-level value applies.
func (e *E2EExpectation) EffectiveStop(spec *E2ESpec) bool {
	if e.OnFailure != nil && e.OnFailure.Stop != nil {
		return *e.OnFailure.Stop
	}
	if spec != nil {
		return spec.EffectiveStop()
	}
	return false
}

// E2EEffectiveStop reports whether a failure of exp should stop the run after
// diagnostics. An expectation-level onFailure.stop wins over the spec-level
// value; with neither set, failures do not stop the run.
func E2EEffectiveStop(spec *E2ESpec, exp *E2EExpectation) bool {
	if exp == nil {
		if spec != nil {
			return spec.EffectiveStop()
		}
		return false
	}
	return exp.EffectiveStop(spec)
}

// E2EInit selects a built-in example pack as the test source.
type E2EInit struct {
	// Pack — the example pack, as in inrun init --pack.
	Pack string `yaml:"pack"`
	// Example — the example inside the pack to test.
	Example string `yaml:"example"`
}

// ClusterProvider is the cluster backend an E2E run uses.
type ClusterProvider string

const (
	// ProviderKind creates an ephemeral kind cluster and deletes it after.
	ProviderKind ClusterProvider = "kind"
	// ProviderNone runs without a cluster: only commands: may be used.
	// cluster.provider: none implies no bundle, no Inrun install, no context switch.
	ProviderNone ClusterProvider = "none"
)

// String returns the annotation/CLI form (stable).
func (p ClusterProvider) String() string { return string(p) }

// validClusterProviders is the authoritative set used by inrun validate.
var validClusterProviders = []string{
	ProviderKind.String(), ProviderNone.String(),
}

// ValidClusterProviders returns the supported providers, in display order.
func ValidClusterProviders() []string {
	return validClusterProviders
}

// IsValid reports whether p is a known provider.
func (p ClusterProvider) IsValid() bool {
	for _, prov := range validClusterProviders {
		if p.String() == prov {
			return true
		}
	}
	return false
}

// RequiresCluster reports whether this provider needs a reachable cluster.
func (p ClusterProvider) RequiresCluster() bool {
	return p != ProviderNone
}

// EffectiveProvider returns the declared provider, defaulting to kind.
func (c E2ECluster) EffectiveProvider() ClusterProvider {
	if c.Provider == "" {
		return ProviderKind
	}
	return c.Provider
}

// E2ECluster controls cluster creation and selection.
type E2ECluster struct {
	// Provider is the cluster backend. Defaults to kind.
	Provider ClusterProvider `yaml:"provider,omitempty"`
	// Name is the kind cluster name. Default: "inrun-e2e".
	Name string `yaml:"name"`
	// Reuse controls whether an existing cluster is reused or recreated.
	// false (default) — delete and recreate for a clean state.
	// true — reuse existing cluster if it exists.
	Reuse bool `yaml:"reuse"`
}

// E2EAfter is the lifecycle event that triggers an expectation block.
type E2EAfter string

const (
	// AfterSetupComplete runs the expectation after all setup steps finish,
	// before the CR is applied. Use for infrastructure assertions.
	AfterSetupComplete E2EAfter = "setup-complete"

	// AfterCRApplied runs the expectation after the CR is applied to the cluster.
	AfterCRApplied E2EAfter = "cr-applied"

	// AfterCRDeleted runs the expectation after the CR is deleted from the cluster.
	AfterCRDeleted E2EAfter = "cr-deleted"
)

// ValidAfterValues is the set of valid values for E2EExpectation.After.
var ValidAfterValues = []E2EAfter{AfterSetupComplete, AfterCRApplied, AfterCRDeleted}

// E2EExpectation is one named assertion block.
type E2EExpectation struct {
	// Include is a path to a YAML file containing a list of E2EExpectation entries.
	// When set, the file is loaded and its entries are expanded in place of this entry.
	// All other fields must be empty when include is set.
	// Relative paths resolve from the e2e.yaml directory.
	Include string `yaml:"include,omitempty"`

	// Name is printed in the results table.
	Name string `yaml:"name"`
	// After is the lifecycle event that triggers this expectation.
	// Valid values: "setup-complete", "cr-applied", "cr-deleted".
	After E2EAfter `yaml:"after"`
	// Timeout is the maximum time to wait for the expectation to pass.
	Timeout string `yaml:"timeout"` // e.g. "60s"
	// Wait is an optional duration to sleep before the polling loop starts.
	// Useful when the previous step needs time to take effect before assertions
	// should begin — e.g. waiting for enough reconcile cycles to accumulate.
	// Must be a valid Go duration string (e.g. "10s", "30s").
	Wait string `yaml:"wait,omitempty"`

	// Resources is a unified list of resource checks across any kind.
	Resources []E2EResourceCheck `yaml:"resources,omitempty"`

	// Commands are shell commands checked in the same polling loop as resources.
	Commands []E2ECommand `yaml:"commands,omitempty"`

	// Kubectl is the structured kubectl DSL block.
	// An alternative to commands: for common kubectl operations — get, logs,
	// describe, exec, port-forward. Compiles to kubectl invocations internally.
	// Use commands: for anything that doesn't fit a subcommand.
	Kubectl *E2EKubectl `yaml:"kubectl,omitempty"`

	// External makes the calls a Blueprint's external: makes — HTTP,
	// Prometheus, Postgres, Redis, Mongo, Kafka — from the machine running the
	// test, and asserts on what they return. The step passes when every call
	// succeeds and its assertions hold.
	External []E2EExternalCheck `yaml:"external,omitempty"`

	// When / Or gate this expectation using the same []Condition type as
	// blueprint when:/or:. An expectation whose conditions do not pass is
	// skipped — not failed. Functions declared in spec.functions are available as
	// template expressions: field: '{{ inBusinessHours }}'.
	// Empty blocks always pass.
	When []Condition `yaml:"when,omitempty"`
	// Or — at least one condition must pass, alongside When.
	Or []Condition `yaml:"or,omitempty"`

	// OnFailure declares diagnostic operations to run immediately when this
	// specific expectation fails. Runs before moving to the next expectation.
	OnFailure *E2EOnFailure `yaml:"onFailure,omitempty"`

	// Finally declares operations to run after this expectation completes —
	// whether it passed, failed or was skipped — before the next expectation
	// runs. Operations are never asserted on; failures are printed, not
	// counted. Use it to undo mutations the step made to files or cluster
	// state.
	Finally *E2EFinally `yaml:"finally,omitempty"`

	// WorkDir is the directory used to resolve relative file paths in this
	// expectation. Set by ExpandIncludes to the directory of the include
	// file; empty means use the e2e.yaml directory.
	WorkDir string `yaml:"-"`

	// Env declares environment variables available to this expectation.
	// Overrides variables declared in the parent E2E spec.
	Env map[string]string `yaml:"env,omitempty"`
}

// E2EExpectationScope is the per-expectation input bundle: what verify and its checks need
// for one expectation, without threading six parameters through every call.
type E2EExpectationScope struct {
	Expectation E2EExpectation
	WorkDir     string
	CS          kubernetes.Interface
	Cfg         *rest.Config
	Mutated     *bool
	Env         EnvResolver
}

// Validate checks every expectation's own fields — the ones not covered by
// the per-operation validators (kubectl, external, resources, commands).
// Per-operation validation is separate: ValidateKubectl and ValidateExternal
// walk the same list with their own locations.
func (s *E2ESpec) Validate() []error {
	var errs []error
	for i, e := range s.Expect {
		loc := fmt.Sprintf("expect[%d] %q", i, e.Name)

		if e.Include != "" {
			errs = append(errs, fmt.Errorf("%s: include must be expanded before validation", loc))
		}
		if e.Name == "" {
			errs = append(errs, fmt.Errorf("%s: name is required", loc))
		}

		if e.After != "" {
			valid := false
			for _, v := range ValidAfterValues {
				if e.After == v {
					valid = true
					break
				}
			}
			if !valid {
				errs = append(errs, fmt.Errorf("%s: after %q is not valid — valid values: %v", loc, e.After, ValidAfterValues))
			}
		}

		if e.Timeout != "" {
			if _, err := core.ParseTimeDuration(e.Timeout); err != nil {
				errs = append(errs, fmt.Errorf("%s: timeout %q is not a valid duration: %w", loc, e.Timeout, err))
			}
		}
		if e.Wait != "" {
			if _, err := core.ParseTimeDuration(e.Wait); err != nil {
				errs = append(errs, fmt.Errorf("%s: wait %q is not a valid duration: %w", loc, e.Wait, err))
			}
		}

		hasOp := len(e.Resources) > 0 ||
			len(e.Commands) > 0 ||
			e.Kubectl != nil ||
			len(e.External) > 0
		if !hasOp {
			errs = append(errs, fmt.Errorf("%s: at least one of resources, commands, kubectl, or external is required", loc))
		}

		if e.OnFailure != nil && e.OnFailure.Empty() {
			errs = append(errs, fmt.Errorf("%s: onFailure block is empty", loc))
		}
		if e.Finally != nil && e.Finally.Empty() {
			errs = append(errs, fmt.Errorf("%s: finally block is empty", loc))
		}
	}
	return errs
}

// E2EResourceCheck asserts the state of any Kubernetes resource.
// Set kind to: Deployment, Service, Secret, ConfigMap.
type E2EResourceCheck struct {
	// Kind is the resource kind: Deployment, Service, Secret, ConfigMap.
	Kind string `yaml:"kind"`
	// Name is the resource name. Empty means any resource of this kind in the namespace.
	Name string `yaml:"name,omitempty"`
	// Namespace to look in. Empty means "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Count asserts the exact number of matching resources. nil means "at least 1".
	// Set to 0 to assert none exist (cleanup check).
	Count *int `yaml:"count,omitempty"`
	// Ready asserts at least one available replica (Deployments only).
	Ready bool `yaml:"ready,omitempty"`
}

// E2ECommand runs a shell command and asserts its result.
type E2ECommand struct {
	// Run is a shell command string executed via sh -c.
	Run string `yaml:"run"`
	// ExitCode is the expected exit code. Default 0 (success).
	// Set to non-zero to assert the command must fail — useful for
	// admission webhook rejection tests.
	ExitCode int `yaml:"exitCode,omitempty"`

	// Extractor selects one value from a command's output before assertions run.
	Extractor `yaml:",inline"`

	// JSON asserts on several values in JSON output in one block. In-process.
	// Runs after JQ extraction if set
	// (JQ output must itself be valid JSON when JSON is present).
	JSON JSONAssertions `yaml:"json,omitempty"`

	E2EAssertions `yaml:",inline"`
}

// E2EKubectl is the structured kubectl DSL block.
// Sits alongside resources: and commands: in each expect entry.
// Each subcommand maps directly to the kubectl command people already know.
type E2EKubectl struct {
	// Get asserts field values on Kubernetes resources.
	// Generates: kubectl get <kind> <name> -n <ns> -o jsonpath='<field>'
	Get []E2EKubectlGet `yaml:"get,omitempty"`
	// Logs asserts container log output.
	// Generates: kubectl logs -n <ns> -l <selector> -c <container> --since=<since>
	Logs []E2EKubectlLogs `yaml:"logs,omitempty"`
	// Describe asserts kubectl describe output — useful for events and conditions.
	// Generates: kubectl describe <kind> <name> -n <ns>
	Describe []E2EKubectlDescribe `yaml:"describe,omitempty"`
	// Exec runs a command inside a running container and asserts its output.
	// Generates: kubectl exec -n <ns> <pod> -c <container> -- <command>
	Exec []E2EKubectlExec `yaml:"exec,omitempty"`
	// PortForward opens a port-forward to a service or pod, makes an HTTP request,
	// and asserts the response. EnsureCurl is called automatically when any entry
	// declares a path. Generates: kubectl port-forward + curl.
	PortForward []E2EKubectlPortForward `yaml:"port-forward,omitempty"`
	// Apply applies manifests from a file path or inline YAML/JSON content.
	// Generates: kubectl apply -f <file>  or  echo '<inline>' | kubectl apply -f -
	Apply []E2EKubectlApply `yaml:"apply,omitempty"`
	// Delete deletes resources by file path or by resource identity.
	// Generates: kubectl delete -f <file>  or  kubectl delete <kind> <name> -n <ns>
	Delete []E2EKubectlDelete `yaml:"delete,omitempty"`
	// Patch patches a Kubernetes resource with a merge, strategic, or JSON patch.
	// Generates: kubectl patch <kind> <name> -n <ns> --type=<type> -p '<patch>'
	Patch []E2EKubectlPatch `yaml:"patch,omitempty"`
	// Events lists Kubernetes events for a specific resource and asserts the output.
	// Generates: kubectl events --for=<kind>/<name> -n <ns>
	Events []E2EKubectlEvents `yaml:"events,omitempty"`
	// Auth checks permissions via kubectl auth can-i and asserts the result.
	// Generates: kubectl auth can-i <verb> <resource> [-n <ns>] [--as <as>]
	Auth []E2EKubectlAuth `yaml:"auth,omitempty"`
	// Cp copies a file out of a container and asserts its content.
	// Generates: kubectl cp <ns>/<pod>:<src> <tempfile>
	Cp []E2EKubectlCp `yaml:"cp,omitempty"`
	// Top queries live CPU and memory usage via kubectl top and asserts the output.
	// Requires metrics-server; installed automatically when entries are present.
	// Generates: kubectl top <kind> [-n <ns>] [<name> | -l <selector>] [--containers]
	Top []E2EKubectlTop `yaml:"top,omitempty"`

	// Restart triggers a rollout restart of a Deployment, StatefulSet, or DaemonSet.
	// Generates: kubectl rollout restart <kind>/<name> -n <ns>
	// Waits for the rollout to complete when ready: true (default).
	// The expect step's timeout governs how long to wait.
	Restart []E2EKubectlRestart `yaml:"restart,omitempty"`

	// Scale sets the replica count on a Deployment, StatefulSet, or ReplicaSet.
	// Generates: kubectl scale <kind>/<name> --replicas=<n> -n <ns>
	// Waits for the rollout to complete when ready: true (default).
	// The expect step's timeout governs how long to wait.
	Scale []E2EKubectlScale `yaml:"scale,omitempty"`
}

// E2EExternalCheck is one call an e2e step makes and asserts on. It is an
// ExternalCallSpec, so every protocol, auth.secretRef and include work as in a
// Blueprint: include: is replaced by its file's calls:, which may carry their
// own jq and assertions — a Blueprint's calls file is one without them.
// Templates see spec.functions and .inrun, with .inrun.inPod false — the call
// is made from the machine running the test.
type E2EExternalCheck struct {
	ExternalCallSpec `yaml:",inline"`
	// JQ in Extractor is a jq expression over the call's result — what a Blueprint reads as
	// .external.<name>: status, body and json (the decoded body) for HTTP;
	// result, rows and rowCount for Postgres; result for the other protocols.
	// Without it, the assertions apply to the body for HTTP and to result
	// for the other protocols.
	// Extractor selects one value from a command's output before assertions run.
	Extractor `yaml:",inline"`
	// JSON asserts on several values in JSON output in one block. In-process.
	// Runs after JQ extraction if set
	// (JQ output must itself be valid JSON when JSON is present).
	JSON JSONAssertions `yaml:"json,omitempty"`
	// Assertions on the extracted value. With none, the call succeeding is
	// the check.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlGet asserts a field value on a Kubernetes resource.
type E2EKubectlGet struct {
	// Kind is the Kubernetes resource kind (e.g. Deployment, ConfigMap).
	Kind string `yaml:"kind"`
	// Name is the resource name.
	Name string `yaml:"name"`
	// Namespace to look in. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Field is a jsonpath expression to extract before asserting.
	// e.g. .spec.template.spec.containers[0].resources.requests.cpu
	Field string `yaml:"field,omitempty"`
	// Format outputs the full resource as yaml or json for looser substring assertions.
	// Ignored when field is set. Use with jq (json) or yq (yaml) for structured extraction.
	Format string `yaml:"format,omitempty"` // yaml | json
	// Extractor selects one value from a command's output before assertions run.
	Extractor `yaml:",inline"`
	// JSON asserts on several values in JSON output in one block. In-process.
	// Runs after JQ extraction if set
	// (JQ output must itself be valid JSON when JSON is present).
	JSON JSONAssertions `yaml:"json,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlLogs asserts container log output.
type E2EKubectlLogs struct {
	// Name is the pod name. Use LabelSelector or LeaderElection instead.
	Name string `yaml:"name,omitempty"`
	// LabelSelector selects pods by label (e.g. "app=my-service").
	LabelSelector string `yaml:"labelSelector,omitempty"`
	// LeaderElection resolves the target pod from a Kubernetes Lease holder.
	// Mutually exclusive with Name and LabelSelector.
	LeaderElection *E2EKubectlLeaderElection `yaml:"leaderElection,omitempty"`
	// Namespace to look in. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Container name. Defaults to the first container.
	Container string `yaml:"container,omitempty"`
	// Since limits log output to the given duration (e.g. "30s", "2m").
	Since string `yaml:"since,omitempty"`
	// Extractor selects one value from a command's output before assertions run.
	Extractor `yaml:",inline"`
	// JSON asserts on several values in JSON output in one block. In-process.
	// Runs after JQ extraction if set
	// (JQ output must itself be valid JSON when JSON is present).
	JSON JSONAssertions `yaml:"json,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlDescribe asserts kubectl describe output.
// Useful for checking events, conditions, and resource details.
type E2EKubectlDescribe struct {
	// Kind is the Kubernetes resource kind.
	Kind string `yaml:"kind"`
	// Name is the resource name. Use LabelSelector to match by label instead.
	Name string `yaml:"name,omitempty"`
	// LabelSelector selects resources by label.
	LabelSelector string `yaml:"labelSelector,omitempty"`
	// Namespace to look in. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlExec runs a command inside a running container and asserts its output.
type E2EKubectlExec struct {
	// Name is the pod name. Use LabelSelector or LeaderElection to match instead.
	Name string `yaml:"name,omitempty"`
	// LabelSelector selects the pod by label.
	LabelSelector string `yaml:"labelSelector,omitempty"`
	// LeaderElection resolves the target pod from a Kubernetes Lease holder.
	// Mutually exclusive with name and labelSelector.
	LeaderElection *E2EKubectlLeaderElection `yaml:"leaderElection,omitempty"`
	// Namespace to look in. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Container name. Defaults to the first container.
	Container string `yaml:"container,omitempty"`
	// Command is the command to execute inside the container.
	Command []string `yaml:"command"`
	// Extractor selects one value from a command's output before assertions run.
	Extractor `yaml:",inline"`
	// JSON asserts on several values in JSON output in one block. In-process.
	// Runs after JQ extraction if set
	// (JQ output must itself be valid JSON when JSON is present).
	JSON JSONAssertions `yaml:"json,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlApply applies one or more manifests during an expect checkpoint.
// Use file to reference a path on disk or inline to embed the manifest directly.
// kubectl apply is idempotent so re-running inside the poll loop is safe.
type E2EKubectlApply struct {
	// File is a path to a manifest file. Relative paths resolve from the e2e.yaml directory.
	// Generates: kubectl apply -f <file>
	File string `yaml:"file,omitempty"`
	// Inline is a raw YAML or JSON manifest string applied via stdin.
	// Generates: echo '<inline>' | kubectl apply -f -
	Inline string `yaml:"inline,omitempty"`
	// Namespace overrides the namespace for resources that don't declare one.
	Namespace string `yaml:"namespace,omitempty"`

	// ExitCode is the expected exit code. Default 0 (success). Set to
	// non-zero to assert the apply must be rejected — e.g. an admission
	// webhook denial — instead of treating any failure as a broken test.
	// Same convention as E2ECommand.ExitCode.
	ExitCode int `yaml:"exitCode,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlDelete deletes resources during an expect checkpoint.
// Use file to delete all resources in a manifest, kind+name for a single resource,
// or leaderElection to delete the current leader pod by resolving the Lease holder.
type E2EKubectlDelete struct {
	// File is a path to a manifest file. All resources in the file are deleted.
	// Relative paths resolve from the e2e.yaml directory.
	// Generates: kubectl delete -f <file>
	File string `yaml:"file,omitempty"`
	// Kind is the Kubernetes resource kind. Used with name for single-resource deletion.
	Kind string `yaml:"kind,omitempty"`
	// Name is the resource name.
	Name string `yaml:"name,omitempty"`
	// Namespace to target. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// LeaderElection resolves the pod to delete by reading the holder of a Kubernetes Lease.
	// Mutually exclusive with file, kind, and name.
	LeaderElection *E2EKubectlLeaderElection `yaml:"leaderElection,omitempty"`
	// IgnoreNotFound silences errors when the resource does not exist.
	IgnoreNotFound bool `yaml:"ignoreNotFound,omitempty"`
}

// E2EKubectlPatch patches a Kubernetes resource in-place.
// Useful for triggering state transitions (e.g. updating a field to drive a state machine).
type E2EKubectlPatch struct {
	// Kind is the Kubernetes resource kind (e.g. Deployment, MyResource).
	Kind string `yaml:"kind"`
	// Name is the resource name.
	Name string `yaml:"name"`
	// Namespace to target. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Type is the patch strategy: merge (default), strategic, or json.
	Type string `yaml:"type,omitempty"` // merge | strategic | json
	// Patch is the patch content as a YAML or JSON string.
	Patch string `yaml:"patch"`
}

// E2EKubectlEvents lists Kubernetes events for a specific resource and asserts
// the output. Useful for verifying that the operator emitted expected events
// (e.g. Reconciled, BackOff) or that no error events occurred.
type E2EKubectlEvents struct {
	// Kind is the Kubernetes resource kind to filter events for.
	Kind string `yaml:"kind"`
	// Name is the resource name.
	Name string `yaml:"name"`
	// Namespace to look in. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlAuth checks permissions via kubectl auth can-i and asserts the result.
// Useful for verifying that the operator created the correct RBAC resources.
type E2EKubectlAuth struct {
	// Verb is the action to check (e.g. get, list, create, delete).
	Verb string `yaml:"verb"`
	// Resource is the Kubernetes resource type (e.g. pods, deployments, secrets).
	Resource string `yaml:"resource"`
	// Namespace scopes the check. Omit for cluster-scoped checks.
	Namespace string `yaml:"namespace,omitempty"`
	// As is the user or service account to impersonate.
	// Use the full service account form: system:serviceaccount:<ns>:<name>
	As string `yaml:"as,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlCp copies a file out of a running container and asserts its content.
// Resolves the pod by name or label selector, copies the file to a temp path,
// reads it, applies assertions, and cleans up.
type E2EKubectlCp struct {
	// Name is the pod name. Use LabelSelector to match by label instead.
	Name string `yaml:"name,omitempty"`
	// LabelSelector selects the pod by label (e.g. "app=my-service").
	LabelSelector string `yaml:"labelSelector,omitempty"`
	// Namespace to look in. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Container name. Defaults to the first container.
	Container string `yaml:"container,omitempty"`
	// Src is the path inside the container to copy from.
	Src string `yaml:"src"`
	// Extractor selects one value from a command's output before assertions run.
	Extractor `yaml:",inline"`
	// JSON asserts on several values in JSON output in one block. In-process.
	// Runs after JQ extraction if set
	// (JQ output must itself be valid JSON when JSON is present).
	JSON JSONAssertions `yaml:"json,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlTop queries live CPU and memory usage via kubectl top and asserts
// the output. Requires metrics-server in the cluster; the runner installs it
// automatically (via Helm) when any top entry is present.
type E2EKubectlTop struct {
	// Kind is the resource type to query: "pod" or "node".
	Kind string `yaml:"kind"`
	// Name is the pod or node name. Omit to list all.
	Name string `yaml:"name,omitempty"`
	// LabelSelector filters pods by label (e.g. "app=my-service"). Pods only.
	LabelSelector string `yaml:"labelSelector,omitempty"`
	// Namespace to query. Applies to pods only. Default: "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Containers shows per-container metrics (--containers). Pods only.
	Containers bool `yaml:"containers,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

// E2EKubectlLeaderElection resolves the target pod by reading the current holder
// from a Kubernetes Lease object. Used by port-forward, logs, delete, and exec
// to pin commands to the elected leader rather than a random replica.
type E2EKubectlLeaderElection struct {
	// Lease is the name of the Kubernetes Lease object that tracks leader election.
	Lease string `yaml:"lease"`
	// Namespace of the lease. Defaults to the port-forward namespace.
	Namespace string `yaml:"namespace,omitempty"`
}

// E2EKubectlPortForward opens a port-forward to a service or pod, makes an HTTP
// request via curl, and asserts the response. The port-forward lifecycle
// (background process, wait for port, cleanup) is handled by the runner.
// EnsureCurl is called automatically when any entry declares a path.
type E2EKubectlPortForward struct {
	// Service is the service name to port-forward to. Optional when leaderElection is set.
	Service string `yaml:"service,omitempty"`
	// Pod is the pod name to port-forward to. Optional when leaderElection is set.
	Pod string `yaml:"pod,omitempty"`
	// Namespace to look in. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// LeaderElection resolves the port-forward target from a Kubernetes Lease object.
	// When set, service and pod are ignored — the current lease holder is used instead.
	// The lease namespace defaults to the port-forward namespace.
	LeaderElection *E2EKubectlLeaderElection `yaml:"leaderElection,omitempty"`
	// Port is the service or pod port to forward.
	Port int `yaml:"port"`
	// Path is the HTTP path to request after the port-forward is ready.
	// Triggers EnsureCurl pre-flight when set.
	Path string `yaml:"path,omitempty"`
	// Method is the HTTP method. Defaults to GET.
	Method string `yaml:"method,omitempty"`
	// Headers are added to the request, e.g. Authorization for a token-gated
	// endpoint like the Gateway API. Values go through os.ExpandEnv
	// (same ${VAR} syntax as gateway.api.auth.tokens.token), so a CI
	// secret never needs to be written into the YAML file.
	//
	//	headers:
	//	  Authorization: "Bearer ${INRUN_CI_TOKEN}"
	Headers map[string]string `yaml:"headers,omitempty"`
	// Body is sent as the request body for methods that accept one (POST,
	// PUT, PATCH). Ignored for GET/HEAD/DELETE. Goes through os.ExpandEnv,
	// same as Headers.
	Body string `yaml:"body,omitempty"`
	// BodyFile is a path to read the request body from. Relative paths
	// resolve against the E2E file's directory. Goes through os.ExpandEnv.
	// Mutually exclusive with Body.
	BodyFile string `yaml:"bodyFile,omitempty"`
	// ContentYAML, when true, sets Content-Type: application/yaml unless a
	// Content-Type header is already declared. Convenience for posting a
	// full CR manifest read from BodyFile.
	ContentYAML bool `yaml:"contentYAML,omitempty"`
	// Wait is an optional duration to sleep after the port-forward is ready
	// but before the curl request is made. Useful when the endpoint needs a
	// moment to stabilise after the connection is established.
	// Must be a valid Go duration string (e.g. "2s", "5s").
	Wait string `yaml:"wait,omitempty"`
	// Extractor selects one value from a command's output before assertions run.
	Extractor `yaml:",inline"`
	// JSON asserts on several values in JSON output in one block. In-process.
	// Runs after JQ extraction if set
	// (JQ output must itself be valid JSON when JSON is present).
	JSON JSONAssertions `yaml:"json,omitempty"`
	// Assertions on the output: the comparisons conditions use.
	E2EAssertions `yaml:",inline"`
	// StatusCode asserts the HTTP response status code. When set, the response
	// body is discarded and only the numeric status is checked (e.g. 404, 200).
	// Cannot be combined with JQ, YQ, Equals, or OutputContains.
	StatusCode int `yaml:"statusCode,omitempty"`
}

// E2EKubectlRestart triggers a rollout restart of a Deployment, StatefulSet, or DaemonSet.
type E2EKubectlRestart struct {
	// Kind is the resource kind: Deployment, StatefulSet, or DaemonSet.
	Kind string `yaml:"kind"`
	// Name is the resource name.
	Name string `yaml:"name"`
	// Namespace to target. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Ready waits for the rollout to complete after the restart. Defaults to true.
	Ready *bool `yaml:"ready,omitempty"`
}

// E2EKubectlScale sets the replica count on a Deployment, StatefulSet, or ReplicaSet.
type E2EKubectlScale struct {
	// Kind is the resource kind: Deployment, StatefulSet, or ReplicaSet.
	Kind string `yaml:"kind"`
	// Name is the resource name.
	Name string `yaml:"name"`
	// Namespace to target. Defaults to "default".
	Namespace string `yaml:"namespace,omitempty"`
	// Replicas is the desired replica count.
	Replicas int `yaml:"replicas"`
	// Ready waits for the rollout to complete after scaling. Defaults to true.
	Ready *bool `yaml:"ready,omitempty"`
}
