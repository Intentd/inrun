package kubectl

import (
	"context"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"github.com/intentd/inrun/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Two gateway replicas: only the one that did the work logs its result.
func gatewayPods(t *testing.T) *kubefake.Clientset {
	t.Helper()
	pod := func(name string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "inrun-system", Labels: map[string]string{"app": "gateway"}}}
	}
	prev := fetchPodLog
	fetchPodLog = func(_ context.Context, o LogsOptions) (string, error) {
		if o.Target == "gateway-a" {
			return "schema migration: complete\n\"migrated\":3", nil
		}
		return "schema migration: complete\n\"migrated\":0,\"skipped\":3", nil
	}
	t.Cleanup(func() { fetchPodLog = prev })
	return kubefake.NewClientset(pod("gateway-b"), pod("gateway-a"))
}

func logsEntry(pods types.E2ELogsPods, contains string) types.E2EKubectlLogs {
	e := types.E2EKubectlLogs{LabelSelector: "app=gateway", Namespace: "inrun-system", Pods: pods}
	e.Assert = &types.Assertions{Contains: contains}
	return e
}

func TestReadLogs_PodsAnyPassesWhenOnePodLogsIt(t *testing.T) {
	cs := gatewayPods(t)
	out, err := readLogs(context.Background(), cs, logsEntry(types.E2ELogsPodsAny, `"migrated":3`), types.NoopEnvResolver(), "")
	if err != nil {
		t.Fatalf("pods: any: %v, want a pass: gateway-a logged it", err)
	}
	if !strings.Contains(out, "── pod/gateway-a") || !strings.Contains(out, "── pod/gateway-b") {
		t.Errorf("output %q, want each pod's log under its name", out)
	}
}

func TestReadLogs_PodsAllNamesThePodThatFailed(t *testing.T) {
	cs := gatewayPods(t)
	if _, err := readLogs(context.Background(), cs, logsEntry(types.E2ELogsPodsAll, "schema migration: complete"), types.NoopEnvResolver(), ""); err != nil {
		t.Fatalf("pods: all on a line every pod logs: %v", err)
	}
	_, err := readLogs(context.Background(), cs, logsEntry(types.E2ELogsPodsAll, `"migrated":3`), types.NoopEnvResolver(), "")
	if err == nil || !strings.Contains(err.Error(), "1 of 2 pods failed") || !strings.Contains(err.Error(), "pod/gateway-b") {
		t.Fatalf("err = %v, want gateway-b named as the one that failed", err)
	}
}

func TestReadLogs_PodsWithNoMatchingPodsFails(t *testing.T) {
	cs := kubefake.NewClientset()
	_, err := readLogs(context.Background(), cs, logsEntry(types.E2ELogsPodsAny, "x"), types.NoopEnvResolver(), "")
	if err == nil || !strings.Contains(err.Error(), "no pods match") {
		t.Fatalf("err = %v, want no pods match", err)
	}
}

func TestValidateKubectlLogs_Pods(t *testing.T) {
	bad := logsEntry("some", "x")
	if errs := validateKubectlLogs("loc", bad); len(errs) == 0 {
		t.Error(`pods: "some" accepted`)
	}
	named := types.E2EKubectlLogs{Name: "p", Pods: types.E2ELogsPodsAll}
	named.Contains = "x"
	if errs := validateKubectlLogs("loc", named); len(errs) == 0 {
		t.Error("pods without labelSelector accepted")
	}
	if errs := validateKubectlLogs("loc", logsEntry(types.E2ELogsPodsAll, "x")); len(errs) != 0 {
		t.Errorf("valid pods: all: %v", errs)
	}
}
