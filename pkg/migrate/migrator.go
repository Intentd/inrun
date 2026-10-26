package migrate

import (
	"context"
	"fmt"

	client "github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/migrate/core"
	"github.com/intentd/inrun/pkg/migrate/field"
	"github.com/intentd/inrun/pkg/migrate/kind"
	"github.com/intentd/inrun/pkg/types"
	kubeutil "github.com/intentd/inrun/pkg/util/kube"
	"github.com/rs/zerolog"
)

// Migrator runs field-level and kind-level migrations over a blueprint.
type Migrator struct {
	core.Runner
	// Kind, when set, limits the run to the CRD with this kind.
	Kind string
}

type (
	MigrateMode       = core.Mode
	CRMigrateResult   = core.CRResult
	KindMigrateResult = core.KindResult
	Selection         = core.Selection
)

const (
	ModeForward  = core.ModeForward
	ModeRollback = core.ModeRollback
	ModeClean    = core.ModeClean
)

// MigratorOpts are the settings shared by forward, rollback and clean runs.
type MigratorOpts struct {
	DryRun     bool
	Strict     bool
	PageSize   int64
	Kind       string
	Namespaces []string
	Name       string
	Selector   string
	Force      bool
	Log        *zerolog.Logger
	Progress   func(label string, done, total int)
}

// New returns a Migrator for mode. A nil opts.Log uses the context logger.
func New(ctx context.Context, kube client.Interface, funcs types.KELFunctionRegistry, mode MigrateMode, opts MigratorOpts) *Migrator {
	log := opts.Log
	if log == nil {
		log = logger.FromContext(ctx)
	}
	return &Migrator{Kind: opts.Kind, Runner: core.Runner{
		Kube:       kube,
		Functions:  funcs,
		Mode:       mode,
		DryRun:     opts.DryRun,
		Strict:     opts.Strict,
		PageSize:   opts.PageSize,
		Namespaces: opts.Namespaces,
		Name:       opts.Name,
		Selector:   opts.Selector,
		Force:      opts.Force,
		Log:        log,
		Progress:   opts.Progress,
	}}
}

// runFor runs the migration crd declares. Validation allows at most one level.
func (m *Migrator) runFor(ctx context.Context, crd *types.CRDEntry) (*KindMigrateResult, error) {
	gvk := crd.GVK()
	log := m.Log.With().Str("kind", gvk.Kind).Bool("dryRun", m.DryRun).Logger()

	// Fail early with a message naming the CRD, not on the first List.
	if _, err := kubeutil.CheckCRDExists(m.Kube.RestConfig(), &gvk); err != nil {
		return nil, err
	}

	switch {
	case crd.HasMigrate():
		return kind.Run(ctx, &m.Runner, crd, gvk, log)
	case crd.HasServeFieldMigrate():
		return field.Run(ctx, &m.Runner, crd, gvk, log)
	}
	return nil, fmt.Errorf("no migration configured for: %s", gvk)
}
