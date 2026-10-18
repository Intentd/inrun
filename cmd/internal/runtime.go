package internal

import (
	"context"

	"github.com/intentd/inrun/pkg/util/term"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/runtime/app"
	"github.com/intentd/inrun/pkg/runtime/leader"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/version"
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

	rt, err := app.New(cfg, m, ctx)
	if err != nil {
		term.FatalExit("runtime startup failed", err)
	}

	if rt == nil {
		term.FatalExit("app.New returned no runtime", nil)
	}

	for _, f := range onCatalog {
		f(rt.Catalog)
	}

	// ── Start ─────────────────────────────────────────────────────────────────
	go func() {
		if err := rt.Process.Start(ctx); err != nil {
			term.FatalExit("inrun startup error", err)
		}
	}()

	ko := leader.NewLeaderElection(
		rt.Kube,
		rt.Event,
		func(ctx context.Context) { rt.Coordinator.Coordinate(ctx) },
		func(leader string) {
			// Only the replica that won gets here.
			logger.Info().
				Str("leader", leader).
				Str("namespace", cfg.Cluster().Namespace()).
				Int("crds", len(rt.Catalog.Enabled())).
				Bool("webhooks", rt.Catalog.HasMutationRules() || rt.Catalog.HasValidationRules() ||
					rt.Catalog.IsDeletionProtectionEnabled() || rt.Catalog.IsNamespaceProtectionEnabled()).
				Msg("leading: reconciling")
		},
		leader.Options{
			Namespace:     cfg.Leader().Namespace(),
			LeaseDuration: cfg.Leader().LeaseDuration(),
			RenewDeadline: cfg.Leader().RenewDeadline(),
			RetryPeriod:   cfg.Leader().RetryPeriod(),
			Labels:        labels.WithDeletionProtection(nil),
		})

	// start leader election as postStartHook after  inrun is ready
	rt.Process.AddPostStartHook(ko, func(ctx context.Context) {
		logger.Info().Msg("starting leader election...")
		ko.Start(ctx)
	})

	// Keep running until cancelled
	rt.Process.Wait()
}
