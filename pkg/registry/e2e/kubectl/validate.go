package kubectl

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// Validate checks every kubectl block across all expect entries.
// Returns one error per violation — callers collect and print all of them.
func Validate(expects []types.E2EExpectation) []error {
	var errs []error
	for i, exp := range expects {
		if exp.Kubectl == nil {
			continue
		}
		loc := func(sub string, idx int) string {
			return fmt.Sprintf("expect[%d] %q kubectl.%s[%d]", i, exp.Name, sub, idx)
		}
		for j, g := range exp.Kubectl.Get {
			errs = append(errs, validateKubectlGet(loc("get", j), g)...)
		}
		for j, l := range exp.Kubectl.Logs {
			errs = append(errs, validateKubectlLogs(loc("logs", j), l)...)
		}
		for j, d := range exp.Kubectl.Describe {
			errs = append(errs, validateKubectlDescribe(loc("describe", j), d)...)
		}
		for j, e := range exp.Kubectl.Exec {
			errs = append(errs, validateKubectlExec(loc("exec", j), e)...)
		}
		for j, p := range exp.Kubectl.PortForward {
			errs = append(errs, validateKubectlPortForward(loc("port-forward", j), p)...)
		}
		for j, a := range exp.Kubectl.Apply {
			errs = append(errs, validateKubectlApply(loc("apply", j), a)...)
		}
		for j, d := range exp.Kubectl.Delete {
			errs = append(errs, validateKubectlDelete(loc("delete", j), d)...)
		}
		for j, p := range exp.Kubectl.Patch {
			errs = append(errs, validateKubectlPatch(loc("patch", j), p)...)
		}
		for j, e := range exp.Kubectl.Events {
			errs = append(errs, validateKubectlEvents(loc("events", j), e)...)
		}
		for j, a := range exp.Kubectl.Auth {
			errs = append(errs, validateKubectlAuth(loc("auth", j), a)...)
		}
		for j, c := range exp.Kubectl.Cp {
			errs = append(errs, validateKubectlCp(loc("cp", j), c)...)
		}
		for j, t := range exp.Kubectl.Top {
			errs = append(errs, validateKubectlTop(loc("top", j), t)...)
		}
		for j, r := range exp.Kubectl.Restart {
			errs = append(errs, validateKubectlRestart(loc("restart", j), r)...)
		}
		for j, s := range exp.Kubectl.Scale {
			errs = append(errs, validateKubectlScale(loc("scale", j), s)...)
		}
	}
	return errs
}

func validateKubectlGet(loc string, g types.E2EKubectlGet) []error {
	var errs []error

	if g.Kind == "" {
		errs = append(errs, fmt.Errorf("%s: kind is required", loc))
	}
	if g.Name == "" {
		errs = append(errs, fmt.Errorf("%s: name is required", loc))
	}

	objectLevel := (g.Exists != nil && !*g.Exists) ||
		(g.NotExists != nil && *g.NotExists)

	hasAssertions := !g.E2EAssertions.Empty() ||
		!g.JSON.Empty() ||
		objectLevel

	if !hasAssertions {
		errs = append(errs, fmt.Errorf("%s: %s", loc, assertionListMsg))
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
	if l.E2EAssertions.Empty() && l.JSON.Empty() {
		errs = append(errs, fmt.Errorf("%s: %s", loc, assertionListMsg))
	}
	return errs
}

func validateKubectlDescribe(loc string, d types.E2EKubectlDescribe) []error {
	var errs []error
	if d.Kind == "" {
		errs = append(errs, fmt.Errorf("%s: kind is required", loc))
	}
	if d.Name == "" && d.LabelSelector == "" {
		errs = append(errs, fmt.Errorf("%s: name or labelSelector is required", loc))
	}
	if d.E2EAssertions.Empty() {
		errs = append(errs, fmt.Errorf("%s: %s", loc, assertionListMsg))
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
	if len(e.Command) == 0 {
		errs = append(errs, fmt.Errorf("%s: command is required", loc))
	}
	if e.E2EAssertions.Empty() && e.JSON.Empty() {
		errs = append(errs, fmt.Errorf("%s: %s", loc, assertionListMsg))
	}
	return errs
}

func validateKubectlPortForward(loc string, p types.E2EKubectlPortForward) []error {
	var errs []error
	if p.LeaderElection != nil {
		if p.LeaderElection.Lease == "" {
			errs = append(errs, fmt.Errorf("%s: leaderElection.lease is required", loc))
		}
	} else if p.Service == "" && p.Pod == "" {
		errs = append(errs, fmt.Errorf("%s: service, pod, or leaderElection is required", loc))
	}
	if p.Port <= 0 {
		errs = append(errs, fmt.Errorf("%s: port must be > 0", loc))
	}
	hasAny := !p.E2EAssertions.Empty() || !p.JSON.Empty()
	if hasAny && p.Path == "" {
		errs = append(errs, fmt.Errorf("%s: path is required when assertions are set", loc))
	}

	// body / bodyFile
	switch {
	case p.Body != "" && p.BodyFile != "":
		errs = append(errs, fmt.Errorf("%s: body and bodyFile are mutually exclusive", loc))
	case p.Body != "" || p.BodyFile != "":
		switch strings.ToUpper(p.Method) {
		case "GET", "HEAD", "DELETE":
			errs = append(errs, fmt.Errorf("%s: %s does not accept a body", loc, strings.ToUpper(p.Method)))
		}
	}

	// contentYAML
	if p.ContentYAML {
		if p.Body == "" && p.BodyFile == "" {
			errs = append(errs, fmt.Errorf("%s: contentYAML set without body or bodyFile", loc))
		}
		if _, set := p.Headers["Content-Type"]; set {
			// header wins; not an error
		}
	}

	// statusCode vs. body assertions
	if p.StatusCode != 0 {
		if p.JQ != "" || p.YQ != "" || p.JSONPath != "" || len(p.JSON) > 0 || hasAny {
			errs = append(errs, fmt.Errorf("%s: statusCode cannot be combined with body assertions", loc))
		}
	}
	return errs
}

func validateKubectlDelete(loc string, d types.E2EKubectlDelete) []error {
	var errs []error
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
	return errs
}

func validateKubectlApply(loc string, a types.E2EKubectlApply) []error {
	var errs []error
	if a.File == "" && a.Inline == "" {
		errs = append(errs, fmt.Errorf("%s: file or inline is required", loc))
	}
	if a.File != "" && a.Inline != "" {
		errs = append(errs, fmt.Errorf("%s: file and inline are mutually exclusive", loc))
	}
	return errs
}

func validateKubectlPatch(loc string, p types.E2EKubectlPatch) []error {
	var errs []error
	if p.Kind == "" {
		errs = append(errs, fmt.Errorf("%s: kind is required", loc))
	}
	if p.Name == "" {
		errs = append(errs, fmt.Errorf("%s: name is required", loc))
	}
	if p.Patch == "" {
		errs = append(errs, fmt.Errorf("%s: patch is required", loc))
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
	if e.E2EAssertions.Empty() {
		errs = append(errs, fmt.Errorf("%s: %s", loc, assertionListMsg))
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
	if a.E2EAssertions.Empty() {
		errs = append(errs, fmt.Errorf("%s: %s", loc, assertionListMsg))
	}
	return errs
}

func validateKubectlCp(loc string, c types.E2EKubectlCp) []error {
	var errs []error
	if c.Name == "" && c.LabelSelector == "" {
		errs = append(errs, fmt.Errorf("%s: name or labelSelector is required", loc))
	}
	if c.Src == "" {
		errs = append(errs, fmt.Errorf("%s: src is required", loc))
	}
	if c.E2EAssertions.Empty() {
		errs = append(errs, fmt.Errorf("%s: %s", loc, assertionListMsg))
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
	if t.E2EAssertions.Empty() {
		errs = append(errs, fmt.Errorf("%s: %s", loc, assertionListMsg))
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
	return errs
}
