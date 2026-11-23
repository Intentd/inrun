package types

import (
	"fmt"
	"github.com/intentd/jig"

	"github.com/intentd/kel/core"
	"github.com/intentd/tools/schemagen"
	"github.com/invopop/jsonschema"
	"gopkg.in/yaml.v3"
)

// Prepare is the second phase of a run: the test's own dependencies and the
// workload under test.
//
// Ecosystem installs infrastructure the workload needs (Postgres, RabbitMQ,
// Redis) from the data-services chart. Apply, Helm and Wait install the
// workload and anything it needs that ecosystem does not cover.
// Shorthand — a plain list of strings is equivalent to setup.apply:
//
//	prepare:
//	  - ./prereqs/secret.yaml
//
// Struct form with per-entry waits:
//
//	prepare:
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
type Prepare struct {
	// Ecosystem declares infrastructure dependencies and their Secrets.
	Ecosystem *Ecosystem `yaml:"ecosystem,omitempty"`

	Provision `yaml:",inline"`
}

// UnmarshalYAML allows setup: to be written as either a plain list of strings (setup.apply) or a full struct.
func (s *Prepare) UnmarshalYAML(value *yaml.Node) error {
	if entries := jig.ApplyShorthand(value); len(entries) > 0 {
		s.Apply = append(s.Apply, entries...)
		return nil
	}
	type plain Prepare
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return strictUnmarshal(raw, (*plain)(s))
}

// JSONSchemaExtend accepts a list of paths (shorthand for apply) beside the
// object, as UnmarshalYAML does.
func (Prepare) JSONSchemaExtend(s *jsonschema.Schema) {
	schemagen.Accept(s, &jsonschema.Schema{Type: "array", Items: schemagen.Str("Path to a manifest"), Description: "Shorthand for apply"})
}

// DefaultE2EClusterName is the kind cluster an e2e run creates when its
// environment names none.
const DefaultE2EClusterName = "inrun-e2e"

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

type E2EMeta struct {
	// Name — shown in the results.
	Name string `yaml:"name"`
	// Description — what the test checks.
	Description string `yaml:"description,omitempty"`
}

type E2ESpec struct {
	// Core operator spec — the three files that define every Inrun operator.

	// Blueprint is the path to the blueprint.yaml file. Without one the test
	// is plain Kubernetes: Inrun generates no bundle and installs nothing.
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

	// Init uses an example pack — for Inrun's own CI.
	Init *E2EInit `yaml:"init,omitempty"`

	// Environment is the cluster the run executes against and the
	// infrastructure it needs before the test's own dependencies are
	// installed: the cluster itself, the images to build and load, and
	// cluster furniture (namespaces, an ingress controller, a storage class)
	// that the prepare phase's charts and manifests expect to exist.
	Environment *E2EEnvironment `yaml:"environment,omitempty"`

	// Prepare is the test's own dependencies and the workload under test:
	// ecosystem infrastructure (Postgres, RabbitMQ, Redis) and the manifests
	// and charts that install the workload.
	Prepare *Prepare `yaml:"prepare,omitempty"`

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
	Expect E2EExpectations `yaml:"expect"`

	// OnFailure declares operations to run once for the suite when any
	// expectation, or the setup before them, fails. It is a post block: the
	// same commands, kubectl and external as a step, run once, with output
	// printed and failures reported as warnings.
	OnFailure *E2EOnFailure `yaml:"onFailure,omitempty"`

	// Finally declares operations to run once for the suite at its end —
	// whether it passed, failed or stopped early, setup failures included —
	// before teardown. It is a post block, like an expectation's finally: its
	// failures are warnings, never the result. Use it to undo what the suite
	// changed outside the cluster, such as a file it wrote.
	Finally *E2EFinally `yaml:"finally,omitempty"`

	// Runtime controls runtime-specific behavior during the e2e test.
	Runtime *E2ERuntimeConfig `yaml:"runtime,omitempty"`

	// Env declares environment variables available to all expectations in this
	// e2e spec. Values support variable expansion.
	Env map[string]string `yaml:"env,omitempty"`
}

// ResolveEnv returns an EnvResolver for expect, scoped to this spec.
func (s *E2ESpec) ResolveEnv(expect E2EExpectation) EnvResolver {
	var specEnv map[string]string
	if s != nil {
		specEnv = s.Env
	}
	return expect.ResolveEnv(specEnv)
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

// EffectiveStop reports whether a failure under this spec should stop the run
// after diagnostics. Spec-level onFailure.stop is the default; an expectation
// that sets its own onFailure.stop overrides it.
func (s *E2ESpec) EffectiveStop() bool {
	if s.OnFailure != nil && s.OnFailure.Stop != nil {
		return *s.OnFailure.Stop
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
	var specOnFailure *E2EOnFailure
	if spec != nil {
		specOnFailure = spec.OnFailure
	}
	return exp.EffectiveStop(specOnFailure)
}

// E2EInit selects a built-in example pack as the test source.
type E2EInit struct {
	// Pack — the example pack, as in inrun init --pack.
	Pack string `yaml:"pack"`
	// Example — the example inside the pack to test.
	Example string `yaml:"example"`
}

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
