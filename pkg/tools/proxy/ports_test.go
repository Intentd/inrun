package proxy_test

import (
	"net"
	"testing"

	"github.com/intentd/inrun/pkg/tools/proxy"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestCheckPort_Free(t *testing.T) {
	port := freePort(t)
	if err := proxy.CheckPort(port); err != nil {
		t.Errorf("unexpected error for a free port: %v", err)
	}
}

func TestCheckPort_InUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("binding a port: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	if err := proxy.CheckPort(port); err == nil {
		t.Fatal("expected error — port is already bound")
	}
}
