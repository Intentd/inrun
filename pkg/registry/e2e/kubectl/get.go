package kubectl

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

func kubectlGet(ctx context.Context, e types.E2EKubectlGet, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}

	var args []string
	if e.Field != "" {
		field := e.Field
		if !strings.HasPrefix(field, ".") {
			field = "." + field
		}
		args = []string{"get", e.Kind, e.Name, "-n", ns, "-o", "jsonpath={" + field + "}"}
	} else {
		format := e.Format
		if format == "" {
			format = "json"
		}
		args = []string{"get", e.Kind, e.Name, "-n", ns, "-o", format}
	}

	out, err := Run(ctx, workDir, env, args...)
	if err != nil {
		return assertKubectlGetError(out, err, e)
	}

	out, err = applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    out,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ, YQ: e.YQ},
	})
	if err != nil {
		return fmt.Errorf("kubectl get %s/%s: %w", e.Kind, e.Name, err)
	}

	if err := e.JSON.Check(out); err != nil {
		return fmt.Errorf("kubectl get %s/%s: %w\noutput: %s", e.Kind, e.Name, err, out)
	}

	return assertKubectlGetOutput(out, e)
}

func assertKubectlGetError(out string, commandErr error, e types.E2EKubectlGet) error {
	loc := fmt.Sprintf("kubectl get %s/%s", e.Kind, e.Name)

	hasExtractor := e.Field != "" || e.Format != "" ||
		e.JSONPath != "" || e.JQ != "" || e.YQ != ""

	if hasExtractor || !e.JSON.Empty() {
		return fmt.Errorf("%s: command failed: %w\noutput: %s", loc, commandErr, out)
	}

	// A missing resource satisfies an explicit absence assertion.
	expectsAbsent := (e.Exists != nil && !*e.Exists) ||
		(e.NotExists != nil && *e.NotExists)

	if expectsAbsent && isKubectlNotFound(out) {
		// Still evaluate any other primitive assertions.
		assertions := e.E2EAssertions
		assertions.Exists = nil
		assertions.NotExists = nil

		if err := assertions.Check(out); err != nil {
			return fmt.Errorf("%s: %w\noutput: %s", loc, err, out)
		}

		return nil
	}

	// Primitive assertions can deliberately inspect failed-command output.
	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("%s: command failed: %w; assertion failed: %v\noutput: %s",
			loc, commandErr, err, out)
	}

	return nil
}

func isKubectlNotFound(output string) bool {
	output = strings.ToLower(output)

	return strings.Contains(output, "(notfound)") ||
		strings.Contains(output, "not found")
}

// assertKubectlGetOutput applies e's assertions to already-fetched (and
// jq/yq-extracted) output. Split out from kubectlGet so this logic is
// testable without kubectl or a cluster.
func assertKubectlGetOutput(out string, e types.E2EKubectlGet) error {
	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("kubectl get %s/%s: %w\noutput: %s", e.Kind, e.Name, err, out)
	}
	return nil
}
