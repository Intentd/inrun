package kubectl

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/types"
)

// Validate checks every kubectl: block across the expectations: each step's,
// and each one's onFailure and finally post blocks.
func Validate(expects types.E2EExpectations, baseDir string) []error {
	var errs []error
	for i, exp := range expects {
		loc := fmt.Sprintf("expect[%d] %q", i, exp.Name)
		dir := exp.Dir(baseDir) // where the step's files resolve when it runs
		errs = append(errs, ValidateBlock(loc, exp.Kubectl, dir, false)...)
		if exp.OnFailure != nil {
			errs = append(errs, ValidateBlock(loc+" onFailure", exp.OnFailure.Kubectl, dir, true)...)
		}
		if exp.Finally != nil {
			errs = append(errs, ValidateBlock(loc+" finally", exp.Finally.Kubectl, dir, true)...)
		}
	}
	return errs
}

// ValidateBlock checks one kubectl: block; loc prefixes each error. A post
// block (onFailure, finally) is checked by the same rules except the
// step-only ones: a read there may only print, so it needs no assertion, and
// a get or describe may cover every object of its kind.
func ValidateBlock(loc string, k *types.E2EKubectl, baseDir string, post bool) []error {
	if k == nil {
		return nil
	}
	at := func(sub string, idx int) string {
		return fmt.Sprintf("%s kubectl.%s[%d]", loc, sub, idx)
	}
	var errs []error
	for j, g := range k.Get {
		errs = append(errs, validateKubectlGet(at("get", j), g)...)
	}
	for j, l := range k.Logs {
		errs = append(errs, validateKubectlLogs(at("logs", j), l)...)
	}
	for j, d := range k.Describe {
		errs = append(errs, validateKubectlDescribe(at("describe", j), d)...)
	}
	for j, e := range k.Exec {
		errs = append(errs, validateKubectlExec(at("exec", j), e)...)
	}
	for j, p := range k.PortForward {
		errs = append(errs, validateKubectlPortForward(at("port-forward", j), p, baseDir)...)
	}
	for j, a := range k.Apply {
		errs = append(errs, validateKubectlApply(at("apply", j), a, baseDir)...)
	}
	for j, d := range k.Delete {
		errs = append(errs, validateKubectlDelete(at("delete", j), d, baseDir)...)
	}
	for j, p := range k.Patch {
		errs = append(errs, validateKubectlPatch(at("patch", j), p, baseDir)...)
	}
	for j, e := range k.Events {
		errs = append(errs, validateKubectlEvents(at("events", j), e)...)
	}
	for j, a := range k.Auth {
		errs = append(errs, validateKubectlAuth(at("auth", j), a)...)
	}
	for j, c := range k.Cp {
		errs = append(errs, validateKubectlCp(at("cp", j), c)...)
	}
	for j, t := range k.Top {
		errs = append(errs, validateKubectlTop(at("top", j), t)...)
	}
	for j, r := range k.Restart {
		errs = append(errs, validateKubectlRestart(at("restart", j), r)...)
	}
	for j, s := range k.Scale {
		errs = append(errs, validateKubectlScale(at("scale", j), s)...)
	}
	if !post {
		return errs
	}
	kept := errs[:0]
	for _, err := range errs {
		if !errors.As(err, new(stepOnly)) {
			kept = append(kept, err)
		}
	}
	return kept
}

// stepOnly marks a rule only a step's reads follow: an assertion, which a
// post block's reads may go without, and a name or selector on a get or
// describe, which a post block may leave out to cover every object. ValidateBlock drops these
// errors for a post block.
type stepOnly struct{ error }

func (e stepOnly) Unwrap() error { return e.error }

// errNeedsAssertion is the step-only error for a read that asserts nothing.
func errNeedsAssertion(loc, msg string) error {
	return stepOnly{fmt.Errorf("%s: %s", loc, msg)}
}

func validateKubectlGet(loc string, g types.E2EKubectlGet) []error {
	var errs []error

	if g.Kind == "" {
		errs = append(errs, fmt.Errorf("%s: kind is required", loc))
	}
	if g.Name == "" && g.LabelSelector == "" {
		errs = append(errs, stepOnly{fmt.Errorf("%s: name or labelSelector is required", loc)})
	}
	if g.Name != "" && g.LabelSelector != "" {
		errs = append(errs, fmt.Errorf("%s: name and labelSelector are mutually exclusive", loc))
	}

	objectLevel := g.Assert != nil &&
		((g.Assert.Exists != nil && !*g.Assert.Exists) ||
			(g.Assert.NotExists != nil && *g.Assert.NotExists))

	if !hasAssertion(g.TextAssertion, g.Assert, g.JSON) && !objectLevel {
		errs = append(errs, errNeedsAssertion(loc, assertionListMsg))
	}

	if g.Format != "" && g.Format != "json" && g.Format != "yaml" {
		errs = append(errs, fmt.Errorf("%s: format must be json or yaml, got %q", loc, g.Format))
	}
	if g.JQ != "" && g.Format != "json" {
		errs = append(errs, fmt.Errorf("%s: jq requires format: json", loc))
	}
	if g.YQ != "" && g.Format != "yaml" {
		errs = append(errs, fmt.Errorf("%s: yq requires format: yaml", loc))
	}
	if g.JSONPath != "" && !hasAssertion(g.TextAssertion, g.Assert, g.JSON) {
		errs = append(errs, errNeedsAssertion(loc, "jsonPath requires an assertion"))
	}

	return errs
}

func validateKubectlLogs(loc string, l types.E2EKubectlLogs) []error {
	var errs []error
	if l.LeaderElection != nil {
		if l.Name != "" || l.LabelSelector != "" {
			errs = append(errs, fmt.Errorf("%s: leaderElection is mutually exclusive with name and labelSelector", loc))
		}
		if l.LeaderElection.Lease == "" {
			errs = append(errs, fmt.Errorf("%s: leaderElection.lease is required", loc))
		}
	} else if l.Name == "" && l.LabelSelector == "" {
		errs = append(errs, fmt.Errorf("%s: name, labelSelector, or leaderElection is required", loc))
	}
	if l.Name != "" && l.LabelSelector != "" {
		errs = append(errs, fmt.Errorf("%s: name and labelSelector are mutually exclusive", loc))
	}
	if l.Pods != "" {
		if !slices.Contains(types.ValidLogsPodsValues, l.Pods) {
			errs = append(errs, fmt.Errorf("%s: pods %q is not valid — valid values: %v", loc, l.Pods, types.ValidLogsPodsValues))
		} else if l.LabelSelector == "" {
			errs = append(errs, fmt.Errorf("%s: pods needs labelSelector: it checks each selected pod's log", loc))
		}
	}
	if !hasAssertion(l.TextAssertion, l.Assert) {
		errs = append(errs, errNeedsAssertion(loc, assertionListMsg))
	}
	return errs
}

func validateKubectlDescribe(loc string, d types.E2EKubectlDescribe) []error {
	var errs []error
	if d.Kind == "" {
		errs = append(errs, fmt.Errorf("%s: kind is required", loc))
	}
	if d.Name == "" && d.LabelSelector == "" {
		errs = append(errs, stepOnly{fmt.Errorf("%s: name or labelSelector is required", loc)})
	}
	if d.Name != "" && d.LabelSelector != "" {
		errs = append(errs, fmt.Errorf("%s: name and labelSelector are mutually exclusive", loc))
	}
	if !hasAssertion(d.TextAssertion, d.Assert) {
		errs = append(errs, errNeedsAssertion(loc, assertionListMsg))
	}
	return errs
}

func validateKubectlExec(loc string, e types.E2EKubectlExec) []error {
	var errs []error
	if e.LeaderElection != nil {
		if e.Name != "" || e.LabelSelector != "" {
			errs = append(errs, fmt.Errorf("%s: leaderElection is mutually exclusive with name and labelSelector", loc))
		}
		if e.LeaderElection.Lease == "" {
			errs = append(errs, fmt.Errorf("%s: leaderElection.lease is required", loc))
		}
	} else if e.Name == "" && e.LabelSelector == "" {
		errs = append(errs, fmt.Errorf("%s: name, labelSelector, or leaderElection is required", loc))
	}
	if e.Name != "" && e.LabelSelector != "" {
		errs = append(errs, fmt.Errorf("%s: name and labelSelector are mutually exclusive", loc))
	}
	if len(e.Command) == 0 {
		errs = append(errs, fmt.Errorf("%s: command is required", loc))
	}
	if !hasAssertion(e.TextAssertion, e.Assert) {
		errs = append(errs, errNeedsAssertion(loc, assertionListMsg))
	}
	return errs
}

func validateKubectlPortForward(loc string, p types.E2EKubectlPortForward, baseDir string) []error {
	var errs []error
	if p.LeaderElection != nil {
		if p.LeaderElection.Lease == "" {
			errs = append(errs, fmt.Errorf("%s: leaderElection.lease is required", loc))
		}
	} else if p.Service == "" && p.Pod == "" {
		errs = append(errs, fmt.Errorf("%s: service, pod, or leaderElection is required", loc))
	}
	if p.Service != "" && p.Pod != "" {
		errs = append(errs, fmt.Errorf("%s: service and pod are mutually exclusive", loc))
	}
	if p.Port <= 0 {
		errs = append(errs, fmt.Errorf("%s: port must be > 0", loc))
	}
	hasAny := hasAssertion(p.TextAssertion, p.Assert)
	if hasAny && p.Path == "" {
		errs = append(errs, fmt.Errorf("%s: path is required when assertions are set", loc))
	}

	// body / bodyFile
	switch {
	case p.Body != "" && p.BodyFile != "":
		errs = append(errs, fmt.Errorf("%s: body and bodyFile are mutually exclusive", loc))
	case p.Body != "" || p.BodyFile != "":
		if p.Path == "" {
			errs = append(errs, fmt.Errorf("%s: path is required when a body is set", loc))
		}
		switch strings.ToUpper(p.Method) {
		case "GET", "HEAD", "DELETE":
			errs = append(errs, fmt.Errorf("%s: %s does not accept a body", loc, strings.ToUpper(p.Method)))
		}
	}

	errs = append(errs, checkFile(loc, "bodyFile", p.BodyFile, baseDir)...)

	// statusCode vs. body assertions
	if p.StatusCode != 0 {
		if p.JQ != "" || p.YQ != "" || p.JSONPath != "" || hasAny {
			errs = append(errs, fmt.Errorf("%s: statusCode cannot be combined with body assertions", loc))
		}
	}
	return errs
}

func validateKubectlDelete(loc string, d types.E2EKubectlDelete, baseDir string) []error {
	var errs []error
	errs = append(errs, checkFile(loc, "file", d.File, baseDir)...)
	if d.LeaderElection != nil {
		if d.File != "" || d.Kind != "" || d.Name != "" {
			errs = append(errs, fmt.Errorf("%s: leaderElection is mutually exclusive with file, kind, and name", loc))
		}
		if d.LeaderElection.Lease == "" {
			errs = append(errs, fmt.Errorf("%s: leaderElection.lease is required", loc))
		}
	} else {
		if d.File == "" && (d.Kind == "" || d.Name == "") {
			errs = append(errs, fmt.Errorf("%s: file or (kind + name) is required", loc))
		}
		if d.File != "" && (d.Kind != "" || d.Name != "") {
			errs = append(errs, fmt.Errorf("%s: file and kind/name are mutually exclusive", loc))
		}
	}
	if d.Timeout != "" {
		if _, err := time.ParseDuration(d.Timeout); err != nil {
			errs = append(errs, fmt.Errorf("%s: timeout %q is not a valid duration: %w", loc, d.Timeout, err))
		}
	}
	return errs
}

func validateKubectlApply(loc string, a types.E2EKubectlApply, baseDir string) []error {
	var errs []error
	errs = append(errs, checkFile(loc, "file", a.File, baseDir)...)
	if a.File == "" && a.Inline == "" {
		errs = append(errs, fmt.Errorf("%s: file or inline is required", loc))
	}
	if a.File != "" && a.Inline != "" {
		errs = append(errs, fmt.Errorf("%s: file and inline are mutually exclusive", loc))
	}
	return errs
}

func validateKubectlPatch(loc string, p types.E2EKubectlPatch, baseDir string) []error {
	var errs []error
	switch {
	case p.File != "" && (p.Kind != "" || p.Name != ""):
		errs = append(errs, fmt.Errorf("%s: file and kind/name are mutually exclusive", loc))
	case p.File != "":
		if p.Namespace != "" {
			errs = append(errs, fmt.Errorf("%s: namespace has no effect with file: each object is patched in the namespace the manifest declares", loc))
		}
		errs = append(errs, checkFile(loc, "file", p.File, baseDir)...)
	default:
		if p.Kind == "" {
			errs = append(errs, fmt.Errorf("%s: kind is required (or file)", loc))
		}
		if p.Name == "" {
			errs = append(errs, fmt.Errorf("%s: name is required (or file)", loc))
		}
	}
	switch {
	case p.Patch != "" && p.PatchFile != "":
		errs = append(errs, fmt.Errorf("%s: patch and patchFile are mutually exclusive", loc))
	case p.Patch == "" && p.PatchFile == "":
		errs = append(errs, fmt.Errorf("%s: patch or patchFile is required", loc))
	case p.PatchFile != "" && p.PatchFile != p.File:
		errs = append(errs, checkFile(loc, "patchFile", p.PatchFile, baseDir)...)
	case p.PatchFile != "":
		// One file as target and patch: kubectl patches every object it names
		// with its first, so it must name exactly one.
		errs = append(errs, checkSingleObject(loc, p.File, baseDir)...)
	}
	if p.Type != "" && p.Type != "merge" && p.Type != "strategic" && p.Type != "json" {
		errs = append(errs, fmt.Errorf("%s: type must be merge, strategic, or json, got %q", loc, p.Type))
	}
	return errs
}

func validateKubectlEvents(loc string, e types.E2EKubectlEvents) []error {
	var errs []error
	if e.Kind == "" {
		errs = append(errs, fmt.Errorf("%s: kind is required", loc))
	}
	if e.Name == "" {
		errs = append(errs, fmt.Errorf("%s: name is required", loc))
	}
	if !hasAssertion(e.TextAssertion, e.Assert) {
		errs = append(errs, errNeedsAssertion(loc, assertionListMsg))
	}
	return errs
}

func validateKubectlAuth(loc string, a types.E2EKubectlAuth) []error {
	var errs []error
	if a.Verb == "" {
		errs = append(errs, fmt.Errorf("%s: verb is required", loc))
	}
	if a.Resource == "" {
		errs = append(errs, fmt.Errorf("%s: resource is required", loc))
	}
	if !hasAssertion(a.TextAssertion, a.Assert) {
		errs = append(errs, errNeedsAssertion(loc, assertionListMsg))
	}
	return errs
}

func validateKubectlCp(loc string, c types.E2EKubectlCp) []error {
	var errs []error
	if c.Name == "" && c.LabelSelector == "" {
		errs = append(errs, fmt.Errorf("%s: name or labelSelector is required", loc))
	}
	if c.Name != "" && c.LabelSelector != "" {
		errs = append(errs, fmt.Errorf("%s: name and labelSelector are mutually exclusive", loc))
	}
	if c.Src == "" {
		errs = append(errs, fmt.Errorf("%s: src is required", loc))
	}
	if !hasAssertion(c.TextAssertion, c.Assert) {
		errs = append(errs, errNeedsAssertion(loc, assertionListMsg))
	}
	return errs
}

func validateKubectlTop(loc string, t types.E2EKubectlTop) []error {
	var errs []error
	if t.Kind == "" {
		errs = append(errs, fmt.Errorf("%s: kind is required (pod or node)", loc))
	} else {
		k := strings.ToLower(t.Kind)
		if k != "pod" && k != "pods" && k != "node" && k != "nodes" {
			errs = append(errs, fmt.Errorf("%s: kind must be pod or node, got %q", loc, t.Kind))
		}
	}
	if t.Name != "" && t.LabelSelector != "" {
		errs = append(errs, fmt.Errorf("%s: name and labelSelector are mutually exclusive", loc))
	}
	if !hasAssertion(t.TextAssertion, t.Assert) {
		errs = append(errs, errNeedsAssertion(loc, assertionListMsg))
	}
	return errs
}

func validateKubectlRestart(loc string, r types.E2EKubectlRestart) []error {
	var errs []error
	if r.Kind == "" {
		errs = append(errs, fmt.Errorf("%s: kind is required", loc))
	}
	if r.Name == "" {
		errs = append(errs, fmt.Errorf("%s: name is required", loc))
	}
	return errs
}

func validateKubectlScale(loc string, s types.E2EKubectlScale) []error {
	var errs []error
	if s.Kind == "" {
		errs = append(errs, fmt.Errorf("%s: kind is required", loc))
	}
	if s.Name == "" {
		errs = append(errs, fmt.Errorf("%s: name is required", loc))
	}
	if s.Replicas < 0 {
		errs = append(errs, fmt.Errorf("%s: replicas must be >= 0", loc))
	}
	return errs
}
