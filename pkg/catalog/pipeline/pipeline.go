// Package pipeline builds a ready Catalog from merged sources: parse, enrich,
// validate and wire it for the runtime. NewCatalog is the runtime path and
// exits on error; BuildExpanded is the CLI path and returns the error.
package pipeline

import (
	"fmt"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/validate"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/typeregistry"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

// NewCatalog returns a fully built and validated Catalog.
// Exits the process on any error
func NewCatalog(kfg *config.Config, m *merger.Merger) *catalog.Catalog {
	k := &catalog.Catalog{}
	k.SetConfig(kfg)

	paths := kfg.Catalog().Paths()

	typeregistry.RegisterRuntimeObjects()

	entries, err := k.BuildRuntimeCatalog(kfg, m, paths...)
	if err != nil {
		utils.Exit(err)
	}

	// The gateway never runs a reconciler, so it needs none of the project's
	// compiled types: every CR is unstructured there, and the stock gateway
	// image serves typed Catalogs too.
	gateway := kfg.IsGatewayInstance()
	if gateway {
		k.UseUnstructured()
	}

	if len(entries) == 0 && !k.IsStandaloneGateway() {
		utils.Exit(fmt.Errorf("validation error: catalog empty"))
	}

	for _, crd := range entries {
		if !gateway && len(types.ObjectRegistry) == 0 && !crd.IsDynamic() {
			utils.Exit(fmt.Errorf(
				"ObjectRegistry is empty — run 'inrun generate registry --file <my-catalog.yaml>' first",
			))
		}
	}

	if err := validate.Execute(k, kfg); err != nil {
		utils.Exit(err)
	}

	if err := k.CheckDeprecationPolicy(); err != nil {
		utils.Exit(err)
	}

	k, err = k.UpdateResourceMapAndReturn()
	if err != nil {
		utils.Exit(err)
	}

	return k
}

// BuildExpanded is the canonical pipeline for CLI commands that need a fully
// ready Catalog: merge → expand modules → validate.
func BuildExpanded(kfg *config.Config, m *merger.Merger) (*catalog.Catalog, error) {
	k := &catalog.Catalog{}
	if _, err := k.BuildRuntimeCatalog(kfg, m); err != nil {
		return nil, err
	}
	if err := validate.Execute(k, kfg); err != nil {
		return nil, fmt.Errorf("catalog validate: %w", err)
	}
	return k, nil
}
