// Package pipeline builds a ready Blueprint from merged sources: parse, enrich,
// validate and wire it for the runtime. NewBlueprint is the runtime path and
// exits on error; BuildExpanded is the CLI path and returns the error.
package pipeline

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/util/term"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/validate"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/typeregistry"
	"github.com/intentd/inrun/pkg/types"
)

// NewBlueprint marks this process as instance (runtime or gateway) and returns
// a fully built and validated Blueprint. Exits the process on any error.
func NewBlueprint(cfg *config.Config, m *merger.Merger, instance config.Instance) *blueprint.Blueprint {
	cfg.SetInstance(instance)
	bp := &blueprint.Blueprint{}
	bp.SetConfig(cfg)

	paths := cfg.Blueprint().Paths()

	typeregistry.RegisterRuntimeObjects()

	entries, err := bp.BuildRuntimeBlueprint(cfg, m, paths...)
	if err != nil {
		term.Exit(err)
	}

	// The gateway never runs a reconciler, so it needs none of the project's
	// compiled types: every CR is unstructured there, and the stock gateway
	// image serves typed Blueprints too.
	gateway := cfg.IsGatewayInstance()
	if gateway {
		bp.UseUnstructured()
	}

	if len(entries) == 0 && !bp.IsStandaloneGateway() {
		term.Exit(fmt.Errorf("validation error: blueprint empty"))
	}

	for _, crd := range entries {
		if !gateway && len(types.ObjectRegistry) == 0 && !crd.IsDynamic() {
			term.Exit(fmt.Errorf(
				"ObjectRegistry is empty — run 'inrun generate registry --file <my-blueprint.yaml>' first",
			))
		}
	}

	if err := validate.Execute(bp, cfg); err != nil {
		term.Exit(err)
	}

	if err := bp.CheckDeprecationPolicy(); err != nil {
		term.Exit(err)
	}

	return bp
}

// BuildExpanded is the canonical pipeline for CLI commands that need a fully
// ready Blueprint: merge → expand modules → validate.
func BuildExpanded(cfg *config.Config, m *merger.Merger) (*blueprint.Blueprint, error) {
	bp := &blueprint.Blueprint{}
	if _, err := bp.BuildRuntimeBlueprint(cfg, m); err != nil {
		return nil, err
	}
	if err := validate.Execute(bp, cfg); err != nil {
		return nil, fmt.Errorf("blueprint validate: %w", err)
	}
	return bp, nil
}

// LoadBlueprint merges the given paths and builds the scheme registry,
// resolving the blueprint without connecting to a cluster.
func LoadBlueprint(cfg *config.Config, paths []string) (*blueprint.Blueprint, *runtime.Scheme, error) {
	m := merger.New(paths...)
	if err := m.Merge(); err != nil {
		return nil, nil, fmt.Errorf("merging blueprints: %w", err)
	}
	bp, err := BuildExpanded(cfg, m)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing blueprint: %w", err)
	}
	scheme, err := blueprint.NewSchemeRegistry(bp)
	if err != nil {
		return nil, nil, fmt.Errorf("building scheme: %w", err)
	}
	return bp, scheme, nil
}

// Bootstrap merges the given blueprint paths, builds a scheme registry, and
// starts a live kube client. Returns the resolved blueprint and a started
// client, ready for a command to run against.
func Bootstrap(ctx context.Context, cfg *config.Config, paths []string) (*blueprint.Blueprint, kubeclient.Interface, error) {
	bp, scheme, err := LoadBlueprint(cfg, paths)
	if err != nil {
		return nil, nil, err
	}

	kube, err := kubeclient.Connect(ctx, cfg, scheme)
	if err != nil {
		return nil, nil, err
	}
	return bp, kube, nil
}
