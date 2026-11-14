package migrate

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/inrunmeta"
	"github.com/intentd/kome"
	"github.com/intentd/kome/declaration"
	"github.com/intentd/kome/engine"
	"github.com/intentd/kome/field"
	"github.com/intentd/kome/kind"
	client "github.com/intentd/tools/kubeclient"
	"github.com/intentd/tools/logger"
	"github.com/intentd/tools/resolver"
)

// MigrateMode is the operation a run performs.
type MigrateMode int

const (
	// ModeForward migrates CRs to the declared shape (KOME's up).
	ModeForward MigrateMode = iota
	// ModeRollback restores CRs from their snapshots (KOME's down).
	ModeRollback
	// ModeClean removes migration annotations without touching spec.
	ModeClean
)

// String returns the CLI and JSON name of the mode.
func (m MigrateMode) String() string {
	switch m {
	case ModeForward:
		return "forward"
	case ModeRollback:
		return "rollback"
	case ModeClean:
		return "clean"
	default:
		return fmt.Sprintf("MigrateMode(%d)", int(m))
	}
}

// CRMigrateResult is the outcome for one CR.
type CRMigrateResult = engine.ObjectResult

// Selection is the scope of one run: the CRD's migrate.selection merged with
// the CLI flags.
type Selection struct {
	Namespaces []string
	Name       string
	Selector   string
}

// IsEmpty reports whether the selection matches everything.
func (s Selection) IsEmpty() bool {
	return len(s.Namespaces) == 0 && s.Name == "" && s.Selector == ""
}

// Equal reports whether two selections are the same.
func (s Selection) Equal(o Selection) bool {
	return s.Name == o.Name && s.Selector == o.Selector && slices.Equal(s.Namespaces, o.Namespaces)
}

// KindMigrateResult is the outcome for all CRs of one kind.
type KindMigrateResult struct {
	Kind string
	// To is the destination kind of a kind-level migration; empty otherwise.
	To        string
	Selection Selection
	CRs       []CRMigrateResult
}

// Migrator runs a blueprint's migrations through KOME, with Inrun's
// annotations, field manager and template functions.
type Migrator struct {
	Kube      client.Interface
	Functions types.KELFunctionRegistry
	Mode      MigrateMode
	Opts      MigratorOpts
	// Kind, when set, limits the run to the CRD with this kind.
	Kind string
	// Progress, when set, is called after each CR with the kind's label
	// ("Old" or "Old → New") and the count so far.
	Progress func(label string, done, total int)
	Log      *zerolog.Logger
}

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
	return &Migrator{Kube: kube, Functions: funcs, Mode: mode, Opts: opts, Kind: opts.Kind, Progress: opts.Progress, Log: log}
}

// Run runs every migration bp declares, or only Kind's when set. Results are
// in blueprint order; a kind-level error is returned with the results so far.
func (m *Migrator) Run(ctx context.Context, bp *blueprint.Blueprint) ([]*KindMigrateResult, error) {
	var ms kome.Migrations
	labels := map[string]string{}
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
		mig, label := migrationFor(name, &entry)
		ms = append(ms, mig)
		labels[mig.Name] = label
	}
	if m.Kind != "" && !matched {
		return nil, fmt.Errorf("no CRD of kind %q declares a migration", m.Kind)
	}

	e := Engine(m.Kube, m.Functions)
	o := kome.Options{
		DryRun: m.Opts.DryRun, Strict: m.Opts.Strict, Force: m.Opts.Force, PageSize: m.Opts.PageSize,
		Scope: engine.Selection{Namespaces: m.Opts.Namespaces, Name: m.Opts.Name, Selector: m.Opts.Selector},
		// The gateway migrates under the CRD it serves;
		// the old paths don't need to be in it.
		NoLiveCheck: true,
	}
	if m.Progress != nil {
		o.Progress = func(name string, done, total int) { m.Progress(labels[name], done, total) }
	}

	var results []*engine.Result
	var err error
	switch m.Mode {
	case ModeRollback:
		results, err = e.Down(ctx, ms, o)
	case ModeClean:
		results, err = e.Clean(ctx, ms, o)
	default:
		results, err = e.Up(ctx, ms, o)
	}
	out := make([]*KindMigrateResult, 0, len(results))
	for _, r := range results {
		for _, w := range r.Warnings {
			m.Log.Warn().Str("kind", r.Kind).Msg(w)
		}
		for _, cr := range r.Objects {
			if cr.Err != nil && !m.Opts.Strict {
				m.Log.Warn().Str("cr", cr.Ref()).Err(cr.Err).Msg("migration skipped")
			}
		}
		out = append(out, &KindMigrateResult{
			Kind: r.Kind, To: r.To, CRs: r.Objects,
			Selection: Selection{Namespaces: r.Selection.Namespaces, Name: r.Selection.Name, Selector: r.Selection.Selector},
		})
	}
	return out, err
}

// Migrations is a KOME declaration's migrations, ready for KOME's engine.
func Migrations(f *declaration.File) (kome.Migrations, error) {
	return kome.FromFile(f)
}

// Engine is KOME's engine as Inrun runs it: the inrun.dev annotations, the
// gateway's field manager, and templates with the Blueprint's functions.
func Engine(kube client.Interface, funcs types.KELFunctionRegistry) *kome.Engine {
	return &kome.Engine{
		Kube:         kube,
		Prefix:       inrunmeta.ReservedDomain,
		FieldManager: config.FieldManagerGateway,
		Resolver: func(data map[string]any) *resolver.Resolver {
			return template.NewResolverFromMap(data).WithUserFunctions(funcs).Core()
		},
	}
}

// migrationFor is the KOME migration a CRD declares, and its progress label.
// Validation allows at most one level per CRD.
// name is the CRD's name in the Blueprint (its spec.crds key).
func migrationFor(name string, crd *types.CRDEntry) (kome.Migration, string) {
	gvk := crd.GVK()
	sel := engine.Selection{}
	if crd.Migrate != nil {
		s := crd.Migrate.Selection
		sel = engine.Selection{Namespaces: s.Namespaces, Name: s.Name, Selector: s.Selector, When: s.When.Unwrap(), Or: s.Or.Unwrap()}
	}
	if crd.HasMigrate() {
		to := crd.Migrate.GVK()
		km := kind.Migration{
			Name: name, From: gvk, To: to,
			Namespace:      crd.Migrate.To.Namespace,
			Labels:         crd.Migrate.To.Metadata.Labels,
			Annotations:    crd.Migrate.To.Metadata.Annotations,
			PreserveSource: crd.ShouldPreserveMeta(),
			CopyStatus:     crd.Migrate.To.CopyStatus,
			Prune:          crd.Migrate.Prune,
			Selection:      sel,
		}
		return kome.Migration{Name: name, Level: declaration.LevelKind, Kind: km}, gvk.Kind + " → " + destinationLabel(gvk, to)
	}
	fm := field.Migration{Name: name, GVK: gvk, Selection: sel}
	for _, key := range crd.Serve.FieldNames() {
		cfg := crd.Serve.Fields[key]
		if !cfg.HasMigrate() {
			continue
		}
		fm.Moves = append(fm.Moves, field.Move{
			Key:   key,
			From:  strings.TrimPrefix(cfg.MigrateFrom(), "spec."),
			Field: cfg.Field(),
		})
	}
	return kome.Migration{Name: name, Level: declaration.LevelField, Field: fm}, gvk.Kind
}

func destinationLabel(from, to schema.GroupVersionKind) string {
	if to.Kind == from.Kind {
		return to.Kind + "." + to.Group
	}
	return to.Kind
}
