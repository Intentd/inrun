package migrate

import (
	"context"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
)

// Run runs the migrator over every CRD in cat that declares migrate.from
// fields. Results are returned in catalog order; a kind-level error is
// returned alongside any results already collected.
func (m *Migrator) Run(ctx context.Context, cat *catalog.Catalog) ([]*KindMigrateResult, error) {
	var results []*KindMigrateResult
	for _, name := range cat.CRDNames() {
		entry, ok := cat.CRDEntry(name)
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

// Apply migrates all CRDs in cat that declare migrate.from fields.
// Returns an error on the first kind-level failure; the caller decides whether
// to fatal or continue.
func Apply(ctx context.Context, cat *catalog.Catalog, kube kubeclient.Interface) error {
	m := New(ctx, kube, cat.UserFunctions(), ModeForward, MigratorOpts{Strict: true})
	results, err := m.Run(ctx, cat)
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
