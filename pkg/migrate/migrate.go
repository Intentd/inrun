package migrate

import (
	"context"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/tools/kubeclient"
	"github.com/intentd/tools/logger"
	"github.com/rs/zerolog"
)

// Apply runs every migration bp declares, forward, as the gateway does at
// startup. It never prunes: migrate.lifecycle is skipped with a warning, since
// looking for a source's children needs access to every kind, which the
// gateway's role does not have; `inrun migrate` prunes. Returns an error on
// the first kind-level failure; the caller decides whether to fatal or
// continue.
func Apply(ctx context.Context, bp *blueprint.Blueprint, kube kubeclient.Interface) error {
	log := logger.FromContext(ctx)
	for _, name := range bp.CRDNames() {
		if entry, ok := bp.CRDEntry(name); ok && entry.HasMigrate() && entry.Migrate.Prunes() {
			log.Warn().Str("crd", name).Msg("migrate.lifecycle.prune is not run at startup; sources are kept. Run inrun migrate to prune them")
		}
	}
	m := New(ctx, kube, bp.UserFunctions(), ModeForward, MigratorOpts{Strict: true, NoLifecycle: true})
	results, err := m.Run(ctx, bp)
	for _, r := range results {
		logMigrationResult(ctx, r)
	}
	return err
}

func logMigrationResult(ctx context.Context, result *KindMigrateResult) {
	if result == nil {
		return
	}
	log := logger.FromContext(ctx)
	var migrated, skipped, excluded, drifted, pruned, failed int
	for _, cr := range result.CRs {
		ev := func(e *zerolog.Event) *zerolog.Event {
			e = e.Str("kind", result.Kind).Str("namespace", cr.Namespace).Str("name", cr.Name)
			if result.To != "" {
				e = e.Str("to", result.To)
			}
			return e.Bool("pruned", cr.Pruned)
		}
		if cr.Pruned && cr.Err == nil {
			pruned++
		}
		switch {
		case cr.Err != nil:
			failed++
			ev(log.Error()).Err(cr.Err).Msg("schema migration: CR failed")
		case cr.Drifted:
			drifted++
			ev(log.Warn()).Msg("schema migration: destination exists and differs, skipped")
		case cr.Excluded:
			excluded++
			ev(log.Debug()).Msg("schema migration: CR not selected by migrate.selection")
		case cr.Skipped:
			skipped++
			ev(log.Debug()).Msg("schema migration: CR already up to date")
		case len(cr.Fields) > 0:
			migrated++
			ev(log.Info()).Strs("fields", cr.Fields).Msg("schema migration: CR migrated")
		default:
			ev(log.Info()).Msg("schema migration: CR already migrated")
		}
	}
	e := log.Info().Str("kind", result.Kind)
	if result.To != "" {
		e = e.Str("to", result.To)
	}
	e.Int("migrated", migrated).
		Int("skipped", skipped).
		Int("excluded", excluded).
		Int("drifted", drifted).
		Int("pruned", pruned).
		Int("failed", failed).
		Msg("schema migration: complete")
}
