package e2e

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func TestAssertKubectlGetOutput(t *testing.T) {
	e := types.E2EKubectlGet{Kind: "Website", Name: "site-a", OutputContains: "Ready"}
	if err := assertKubectlGetOutput("phase: Ready", e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertKubectlGetOutput("phase: Pending", e); err == nil {
		t.Fatal("expected error — output does not contain \"Ready\"")
	}
}

func TestAssertKubectlGetOutput_Exists(t *testing.T) {
	e := types.E2EKubectlGet{Kind: "Website", Name: "site-a", NotExists: true}
	if err := assertKubectlGetOutput("", e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertKubectlGetOutput("some-value", e); err == nil {
		t.Fatal("expected error — output is non-empty but NotExists was set")
	}
}

func TestAssertKubectlLogsOutput(t *testing.T) {
	e := types.E2EKubectlLogs{OutputContains: "started"}
	if err := assertKubectlLogsOutput("server started on :8080", e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertKubectlLogsOutput("panic: something broke", e); err == nil {
		t.Fatal("expected error — output does not contain \"started\"")
	}
}

func TestAssertKubectlDescribeOutput(t *testing.T) {
	e := types.E2EKubectlDescribe{Kind: "Pod", Name: "site-a-0", OutputContains: "Status:        Running"}
	if err := assertKubectlDescribeOutput("Status:        Running", e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertKubectlDescribeOutput("Status:        Pending", e); err == nil {
		t.Fatal("expected error — output does not contain the expected status line")
	}
}

func TestAssertKubectlExecOutput(t *testing.T) {
	e := types.E2EKubectlExec{Command: []string{"cat", "/etc/hostname"}, Equals: "site-a-0"}
	if err := assertKubectlExecOutput("site-a-0", "site-a-0", e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertKubectlExecOutput("site-a-0", "site-b-0", e); err == nil {
		t.Fatal("expected error — output does not equal \"site-a-0\"")
	}
}

func TestAssertKubectlEventsOutput(t *testing.T) {
	e := types.E2EKubectlEvents{Kind: "Website", Name: "site-a", OutputContains: "Reconciled"}
	if err := assertKubectlEventsOutput("1m  Normal  Reconciled  website/site-a  Reconciled successfully", e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertKubectlEventsOutput("1m  Warning  Failed  website/site-a  admission denied", e); err == nil {
		t.Fatal("expected error — output does not contain \"Reconciled\"")
	}
}

func TestAssertKubectlCpOutput(t *testing.T) {
	e := types.E2EKubectlCp{Src: "/data/config.json", OutputContains: `"env":"prod"`}
	if err := assertKubectlCpOutput("/data/config.json", `{"env":"prod"}`, e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertKubectlCpOutput("/data/config.json", `{"env":"staging"}`, e); err == nil {
		t.Fatal("expected error — output does not contain the expected substring")
	}
}

func TestAssertKubectlTopOutput(t *testing.T) {
	e := types.E2EKubectlTop{Kind: "pod", OutputContains: "site-a-0"}
	if err := assertKubectlTopOutput("pod", "NAME       CPU   MEMORY\nsite-a-0   5m    32Mi", e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertKubectlTopOutput("pod", "NAME       CPU   MEMORY\nsite-b-0   5m    32Mi", e); err == nil {
		t.Fatal("expected error — output does not contain \"site-a-0\"")
	}
}
