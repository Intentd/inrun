package kubectl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/jsonpath"
)

// The read verbs: each runs one kubectl entry that observes the cluster and
// returns its output (extracted when the entry declares an extractor) with the
// error from its assertions. Assert stops at the first error; Post prints the
// output and carries on. auth and port-forward, which talk to the API
// directly, are in their own files.

func readGet(ctx context.Context, cs kubernetes.Interface, dyn dynamic.Interface, mapper apimeta.RESTMapper, e types.E2EKubectlGet, env types.EnvResolver, workDir string) (string, error) {
	format := e.Format
	if e.Field != "" {
		field := e.Field
		if !strings.HasPrefix(field, ".") {
			field = "." + field
		}
		format = "jsonpath={" + field + "}"
	} else if format == "" {
		format = "json"
	}

	if dyn != nil && mapper != nil && dynamicFormat(format) {
		return readGetDynamic(ctx, dyn, mapper, e, format, env, workDir)
	}

	out, err := Get(ctx, GetOptions{
		Kind:          e.Kind,
		Name:          e.Name,
		LabelSelector: e.LabelSelector,
		Namespace:     e.Namespace,
		Format:        format,
		WorkDir:       workDir,
		Env:           env,
	})
	if err != nil {
		return out, assertGetError(out, err, e)
	}

	got, err := applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    out,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ, YQ: e.YQ},
	})
	if err != nil {
		return out, fmt.Errorf("kubectl get %s/%s: %w", e.Kind, e.Name, err)
	}

	loc := "kubectl get " + e.Kind + "/" + e.Name
	if err := assertInlineJSON(got, e.JSON, loc); err != nil {
		return got, err
	}
	return got, assertOutput(got, e.TextAssertion, e.Assert, loc)
}

// dynamicFormat reports whether format can be served by the dynamic client
// (JSON is returned directly; jsonpath is applied locally over the object).
// Formats like yaml, wide, custom-columns still require the kubectl subprocess.
func dynamicFormat(format string) bool {
	return format == "json" || strings.HasPrefix(format, "jsonpath=")
}

// readGetDynamic fetches the resource via the dynamic client and applies the
// requested format locally, without shelling out to kubectl.
func readGetDynamic(ctx context.Context, dyn dynamic.Interface, mapper apimeta.RESTMapper, e types.E2EKubectlGet, format string, env types.EnvResolver, workDir string) (string, error) {
	loc := fmt.Sprintf("kubectl get %s/%s", e.Kind, e.Name)

	obj, found, err := kubeclient.GetUnstructuredByKind(ctx, mapper, dyn, e.Kind, e.Name, e.Namespace, e.LabelSelector)
	if err != nil {
		out := err.Error()
		return out, assertGetError(out, err, e)
	}
	if !found {
		// Kind not found via discovery — fall back to kubectl subprocess.
		return readGetSubprocess(ctx, e, format, env, workDir)
	}

	var out string
	switch {
	case format == "json":
		b, err := json.Marshal(obj)
		if err != nil {
			return "", fmt.Errorf("%s: marshalling to JSON: %w", loc, err)
		}
		out = string(b)
	case strings.HasPrefix(format, "jsonpath="):
		expr := strings.TrimPrefix(format, "jsonpath=")
		out, err = applyJSONPath(expr, obj)
		if err != nil {
			return "", fmt.Errorf("%s: jsonpath %q: %w", loc, expr, err)
		}
	}

	got, err := applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    out,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ, YQ: e.YQ},
	})
	if err != nil {
		return out, fmt.Errorf("%s: %w", loc, err)
	}

	return got, assertOutput(got, e.TextAssertion, e.Assert, loc)
}

// applyJSONPath evaluates a kubectl-style jsonpath expression (e.g. "{.metadata.name}")
// against obj and returns the result as a trimmed string.
func applyJSONPath(expr string, obj map[string]interface{}) (string, error) {
	jp := jsonpath.New("get").AllowMissingKeys(false)
	if err := jp.Parse(expr); err != nil {
		return "", fmt.Errorf("parsing jsonpath %q: %w", expr, err)
	}
	var buf bytes.Buffer
	if err := jp.Execute(&buf, obj); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// readGetSubprocess falls back to the kubectl subprocess for formats the
// dynamic client cannot produce (yaml, wide, custom-columns, name).
func readGetSubprocess(ctx context.Context, e types.E2EKubectlGet, format string, env types.EnvResolver, workDir string) (string, error) {
	out, err := Get(ctx, GetOptions{
		Kind:          e.Kind,
		Name:          e.Name,
		LabelSelector: e.LabelSelector,
		Namespace:     e.Namespace,
		Format:        format,
		WorkDir:       workDir,
		Env:           env,
	})
	if err != nil {
		return out, assertGetError(out, err, e)
	}

	got, err := applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    out,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ, YQ: e.YQ},
	})
	if err != nil {
		return out, fmt.Errorf("kubectl get %s/%s: %w", e.Kind, e.Name, err)
	}

	loc := "kubectl get " + e.Kind + "/" + e.Name
	if err := assertInlineJSON(got, e.JSON, loc); err != nil {
		return got, err
	}
	return got, assertOutput(got, e.TextAssertion, e.Assert, loc)
}

func assertGetError(out string, commandErr error, e types.E2EKubectlGet) error {
	loc := fmt.Sprintf("kubectl get %s/%s", e.Kind, e.Name)

	hasExtractor := e.Field != "" || e.Format != "" ||
		e.JSONPath != "" || e.JQ != "" || e.YQ != ""

	if hasExtractor || (e.Assert != nil && e.Assert.HasJSON()) {
		return fmt.Errorf("%s: command failed: %w\noutput: %s", loc, commandErr, out)
	}

	// A missing resource satisfies an explicit absence assertion.
	expectsAbsent := e.Assert != nil &&
		((e.Assert.Exists != nil && !*e.Assert.Exists) ||
			(e.Assert.NotExists != nil && *e.Assert.NotExists))

	if expectsAbsent && isNotFound(out) {
		if e.Assert != nil {
			// Still evaluate any other primitive assertions.
			assertions := *e.Assert
			assertions.Exists = nil
			assertions.NotExists = nil
			if err := assertions.Check(out); err != nil {
				return fmt.Errorf("%s: %w\noutput: %s", loc, err, out)
			}
		}
		return nil
	}

	// Primitive assertions can deliberately inspect failed-command output.
	if e.Assert != nil {
		if err := e.Assert.Check(out); err != nil {
			return fmt.Errorf("%s: command failed: %w; assertion failed: %v\noutput: %s",
				loc, commandErr, err, out)
		}
	}

	return nil
}

func isNotFound(output string) bool {
	output = strings.ToLower(output)

	return strings.Contains(output, "(notfound)") ||
		strings.Contains(output, "not found")
}

func readLogs(ctx context.Context, cs kubernetes.Interface, e types.E2EKubectlLogs, env types.EnvResolver, workDir string) (string, error) {
	if e.Pods != "" {
		return readLogsPerPod(ctx, cs, e, env, workDir)
	}
	target, err := resolveLogsTarget(ctx, cs, e)
	if err != nil {
		return "", err
	}

	out, err := Logs(ctx, LogsOptions{
		Target:        target.Pod,
		LabelSelector: target.LabelSelector,
		Namespace:     target.Namespace,
		Container:     e.Container,
		Since:         e.Since,
		WorkDir:       workDir,
		Env:           env,
	})
	if err != nil {
		return out, fmt.Errorf("kubectl logs: %s", out)
	}

	got, err := applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    out,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ},
	})
	if err != nil {
		return out, fmt.Errorf("kubectl logs: %w", err)
	}

	return got, assertOutput(got, e.TextAssertion, e.Assert, "kubectl logs")
}

func readDescribe(ctx context.Context, e types.E2EKubectlDescribe, env types.EnvResolver, workDir string) (string, error) {
	out, err := Describe(ctx, DescribeOptions{
		Kind:          e.Kind,
		Name:          e.Name,
		LabelSelector: e.LabelSelector,
		Namespace:     e.Namespace,
		WorkDir:       workDir,
		Env:           env,
	})
	if err != nil {
		return out, fmt.Errorf("kubectl describe %s: %s", e.Kind, out)
	}

	return out, assertOutput(out, e.TextAssertion, e.Assert, "kubectl describe "+e.Kind)
}

func readEvents(ctx context.Context, e types.E2EKubectlEvents, env types.EnvResolver, workDir string) (string, error) {
	out, err := Events(ctx, EventsOptions{
		Kind:      e.Kind,
		Name:      e.Name,
		Namespace: e.Namespace,
		WorkDir:   workDir,
		Env:       env,
	})
	if err != nil {
		return out, fmt.Errorf("kubectl events %s/%s: %s", e.Kind, e.Name, out)
	}

	return out, assertOutput(out, e.TextAssertion, e.Assert, "kubectl events "+e.Kind+"/"+e.Name)
}

func readExec(ctx context.Context, cs kubernetes.Interface, e types.E2EKubectlExec, env types.EnvResolver, workDir string) (string, error) {
	target, err := resolveExecTarget(ctx, cs, e, env, workDir)
	if err != nil {
		return "", err
	}

	out, err := Exec(ctx, ExecOptions{
		Pod:       target.Pod,
		Namespace: target.Namespace,
		Container: e.Container,
		Command:   e.Command,
		WorkDir:   workDir,
		Env:       env,
	})
	if err != nil {
		return out, fmt.Errorf("kubectl exec %s: %s", target.Pod, out)
	}

	got, err := applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    out,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ, YQ: e.YQ},
	})
	if err != nil {
		return out, fmt.Errorf("kubectl exec %s: %w", target.Pod, err)
	}

	return got, assertOutput(got, e.TextAssertion, e.Assert, "kubectl exec "+target.Pod)
}

func readCp(ctx context.Context, e types.E2EKubectlCp, env types.EnvResolver, workDir string) (string, error) {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}

	pod := e.Name
	if pod == "" && e.LabelSelector != "" {
		var err error
		pod, err = Get(ctx, GetOptions{
			Kind:          "pod",
			LabelSelector: e.LabelSelector,
			Namespace:     ns,
			Format:        "jsonpath={.items[0].metadata.name}",
			WorkDir:       workDir,
			Env:           env,
		})
		if err != nil || strings.TrimSpace(pod) == "" {
			return "", fmt.Errorf("kubectl cp: no pod found for selector %q in %s", e.LabelSelector, ns)
		}
	}

	tmp, err := os.CreateTemp("", "inrun-e2e-cp-*")
	if err != nil {
		return "", fmt.Errorf("kubectl cp: creating temp file: %w", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	src := fmt.Sprintf("%s/%s:%s", ns, pod, e.Src)
	if out, err := Cp(ctx, CpOptions{
		Source:    src,
		Dest:      tmp.Name(),
		Container: e.Container,
		WorkDir:   workDir,
		Env:       env,
	}); err != nil {
		return out, fmt.Errorf("kubectl cp %s: %s", src, out)
	}

	raw, err := readLocal(tmp.Name())
	if err != nil {
		return "", fmt.Errorf("kubectl cp: reading temp file: %w", err)
	}
	out := string(raw)

	got, err := applyExtract(ctx, types.ExtractRequest{
		WorkDir: workDir,
		Output:  out,
		Extractor: types.Extractor{
			JSONPath: e.JSONPath,
			JQ:       e.JQ,
			YQ:       e.YQ,
		}})
	if err != nil {
		return out, fmt.Errorf("kubectl cp %s: %w", src, err)
	}

	return got, assertOutput(got, e.TextAssertion, e.Assert, "kubectl cp "+src)
}

func readTop(ctx context.Context, e types.E2EKubectlTop, env types.EnvResolver, workDir string) (string, error) {
	kind := strings.ToLower(e.Kind)
	out, err := Top(ctx, TopOptions{
		Kind:          kind,
		Name:          e.Name,
		LabelSelector: e.LabelSelector,
		Namespace:     e.Namespace,
		Containers:    e.Containers,
		WorkDir:       workDir,
		Env:           env,
	})
	if err != nil {
		return out, fmt.Errorf("kubectl top %s: %s", kind, out)
	}

	return out, assertOutput(out, e.TextAssertion, e.Assert, "kubectl top "+kind)
}

// readLogsPerPod checks each pod the selector matches on its own: its full log,
// extracted and asserted. pods: any passes when one pod passes, all when every
// one does. It returns every pod's output, each under its name.
func readLogsPerPod(ctx context.Context, cs kubernetes.Interface, e types.E2EKubectlLogs, env types.EnvResolver, workDir string) (string, error) {
	ns := defaultOr(e.Namespace)
	list, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: e.LabelSelector})
	if err != nil {
		return "", fmt.Errorf("kubectl logs: listing pods %q in %s: %w", e.LabelSelector, ns, err)
	}
	if len(list.Items) == 0 {
		return "", fmt.Errorf("kubectl logs: no pods match %q in %s", e.LabelSelector, ns)
	}
	names := make([]string, 0, len(list.Items))
	for _, p := range list.Items {
		names = append(names, p.Name)
	}
	sort.Strings(names)

	var outputs, failures []string
	for _, pod := range names {
		got, err := podLog(ctx, e, pod, ns, env, workDir)
		outputs = append(outputs, "── pod/"+pod+"\n"+got)
		if err != nil {
			failures = append(failures, "pod/"+pod+": "+err.Error())
		}
	}
	out := strings.Join(outputs, "\n")
	passed := len(names) - len(failures)
	switch {
	case e.Pods == types.E2ELogsPodsAny && passed == 0:
		return out, fmt.Errorf("kubectl logs (pods: any): no pod of %d passed:\n%s", len(names), strings.Join(failures, "\n"))
	case e.Pods == types.E2ELogsPodsAll && len(failures) > 0:
		return out, fmt.Errorf("kubectl logs (pods: all): %d of %d pods failed:\n%s", len(failures), len(names), strings.Join(failures, "\n"))
	}
	return out, nil
}

// fetchPodLog runs kubectl logs for one pod; tests replace it.
var fetchPodLog = Logs

// podLog is one pod's full log, extracted and asserted.
func podLog(ctx context.Context, e types.E2EKubectlLogs, pod, ns string, env types.EnvResolver, workDir string) (string, error) {
	out, err := fetchPodLog(ctx, LogsOptions{
		Target:    pod,
		Namespace: ns,
		Container: e.Container,
		Since:     e.Since,
		WorkDir:   workDir,
		Env:       env,
	})
	if err != nil {
		return out, fmt.Errorf("kubectl logs: %s", out)
	}
	got, err := applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    out,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ},
	})
	if err != nil {
		return out, fmt.Errorf("kubectl logs: %w", err)
	}
	return got, assertOutput(got, e.TextAssertion, e.Assert, "kubectl logs")
}
