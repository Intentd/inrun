package kubectl

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const portForwardTimeout = 15 * time.Second

func kubectlPortForward(ctx context.Context, cs kubernetes.Interface, cfg *rest.Config, e types.E2EKubectlPortForward, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}

	// Resolve to a pod — the k8s portforward API operates on pods, not services.
	var pod, podNs string
	if e.LeaderElection != nil {
		p, ln, err := ResolveLeaderHolder(ctx, cs, e.LeaderElection, ns)
		if err != nil {
			return fmt.Errorf("port-forward: %w", err)
		}
		pod, podNs = p, ln
	} else if e.Service != "" {
		p, err := resolveServicePod(ctx, cs, ns, e.Service)
		if err != nil {
			return fmt.Errorf("port-forward svc/%s: %w", e.Service, err)
		}
		pod, podNs = p, ns
	} else {
		pod, podNs = e.Pod, ns
	}

	raw, err := doPortForwardGoRaw(ctx, cfg, workDir, podNs, pod, e, env)
	if err != nil {
		return err
	}
	if e.StatusCode != 0 {
		got, _ := strconv.Atoi(strings.TrimSpace(raw))
		if got != e.StatusCode {
			return fmt.Errorf("port-forward pod/%s%s: expected HTTP %d, got %s", pod, e.Path, e.StatusCode, raw)
		}
		return nil
	}
	return assertPortForwardOutput(ctx, workDir, raw, e, "pod/"+pod)
}

// resolveServicePod returns a running pod name backing the given service.
func resolveServicePod(ctx context.Context, cs kubernetes.Interface, ns, svcName string) (string, error) {
	svc, err := cs.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get service %s: %w", svcName, err)
	}
	if len(svc.Spec.Selector) == 0 {
		return "", fmt.Errorf("service %s has no selector", svcName)
	}
	var parts []string
	for k, v := range svc.Spec.Selector {
		parts = append(parts, k+"="+v)
	}
	pods, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: strings.Join(parts, ",")})
	if err != nil {
		return "", fmt.Errorf("list pods for %s: %w", svcName, err)
	}
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodRunning {
			return p.Name, nil
		}
	}
	return "", fmt.Errorf("no running pod for service %s", svcName)
}

// doPortForwardGoRaw opens a Go port-forward to a pod, makes an HTTP request,
// and returns the raw response body (or status code string when e.StatusCode != 0).
func doPortForwardGoRaw(ctx context.Context, cfg *rest.Config, workDir, ns, pod string, e types.E2EKubectlPortForward, env types.EnvResolver) (string, error) {
	localPort, err := freeLocalPort()
	if err != nil {
		return "", fmt.Errorf("port-forward: free port: %w", err)
	}

	pfURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/portforward", cfg.Host, ns, pod)
	req, err := http.NewRequest(http.MethodPost, pfURL, nil)
	if err != nil {
		return "", fmt.Errorf("port-forward url: %w", err)
	}
	transport, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return "", fmt.Errorf("port-forward transport: %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL)

	stopChan := make(chan struct{})
	readyChan := make(chan struct{}, 1)
	fw, err := portforward.New(dialer,
		[]string{fmt.Sprintf("%s:%d", localPort, e.Port)},
		stopChan, readyChan, io.Discard, io.Discard,
	)
	if err != nil {
		return "", fmt.Errorf("port-forward: %w", err)
	}

	fwErrCh := make(chan error, 1)
	go func() { fwErrCh <- fw.ForwardPorts() }()
	defer close(stopChan)

	select {
	case <-readyChan:
	case err := <-fwErrCh:
		return "", fmt.Errorf("port-forward pod/%s: %w", pod, err)
	case <-time.After(portForwardTimeout):
		return "", fmt.Errorf("port-forward pod/%s: timed out waiting for ready", pod)
	case <-ctx.Done():
		return "", ctx.Err()
	}

	if e.Path == "" {
		return "", nil
	}

	if e.Wait != "" {
		if d, err := parseTimeDuration(e.Wait); err == nil && d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}

	method := e.Method
	if method == "" {
		method = "GET"
	}
	reqURL := fmt.Sprintf("http://localhost:%s%s", localPort, e.Path)
	sp := startSpinner(fmt.Sprintf("%s %s (→ pod/%s)", method, reqURL, pod))

	var reqBody io.Reader
	switch {
	case e.Body != "" && e.BodyFile != "":
		sp.Stop()
		return "", fmt.Errorf("port-forward %s: body and bodyFile are mutually exclusive", e.Path)
	case e.Body != "":
		reqBody = strings.NewReader(os.Expand(e.Body, env.Lookup))
	case e.BodyFile != "":
		p, err := toAbs(e.BodyFile, workDir)
		if err != nil {
			return "", err
		}
		raw, err := readLocal(p)
		if err != nil {
			sp.Stop()
			return "", fmt.Errorf("port-forward %s: bodyFile %q: %w", e.Path, e.BodyFile, err)
		}
		reqBody = strings.NewReader(os.Expand(string(raw), env.Lookup))
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		sp.Stop()
		return "", fmt.Errorf("port-forward request: %w", err)
	}
	for k, v := range e.Headers {
		httpReq.Header.Set(k, os.Expand(v, env.Lookup))
	}
	if e.BodyFile != "" {
		if _, set := e.Headers["Content-Type"]; !set {
			if ct := defaultContentType(e.BodyFile); ct != "" {
				httpReq.Header.Set("Content-Type", ct)
			}
		}
	}
	resp, err := http.DefaultClient.Do(httpReq)
	sp.Stop()
	if err != nil {
		return "", fmt.Errorf("port-forward %s: %w", reqURL, err)
	}
	defer resp.Body.Close()

	if e.StatusCode != 0 {
		return strconv.Itoa(resp.StatusCode), nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("port-forward %s: reading response: %w", reqURL, err)
	}
	return strings.TrimSpace(string(body)), nil
}

// assertPortForwardOutput applies json/jq/yq extraction and assertions to raw curl output.
func assertPortForwardOutput(ctx context.Context, workDir, raw string, e types.E2EKubectlPortForward, target string) error {
	out, err := applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    raw,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ, YQ: e.YQ},
	})

	if err != nil {
		return fmt.Errorf("kubectl port-forward %s%s: %w", target, e.Path, err)
	}

	if err := e.JSON.Check(out); err != nil {
		return fmt.Errorf("kubectl port-forward %s%s: %w\noutput: %s", target, e.Path, err, out)
	}

	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("kubectl port-forward %s%s: %w\noutput: %s", target, e.Path, err, out)
	}
	return nil
}

func defaultContentType(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	default:
		return "" // let the server guess; or default to json
	}
}

// freeLocalPort finds an available local TCP port by binding to :0 and releasing it.
func freeLocalPort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	return port, err
}
