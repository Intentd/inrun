// Command inrun is the Inrun CLI.
//
// Bare inrun runs the Blueprint in the current directory (or -f), and
// inrun <name>:<version> runs a published pattern. Subcommands validate,
// simulate, test end to end, generate install manifests, and push and pull
// patterns; run inrun --help for the full list.
//
// Built with -tags runtime or -tags gateway, the binary contains only the
// runtime or the gateway, for the in-cluster images.
package main

import (
	"context"

	"github.com/intentd/inrun/pkg/util/term"

	"github.com/intentd/inrun/cmd/cli"
	"github.com/intentd/inrun/pkg/config"
)

func main() {
	cfg, err := config.Init()
	if err != nil {
		term.FatalExit("failed to load configurations", err)
	}

	// define root context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli.Execute(cfg, ctx)
}
