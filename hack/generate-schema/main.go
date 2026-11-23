// Command generate-schema writes a JSON Schema for each Inrun kind (Blueprint,
// Stack, Simulate, E2E, Module) into website/static/schema/v1/, served at
// https://inrun.dev/schema/v1/<kind>.json for code editors, and one for each
// include file (types.IncludeSchemas) under include/.
//
// The schemas are reflected by github.com/intentd/tools/schemagen from the
// same Go types the loader decodes into, with descriptions from their
// comments. A type that also accepts a shorthand (a string for an object, a
// list for a map) declares it beside its UnmarshalYAML, in JSONSchemaExtend.
//
// Run from the repository root: go run ./hack/generate-schema
package main

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/schemagen"
)

const module = "github.com/intentd/inrun"

// kinds is one schema per kind, written to types.SchemaFiles[kind]. Blueprint
// and Stack decode into the same type.
var kinds = []struct {
	kind string
	v    any
}{
	{"Blueprint", types.BlueprintFile{}},
	{"Stack", types.BlueprintFile{}},
	{"Simulate", types.Simulate{}},
	{"E2E", types.E2E{}},
	{"Module", types.Module{}},
}

// comments are the types whose Go comments become the descriptions.
var comments = []schemagen.Source{
	{Module: module, Dirs: []string{"pkg/types", "pkg/blueprint", "domain"}},
	{Module: "github.com/intentd/jig", Dirs: []string{"pkg/spec"}},
	{Module: "github.com/intentd/kubewright", Dirs: []string{"core/types", "presets"}},
	{Module: "github.com/intentd/tools", Dirs: []string{"condition", "duration", "assert", "resolver/env", "cluster"}},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate-schema:", err)
		os.Exit(1)
	}
}

func run() error {
	g := &schemagen.Generator{
		BaseURL:  types.SchemaBaseURL,
		OutDir:   "website/static/schema/v1",
		Local:    module + "/pkg/types",
		Prefixes: map[string]string{"github.com/intentd/kubewright/core/types": "kubewright"},
	}
	if err := g.LoadComments(comments...); err != nil {
		return err
	}
	for _, k := range kinds {
		if err := write(g, types.SchemaFiles[k.kind], "Inrun "+k.kind, k.v,
			schemagen.Enum("apiVersion", "inrun.dev/v1"),
			schemagen.Const("kind", k.kind),
			schemagen.Required("apiVersion", "kind"),
		); err != nil {
			return err
		}
	}
	// An include file has no apiVersion or kind: its schema is the type alone.
	for _, file := range sortedKeys(types.IncludeSchemas) {
		name := strings.TrimSuffix(path.Base(file), ".json")
		if err := write(g, file, "Inrun include: "+name, types.IncludeSchemas[file]); err != nil {
			return err
		}
	}
	return nil
}

func write(g *schemagen.Generator, file, title string, v any, pins ...schemagen.Pin) error {
	dst, err := g.Write(file, title, v, pins...)
	if err != nil {
		return err
	}
	fmt.Printf("generated %s\n", dst)
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
