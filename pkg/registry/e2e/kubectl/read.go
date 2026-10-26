package kubectl

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func kubectlLogs(ctx context.Context, cs kubernetes.Interface, e types.E2EKubectlLogs, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}

	args := []string{"logs", "-n", ns}
	if e.LeaderElection != nil {
		pod, leaseNs, err := ResolveLeaderHolder(ctx, cs, e.LeaderElection, ns)
		if err != nil {
			return fmt.Errorf("kubectl logs: %w", err)
		}
		args = []string{"logs", "-n", leaseNs, pod}
	} else if e.LabelSelector != "" {
		args = append(args, "-l", e.LabelSelector)
	} else {
		args = append(args, e.Name)
	}
	if e.Container != "" {
		args = append(args, "-c", e.Container)
	}
	if e.Since != "" {
		args = append(args, "--since="+e.Since)
	}

	out, err := Run(ctx, workDir, env, args...)
	if err != nil {
		return fmt.Errorf("kubectl logs: %s", out)
	}

	out, err = applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    out,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ},
	})
	if err != nil {
		return fmt.Errorf("kubectl logs: %w", err)
	}

	if err := e.JSON.Check(out); err != nil {
		return fmt.Errorf("kubectl logs: %w\noutput: %s", err, out)
	}
	return assertKubectlLogsOutput(out, e)
}

// assertKubectlLogsOutput applies e's assertions to already-fetched (and
// json/jq-extracted) log output. Split out so this logic is testable without
// kubectl or a cluster.
func assertKubectlLogsOutput(out string, e types.E2EKubectlLogs) error {
	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("kubectl logs: %w\noutput: %s", err, out)
	}
	return nil
}

func kubectlDescribe(ctx context.Context, e types.E2EKubectlDescribe, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}

	args := []string{"describe", e.Kind, "-n", ns}
	if e.Name != "" {
		args = append(args, e.Name)
	} else if e.LabelSelector != "" {
		args = append(args, "-l", e.LabelSelector)
	}

	out, err := Run(ctx, workDir, env, args...)
	if err != nil {
		return fmt.Errorf("kubectl describe %s: %s", e.Kind, out)
	}

	return assertKubectlDescribeOutput(out, e)
}

// assertKubectlDescribeOutput applies e's assertions to already-fetched
// describe output. Split out so this logic is testable without kubectl or
// a cluster.
func assertKubectlDescribeOutput(out string, e types.E2EKubectlDescribe) error {
	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("kubectl describe %s: %w\noutput: %s", e.Kind, err, out)
	}
	return nil
}

func kubectlEvents(ctx context.Context, e types.E2EKubectlEvents, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}
	args := []string{"events", "--for", e.Kind + "/" + e.Name, "-n", ns}
	out, err := Run(ctx, workDir, env, args...)
	if err != nil {
		return fmt.Errorf("kubectl events %s/%s: %s", e.Kind, e.Name, out)
	}
	return assertKubectlEventsOutput(out, e)
}

// assertKubectlEventsOutput applies e's assertions to already-fetched
// events output. Split out so this logic is testable without kubectl or a
// cluster.
func assertKubectlEventsOutput(out string, e types.E2EKubectlEvents) error {
	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("kubectl events %s/%s: %w\noutput: %s", e.Kind, e.Name, err, out)
	}
	return nil
}
