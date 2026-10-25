package migrate

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/rs/zerolog"
)

// Run runs every migration bp declares, or only Kind's when set. Results are
// in blueprint order; a kind-level error is returned with the results so far.
func (m *Migrator) Run(ctx context.Context, bp *blueprint.Blueprint) ([]*KindMigrateResult, error) {
	var results []*KindMigrateResult
	matched := false
	for _, name := range bp.CRDNames() {
		entry, ok := bp.CRDEntry(name)
		if !ok || !entry.HasAnyMigrate() {
			continue
		}
		if m.Kind != "" && entry.GVK().Kind != m.Kind {
			continue
		}
		matched = true
		r, err := m.runFor(ctx, &entry)
		if r != nil {
			results = append(results, r)
		}
		if err != nil {
			return results, err
		}
	}
	if m.Kind != "" && !matched {
		return nil, fmt.Errorf("no CRD of kind %q declares a migration", m.Kind)
	}
	return results, nil
}

// Apply migrates all CRDs in bp that declare migrate.from fields.
// Returns an error on the first kind-level failure; the caller decides whether
// to fatal or continue.
func Apply(ctx context.Context, bp *blueprint.Blueprint, kube kubeclient.Interface) error {
	m := New(ctx, kube, bp.UserFunctions(), ModeForward, MigratorOpts{Strict: true})
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
