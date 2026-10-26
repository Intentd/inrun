package expect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// errSkipped is a sentinel returned when a when:/or: gate is not met.
// The runner detects it to record a skipped case rather than a failure.
var errSkipped = fmt.Errorf("skipped")

// verify polls until all conditions pass or timeout expires.
// workDir is the working directory for command checks — relative paths in
// commands and resource file refs resolve from there.
func verify(ctx context.Context, sc types.E2EExpectationScope, resolver *template.Resolver) error {
	exp := sc.Expectation
	if !types.EvaluateConditions(nil, exp.When, exp.Or, resolver.TemplateEvaluator()) {
		return errSkipped
	}
	if exp.Wait != "" {
		if d, err := parseTimeDuration(exp.Wait); err == nil && d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	timeout, err := parseTimeDuration(exp.Timeout)
	if err != nil || timeout <= 0 {
		timeout = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// kubectl mutations (apply, patch, restart, scale, delete) run until they
	// succeed once; only the assertions are retried after that. Re-running a
	// mutation on every retry would undo what the assertion waits for, e.g. a
	// drift patch re-applied right before checking it was reverted.
	mutated := exp.Kubectl == nil
	sc.Mutated = &mutated
	for {
		if err := checkAll(ctx, sc, resolver); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %s waiting for %q: %w", timeout, exp.Name, checkAll(ctx, sc, resolver))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func checkAll(ctx context.Context, sc types.E2EExpectationScope, resolver *template.Resolver) error {
	exp := sc.Expectation
	// Commands run before resources so that action commands (e.g. cleanup deletes)
	// execute before the resource state is checked.
	for _, cmd := range exp.Commands {
		if err := checkCommand(ctx, cmd, sc.WorkDir, sc.Env); err != nil {
			return err
		}
	}
	if exp.Kubectl != nil {
		if !*sc.Mutated {
			if err := runKubectlMutations(ctx, exp.Kubectl, sc.WorkDir, sc.Env, sc.CS); err != nil {
				return err
			}
			*sc.Mutated = true
		}
		if err := checkKubectl(ctx, exp.Kubectl, sc.Env, sc.WorkDir, sc.CS, sc.Cfg); err != nil {
			return err
		}
	}
	for _, r := range exp.Resources {
		if err := checkResource(ctx, r, sc.Env, sc.WorkDir); err != nil {
			return err
		}
	}
	for _, c := range exp.External {
		if err := checkExternal(ctx, c, sc.WorkDir, resolver, sc.CS); err != nil {
			return err
		}
	}
	return nil
}

// checkExternal makes the call and asserts on its result. A call that fails
// fails the check, as does an assertion that does not hold.
func checkExternal(ctx context.Context, c types.E2EExternalCheck, workDir string, resolver *template.Resolver, cs kubernetes.Interface) error {
	result, err := external.Call(ctx, "e2e", "external", resolver, c.ExternalCallSpec, cs)
	if err != nil {
		return fmt.Errorf("external %q: %w", c.Name, err)
	}
	if msg, _ := result["error"].(string); msg != "" {
		return fmt.Errorf("external %q: %s", c.Name, msg)
	}
	// A call whose when:/or: does not pass is skipped, like a gated step.
	if called, _ := result["called"].(string); called == "false" {
		return nil
	}
	if c.JQ == "" && c.E2EAssertions.Empty() {
		return nil
	}
	out, err := externalOutput(ctx, result, types.Extractor{JSONPath: c.JSONPath, JQ: c.JQ, YQ: c.YQ}, workDir)
	if err != nil {
		return fmt.Errorf("external %q: %w", c.Name, err)
	}
	if err := c.JSON.Check(out); err != nil {
		return fmt.Errorf("external %q: %w", c.Name, err)
	}
	if err := c.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("external %q: %w\noutput: %s", c.Name, err, out)
	}
	return nil
}

// externalOutput is what a check's assertions apply to: the extractor over
// the result when set, else the body for HTTP and result for the other protocols.
func externalOutput(ctx context.Context, result map[string]interface{}, ext types.Extractor, workDir string) (string, error) {
	// What the assertions see by default: body for HTTP, result otherwise.
	defaultOut := fmt.Sprint(result["result"])
	if body, ok := result["body"].(string); ok {
		defaultOut = body
	}

	switch {
	case ext.JSONPath != "":
		// Navigate the default output as JSON (the body for HTTP).
		return applyExtract(ctx, types.ExtractRequest{
			WorkDir: workDir, Output: defaultOut,
			Extractor: types.Extractor{JSONPath: ext.JSONPath},
		})
	case ext.JQ != "" || ext.YQ != "":
		// External extractors see the whole envelope.
		raw, err := json.Marshal(result)
		if err != nil {
			return "", err
		}
		return applyExtract(ctx, types.ExtractRequest{
			WorkDir: workDir, Output: string(raw),
			Extractor: types.Extractor{JQ: ext.JQ, YQ: ext.YQ},
		})
	default:
		return defaultOut, nil
	}
}

// checkResource asserts the state of any Kubernetes resource using kubectl.
// Kind can be any built-in or custom resource kind.
func checkResource(ctx context.Context, r types.E2EResourceCheck, env types.EnvResolver, workDir string) error {
	ns := r.Namespace
	if ns == "" {
		ns = "default"
	}

	// count=0: assert resource(s) do NOT exist.
	// A kubectl error (CRD unknown, type not registered) also means nothing exists — pass.
	if r.Count != nil && *r.Count == 0 {
		var args []string
		if r.Name != "" {
			args = []string{"get", r.Kind, r.Name, "-n", ns, "--ignore-not-found", "-o", "name"}
		} else {
			args = []string{"get", r.Kind, "-n", ns, "--ignore-not-found", "-o", "name"}
		}
		out, err := runKubectl(ctx, workDir, env, args...)
		if err != nil {
			return nil // kubectl error = CRD unknown or type missing = nothing exists
		}
		if strings.TrimSpace(out) != "" {
			if r.Name != "" {
				return fmt.Errorf("%s/%s still exists in %s", r.Kind, r.Name, ns)
			}
			return fmt.Errorf("%s still exists in namespace %s:\n%s", r.Kind, ns, out)
		}
		return nil
	}

	// Named resource: assert it exists.
	if r.Name != "" {
		out, err := runKubectl(ctx, workDir, env, "get", r.Kind, r.Name, "-n", ns, "--ignore-not-found", "-o", "name")
		if err != nil || strings.TrimSpace(out) == "" {
			return fmt.Errorf("%s/%s not found in %s", r.Kind, r.Name, ns)
		}
		if r.Ready {
			return checkReady(ctx, env, workDir, r.Kind, r.Name, ns)
		}
		return nil
	}

	// Unnamed: assert at least one (or exact count) exists.
	out, err := runKubectl(ctx, workDir, env, "get", r.Kind, "-n", ns, "-o", "name")
	if err != nil || strings.TrimSpace(out) == "" {
		return fmt.Errorf("no %s found in namespace %s", r.Kind, ns)
	}
	if r.Count != nil {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != *r.Count {
			return fmt.Errorf("%s count in %s: want %d, got %d", r.Kind, ns, *r.Count, len(lines))
		}
	}
	if r.Ready {
		// Check readiness of the first result.
		first := strings.SplitN(strings.TrimSpace(out), "\n", 2)[0]
		if parts := strings.SplitN(first, "/", 2); len(parts) == 2 {
			first = parts[1]
		}
		return checkReady(ctx, env, workDir, r.Kind, first, ns)
	}
	return nil
}

// checkReady checks whether a resource has available replicas.
// Uses jsonpath to read status.availableReplicas — covers Deployments and
// StatefulSets. Returns nil (ready) if the field is absent (e.g. Services).
func checkReady(ctx context.Context, env types.EnvResolver, workDir, kind, name, ns string) error {
	out, err := runKubectl(ctx, workDir, env,
		"get", kind, name, "-n", ns, "-o", "jsonpath={.status.availableReplicas}")
	if err != nil {
		return fmt.Errorf("%s/%s in %s: not ready (%s)", kind, name, ns, strings.TrimSpace(out))
	}
	val := strings.TrimSpace(out)
	if val == "0" {
		return fmt.Errorf("%s/%s in %s: not ready (availableReplicas=0)", kind, name, ns)
	}
	// val == "" means field doesn't exist on this kind — treat as ready (e.g. Service).
	return nil
}

func checkCommand(ctx context.Context, c types.E2ECommand, workDir string, resolver types.EnvResolver) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", c.Run)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = append(os.Environ(), resolver.Pairs()...)
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return fmt.Errorf("running %q: %w", c.Run, err)
		}
	}

	if exitCode != c.ExitCode {
		return fmt.Errorf("command %q: expected exit code %d, got %d\noutput: %s",
			c.Run, c.ExitCode, exitCode, strings.TrimSpace(string(out)))
	}

	output, err := applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    string(out),
		Extractor: types.Extractor{JSONPath: c.JSONPath, JQ: c.JQ, YQ: c.YQ},
	})

	if err := c.JSON.Check(output); err != nil {
		return fmt.Errorf("command %q: %w\noutput: %s", c.Run, err, strings.TrimSpace(string(out)))
	}

	if err := c.E2EAssertions.Check(output); err != nil {
		return fmt.Errorf("command %q: %w\noutput: %s", c.Run, err, strings.TrimSpace(string(out)))
	}
	return nil
}
