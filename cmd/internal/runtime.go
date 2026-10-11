package internal

import (
	"context"

	"github.com/inrundev/inrun/pkg/catalog"
	"github.com/inrundev/inrun/pkg/config"
	"github.com/inrundev/inrun/pkg/labels"
	"github.com/inrundev/inrun/pkg/logger"
	"github.com/inrundev/inrun/pkg/merger"
	"github.com/inrundev/inrun/pkg/runtime/leader"
	"github.com/inrundev/inrun/pkg/template"
	"github.com/inrundev/inrun/pkg/utils"
	"github.com/inrundev/inrun/pkg/version"
)

// RunRuntime builds and runs the runtime until ctx ends. Each onCatalog func is
// called with the validated Catalog once it is built, before the runtime
// starts; the dev CLI uses it to start the console and gateway.
func RunRuntime(cfg *config.Config, m *merger.Merger, ctx context.Context, onCatalog ...func(*catalog.Catalog)) {
	// Runtime facts, available to every template expression as .inrun.*. Set on
	// the context that construction, startup and the coordinator all share.
	ctx = template.ContextWithInrunContext(ctx, template.NewInrunContext(
		cfg.Cluster().Namespace(),
		version.Version,
	))

	// create domain component and build inrun
	startup := constructRuntime(cfg, m, ctx)
	for _, f := range onCatalog {
		f(startup.catalog)
	}

	// ── Start ─────────────────────────────────────────────────────────────────
	go func() {
		if err := startup.inrun.Start(ctx); err != nil {
			logger.Fatal().AnErr("inrun startup error", err)
			utils.Exit(err)
		}
	}()

	ko := leader.NewLeaderElection(
		startup.kube,
		startup.event,
		func(ctx context.Context) { startup.cord.Coordinate(ctx) },
		func(leader string) {
			// Banner prints here — leader is the actual winner
			printBanner(startup, leader)
		},
		leader.Options{
			Namespace:     cfg.Leader().Namespace(),
			LeaseDuration: cfg.Leader().LeaseDuration(),
			RenewDeadline: cfg.Leader().RenewDeadline(),
			RetryPeriod:   cfg.Leader().RetryPeriod(),
			Labels:        labels.WithDeletionProtection(nil),
		})

	// start leader election as postStartHook after  inrun is ready
	startup.inrun.AddPostStartHook(ko, func(ctx context.Context) {
		logger.Info().Msg("starting leader election...")
		ko.Start(ctx)
	})

	// Keep running until cancelled
	startup.inrun.Wait()
}
