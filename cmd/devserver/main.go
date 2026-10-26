// Package main is the standalone entry point for the Inrun dev server.
// It starts the mock HTTP server on the configured port and blocks until the
// process is killed. Runs as ghcr.io/intentd/inrun-dev-server:latest.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/intentd/inrun/pkg/tools/devserver"
)

func main() {
	if err := devserver.Start(devserver.Port); err != nil {
		fmt.Fprintf(os.Stderr, "dev server: %v\n", err)
		os.Exit(1)
	}

	// Block until SIGTERM or SIGINT.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig
}
