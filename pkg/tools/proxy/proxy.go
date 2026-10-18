package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/intentd/inrun/pkg/util/term"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// ForwardTarget describes one component to port-forward.
type ForwardTarget struct {
	Label     string // display name, e.g. "Runtime"
	Component string // ComponentRuntime | ComponentConsole | ComponentGateway — empty when ServiceName is set
	// ServiceName looks up a Service by exact name instead of the component
	// label — for non-Inrun components like the devserver. Takes precedence
	// over Component when set.
	ServiceName string
	Namespace   string
	LocalPort   int
	Scheme      string // "http" or "https"
	ViaLease    bool   // true for Runtime: resolve pod from Lease, probe health before declaring connected
	// FlagName is the CLI flag suffix used in "use --<FlagName>-port" hints
	// (e.g. "cc" for --console-port). Falls back to Component when empty.
	FlagName string
}

// RunAll discovers and forwards all targets, printing status to out.
// It blocks until ctx is cancelled or all targets report not-deployed.
func RunAll(ctx context.Context, cfg *rest.Config, cs kubernetes.Interface, targets []ForwardTarget, out io.Writer) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	notDeployed := make(chan struct{}, len(targets))
	for _, t := range targets {
		go func(t ForwardTarget) {
			forwardWithReconnect(ctx, cfg, cs, t, out, notDeployed)
		}(t)
	}

	// Exit early if every target reports not-deployed.
	go func() {
		for range targets {
			select {
			case <-notDeployed:
			case <-ctx.Done():
				return
			}
		}
		cancel()
	}()

	<-ctx.Done()
}

func forwardWithReconnect(ctx context.Context, cfg *rest.Config, cs kubernetes.Interface, t ForwardTarget, out io.Writer, notDeployed chan<- struct{}) {
	var prevPod string
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Show reconnecting line once per retry cycle (not on first attempt).
		if prevPod != "" {
			fmt.Fprintf(out, "  %s %-14s reconnecting...  (was %s)\n", term.Yellow("↺"), t.Label, prevPod)
		}

		pod, ns, err := resolveTarget(ctx, cs, t)
		if err != nil {
			fmt.Fprintf(out, "  %s %-14s %v\n", term.Yellow("✗"), t.Label, err)
			if isNotDeployed(err) {
				notDeployed <- struct{}{}
				return
			}
			sleep(ctx, 2*time.Second)
			continue
		}

		stop, done, err := startForward(ctx, cfg, ns, pod, t.LocalPort, int(resolveRemotePort(t)))
		if err != nil {
			sleep(ctx, 2*time.Second)
			continue
		}

		// For lease-based targets (Runtime), probe health before declaring connected.
		// The SPDY handshake can succeed against a dead pod while the Lease transition
		// is in progress — probing confirms the pod is actually serving.
		// Loop until the probe passes or ctx is cancelled (no fixed timeout).
		if t.ViaLease && !probeReady(ctx, t.Scheme, t.LocalPort) {
			stop()
			<-done
			sleep(ctx, 2*time.Second)
			continue
		}

		if prevPod == "" {
			fmt.Fprintf(out, "  %s %-14s %s://localhost:%d   (%s)\n", term.Green("✓"), t.Label, t.Scheme, t.LocalPort, pod)
		} else {
			fmt.Fprintf(out, "  %s %-14s %s://localhost:%d   (%s)  [reconnected]\n", term.Green("✓"), t.Label, t.Scheme, t.LocalPort, pod)
		}
		prevPod = pod

		// Watch the pod independently so that when it disappears we call stop()
		// immediately rather than waiting for traffic to surface the broken tunnel.
		watchCtx, watchCancel := context.WithCancel(ctx)
		go watchPod(watchCtx, cs, t.Namespace, pod, stop)
		<-done
		watchCancel()
		stop()
	}
}

func resolveTarget(ctx context.Context, cs kubernetes.Interface, t ForwardTarget) (pod, ns string, err error) {
	var svc *FoundService
	if t.ServiceName != "" {
		svc, err = FindServiceByName(ctx, cs, t.Namespace, t.ServiceName)
	} else {
		svc, err = FindService(ctx, cs, t.Namespace, t.Component)
	}
	if err != nil {
		return "", "", err
	}
	if svc == nil {
		return "", "", fmt.Errorf("not deployed in %s: %w", t.Namespace, errNotDeployed)
	}

	if t.ViaLease {
		pod, err = ResolveRuntimePod(ctx, cs, t.Namespace)
		if err != nil {
			return "", "", err
		}
	} else {
		pod, err = ResolvePod(ctx, cs, t.Namespace, svc.Name)
		if err != nil {
			return "", "", err
		}
	}
	return pod, t.Namespace, nil
}

// resolveRemotePort returns the container port for the target.
func resolveRemotePort(t ForwardTarget) int32 {
	switch t.Component {
	case ComponentRuntime:
		return 8080
	case ComponentConsole:
		return 8081
	case ComponentGateway:
		return 8080
	case DevServer:
		return 9999
	}
	return int32(t.LocalPort)
}

// startForward establishes a SPDY port-forward and returns once the tunnel is ready.
// The caller must call stop() when done, and drain done to avoid goroutine leak.
func startForward(ctx context.Context, cfg *rest.Config, ns, pod string, localPort, remotePort int) (stop func(), done <-chan error, err error) {
	pfURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/portforward", cfg.Host, ns, pod)
	req, err := http.NewRequest(http.MethodPost, pfURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build portforward url: %w", err)
	}

	transport, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("spdy transport: %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL)

	stopChan := make(chan struct{})
	readyChan := make(chan struct{}, 1)
	errChan := make(chan error, 1)

	var once sync.Once
	stopFn := func() { once.Do(func() { close(stopChan) }) }

	go func() {
		<-ctx.Done()
		stopFn()
	}()

	fw, err := portforward.New(dialer,
		[]string{fmt.Sprintf("%d:%d", localPort, remotePort)},
		stopChan, readyChan,
		io.Discard, io.Discard,
	)
	if err != nil {
		stopFn()
		return nil, nil, fmt.Errorf("portforward: %w", err)
	}

	go func() {
		errChan <- fw.ForwardPorts()
	}()

	// Wait for the tunnel to be ready before returning.
	select {
	case <-readyChan:
	case err := <-errChan:
		stopFn()
		return nil, nil, err
	case <-ctx.Done():
		stopFn()
		return nil, nil, ctx.Err()
	}

	return stopFn, errChan, nil
}

// probeReady polls scheme://localhost:port/health until it returns 2xx or ctx is cancelled.
func probeReady(ctx context.Context, scheme string, port int) bool {
	url := fmt.Sprintf("%s://localhost:%d/health", scheme, port)
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 300 {
				return true
			}
		}
		sleep(ctx, 500*time.Millisecond)
	}
}

// watchPod polls the given pod every 3 seconds and calls stop() as soon as
// the pod is no longer Running, triggering proactive reconnection.
func watchPod(ctx context.Context, cs kubernetes.Interface, ns, pod string, stop func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
		p, err := cs.CoreV1().Pods(ns).Get(ctx, pod, metav1.GetOptions{})
		if err != nil || p.Status.Phase != corev1.PodRunning {
			stop()
			return
		}
	}
}

var errNotDeployed = errors.New("not deployed")

func isNotDeployed(err error) bool {
	return errors.Is(err, errNotDeployed)
}

// sleep waits for d or until ctx is cancelled.
func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
