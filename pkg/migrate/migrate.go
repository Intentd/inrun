package migrate

import (
	"context"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
)

// Run runs the migrator over every CRD in bp that declares migrate.from
// fields. Results are returned in blueprint order; a kind-level error is
// returned alongside any results already collected.
func (m *Migrator) Run(ctx context.Context, bp *blueprint.Blueprint) ([]*KindMigrateResult, error) {
	var results []*KindMigrateResult
	for _, name := range bp.CRDNames() {
		entry, ok := bp.CRDEntry(name)
		if !ok {
			continue
		}
		r, err := m.runFor(ctx, &entry)
		results = append(results, r)
		if err != nil {
			return results, err
		}
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
	migrated, skipped, failed := 0, 0, 0
	for _, cr := range result.CRs {
		switch {
		case cr.Err != nil:
			failed++
		case cr.Skipped:
			skipped++
			log.Debug().
				Str("kind", result.Kind).
				Str("namespace", cr.Namespace).
				Str("name", cr.Name).
				Msg("schema migration: CR already up to date")
		default:
			migrated++
			log.Info().
				Str("kind", result.Kind).
				Str("namespace", cr.Namespace).
				Str("name", cr.Name).
				Strs("fields", cr.Fields).
				Msg("schema migration: CR migrated")
		}
	}
	log.Info().
		Str("kind", result.Kind).
		Int("migrated", migrated).
		Int("skipped", skipped).
		Int("failed", failed).
		Msg("schema migration: complete")
}
