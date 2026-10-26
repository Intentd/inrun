package proxy

import (
	"errors"
	"fmt"
	"testing"
)

func TestResolveRemotePort(t *testing.T) {
	tests := []struct {
		component string
		want      int32
		localPort int
	}{
		{ComponentRuntime, 8080, 9000},
		{ComponentGateway, 8080, 9000},
		{ComponentConsole, 8081, 9000},
		{"unknown", 9000, 9000},
	}
	for _, tt := range tests {
		got := resolveRemotePort(ForwardTarget{Component: tt.component, LocalPort: tt.localPort})
		if got != tt.want {
			t.Errorf("resolveRemotePort(%q) = %d, want %d", tt.component, got, tt.want)
		}
	}
}

func TestIsNotDeployed(t *testing.T) {
	if !isNotDeployed(errNotDeployed) {
		t.Error("expected isNotDeployed(errNotDeployed) to be true")
	}
	if !isNotDeployed(fmt.Errorf("not deployed in inrun-system: %w", errNotDeployed)) {
		t.Error("expected isNotDeployed to unwrap a wrapped errNotDeployed")
	}
	if isNotDeployed(errors.New("some other error")) {
		t.Error("expected isNotDeployed to be false for an unrelated error")
	}
}
