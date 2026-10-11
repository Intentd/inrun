package proxy_test

import (
	"context"
	"testing"

	"github.com/intentd/inrun/pkg/tools/proxy"
	"github.com/intentd/inrun/pkg/types"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFindService_Found(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inrun-runtime",
			Namespace: "inrun-system",
			Labels:    map[string]string{"inrun.dev/component": proxy.ComponentRuntime},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 8080}},
		},
	})

	svc, err := proxy.FindService(context.Background(), cs, "inrun-system", proxy.ComponentRuntime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected a service, got nil")
	}
	if svc.Name != "inrun-runtime" || svc.Port != 8080 {
		t.Errorf("got %+v, want Name=inrun-runtime Port=8080", svc)
	}
}

func TestFindService_NotDeployed(t *testing.T) {
	cs := fake.NewSimpleClientset()

	svc, err := proxy.FindService(context.Background(), cs, "inrun-system", proxy.ComponentGateway)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != nil {
		t.Errorf("expected nil service for a component with no matching Service, got %+v", svc)
	}
}

func TestFindService_NoPorts(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inrun-console",
			Namespace: "inrun-system",
			Labels:    map[string]string{"inrun.dev/component": proxy.ComponentConsole},
		},
	})

	svc, err := proxy.FindService(context.Background(), cs, "inrun-system", proxy.ComponentConsole)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != nil {
		t.Errorf("expected nil service for a Service with no ports, got %+v", svc)
	}
}

func TestResolvePod_ReturnsRunningPod(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "inrun-runtime", Namespace: "inrun-system"},
			Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "inrun-runtime"}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "inrun-runtime-abc", Namespace: "inrun-system", Labels: map[string]string{"app": "inrun-runtime"}},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "inrun-runtime-def", Namespace: "inrun-system", Labels: map[string]string{"app": "inrun-runtime"}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)

	pod, err := proxy.ResolvePod(context.Background(), cs, "inrun-system", "inrun-runtime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pod != "inrun-runtime-def" {
		t.Errorf("got pod %q, want the Running one (inrun-runtime-def)", pod)
	}
}

func TestResolvePod_NoRunningPod(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "inrun-runtime", Namespace: "inrun-system"},
			Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "inrun-runtime"}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "inrun-runtime-abc", Namespace: "inrun-system", Labels: map[string]string{"app": "inrun-runtime"}},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
	)

	if _, err := proxy.ResolvePod(context.Background(), cs, "inrun-system", "inrun-runtime"); err == nil {
		t.Fatal("expected error — no pod is Running")
	}
}

func TestResolvePod_NoSelector(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "inrun-runtime", Namespace: "inrun-system"},
	})

	if _, err := proxy.ResolvePod(context.Background(), cs, "inrun-system", "inrun-runtime"); err == nil {
		t.Fatal("expected error — service has no selector")
	}
}

func TestResolveRuntimePod_ReturnsHolder(t *testing.T) {
	holder := "inrun-runtime-def"
	cs := fake.NewSimpleClientset(&coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: types.LeaderLeaseName, Namespace: "inrun-system"},
		Spec:       coordinationv1.LeaseSpec{HolderIdentity: &holder},
	})

	pod, err := proxy.ResolveRuntimePod(context.Background(), cs, "inrun-system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pod != holder {
		t.Errorf("got %q, want %q", pod, holder)
	}
}

func TestResolveRuntimePod_NoHolder(t *testing.T) {
	cs := fake.NewSimpleClientset(&coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: types.LeaderLeaseName, Namespace: "inrun-system"},
	})

	if _, err := proxy.ResolveRuntimePod(context.Background(), cs, "inrun-system"); err == nil {
		t.Fatal("expected error — lease has no holder identity yet")
	}
}

func TestResolveRuntimePod_MissingLease(t *testing.T) {
	cs := fake.NewSimpleClientset()

	if _, err := proxy.ResolveRuntimePod(context.Background(), cs, "inrun-system"); err == nil {
		t.Fatal("expected error — lease does not exist")
	}
}
