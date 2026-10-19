// Package pipeline builds a ready Blueprint from merged sources: parse, enrich,
// validate and wire it for the runtime. NewBlueprint is the runtime path and
// exits on error; BuildExpanded is the CLI path and returns the error.
package pipeline

import (
	"fmt"

	"github.com/intentd/inrun/pkg/util/term"

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
