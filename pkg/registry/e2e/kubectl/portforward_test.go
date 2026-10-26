package kubectl

import (
	"context"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func TestAssertPortForwardOutput(t *testing.T) {
	e := types.E2EKubectlPortForward{Path: "/healthz", E2EAssertions: types.E2EAssertions{Contains: "ok"}}
	if err := assertPortForwardOutput(context.Background(), "", `{"status":"ok"}`, e, "pod/inrun-runtime-0"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertPortForwardOutput(context.Background(), "", `{"status":"degraded"}`, e, "pod/inrun-runtime-0"); err == nil {
		t.Fatal("expected error — output does not contain \"ok\"")
	}
}

func TestAssertPortForwardOutput_StatusCode(t *testing.T) {
	e := types.E2EKubectlPortForward{Path: "/healthz", StatusCode: 200, E2EAssertions: types.E2EAssertions{Equals: "200"}}
	if err := assertPortForwardOutput(context.Background(), "", "200", e, "pod/inrun-runtime-0"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := assertPortForwardOutput(context.Background(), "", "503", e, "pod/inrun-runtime-0"); err == nil {
		t.Fatal("expected error — status code 503 does not equal \"200\"")
	}
}
