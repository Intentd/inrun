// Command generate-schema writes a JSON Schema for each Inrun kind (Blueprint,
// Stack, Simulate, E2E, Module) into website/static/schema/v1/, served at
// https://inrun.dev/schema/v1/<kind>.json for code editors, and one for each
// include file (types.IncludeSchemas) under include/.
//
// The schemas are reflected by github.com/intentd/tools/schemagen from the
// same Go types the loader decodes into, with descriptions from their
// comments. Types that also accept a shorthand (a string for an object, a
// list for a map) get it added as an alternative in shorthands below.
//
// Run from the repository root: go run ./hack/generate-schema
package main

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/invopop/jsonschema"

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
	{Module: "github.com/intentd/kubewright", Dirs: []string{"core/types", "presets"}},
	{Module: "github.com/intentd/tools", Dirs: []string{"condition", "duration"}},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate-schema:", err)
		os.Exit(1)
	}
}

func run() error {
	g := &schemagen.Generator{
		BaseURL:    types.SchemaBaseURL,
		OutDir:     "website/static/schema/v1",
		Local:      module + "/pkg/types",
		Prefixes:   map[string]string{"github.com/intentd/kubewright/core/types": "kubewright"},
		Shorthands: shorthands,
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

var one uint64 = 1

// shorthands are the types whose UnmarshalYAML accepts more than the
// reflected struct. Keep in step with those decoders in pkg/types.
var shorthands = map[string]schemagen.Shorthand{
	"SetupApplyEntry":    schemagen.Either(schemagen.Str("Path to a manifest")),
	"E2EKubectlApply":    schemagen.Either(schemagen.Str("Path to a manifest to apply (shorthand for file)")),
	"E2EKubectlDelete":   schemagen.Either(schemagen.Str("Path to a manifest whose resources to delete (shorthand for file)")),
	"E2EKubectlPatch":    schemagen.Either(schemagen.Str("A file that is both the target and the merge patch: one partial object (shorthand for file and patchFile)")),
	"E2EImport":          schemagen.Either(schemagen.Str("Path to another E2E file")),
	"FileSource":         schemagen.Either(schemagen.Str("Path or URL of the file")),
	"RegistrySource":     schemagen.Either(schemagen.Str("Pattern reference, e.g. oci://registry/path:tag")),
	"RetryBackoffConfig": schemagen.Either(schemagen.Str("Initial delay, e.g. 500ms")),
	"DependsOnCondition": schemagen.Either(schemagen.Str("started or healthy")),
	"IdempotencyKeyConfig": schemagen.Either(&jsonschema.Schema{
		Type: "boolean", Const: false, Description: "false sends no idempotency key",
	}),
	"SetupConfig": schemagen.Either(&jsonschema.Schema{
		Type: "array", Items: schemagen.Str("Path to a manifest"), Description: "Shorthand for apply",
	}),
	// EnrichTarget is "pods", or {events: {when: [...], or: [...]}}: one key,
	// the enrichment's name.
	"EnrichTarget": func(d *jsonschema.Schema) *jsonschema.Schema {
		gate := jsonschema.NewProperties()
		gate.Set("when", &jsonschema.Schema{Type: "array", Items: schemagen.Ref("Condition"), Description: "Enrich when every condition passes"})
		gate.Set("or", &jsonschema.Schema{Type: "array", Items: schemagen.Ref("Condition"), Description: "Enrich when at least one condition passes"})
		return &jsonschema.Schema{
			Description: d.Description,
			AnyOf: []*jsonschema.Schema{
				schemagen.Str("An enrichment to turn on, always: pods, events, replicasets, ..."),
				{
					Type: "object", MinProperties: &one, MaxProperties: &one,
					AdditionalProperties: &jsonschema.Schema{
						Type: "object", Properties: gate, AdditionalProperties: jsonschema.FalseSchema,
					},
				},
			},
		}
	},
	"duration.Duration": func(d *jsonschema.Schema) *jsonschema.Schema {
		return schemagen.Str("A duration: Go units (15s, 2m, 1h) or d, w, mo, y")
	},
	"DependsOnMap": func(d *jsonschema.Schema) *jsonschema.Schema {
		return &jsonschema.Schema{
			Description: d.Description,
			AnyOf: []*jsonschema.Schema{
				{Type: "array", Items: schemagen.Str("A CRD name; waits until it has started")},
				{Type: "object", AdditionalProperties: schemagen.Ref("DependsOnCondition")},
			},
		}
	},
	"ServeTargetValue": func(d *jsonschema.Schema) *jsonschema.Schema {
		return &jsonschema.Schema{
			Description: d.Description,
			AnyOf: []*jsonschema.Schema{
				schemagen.Str("The one target's name"),
				{Type: "object", AdditionalProperties: schemagen.Ref("ServeTargetConfig")},
			},
		}
	},
	"GatewayClustersConfig": func(d *jsonschema.Schema) *jsonschema.Schema {
		props := jsonschema.NewProperties()
		props.Set("include", schemagen.Str("Path to a file whose clusters: key holds more entries"))
		return &jsonschema.Schema{
			Description:          d.Description,
			Type:                 "object",
			Properties:           props,
			AdditionalProperties: schemagen.Ref("GatewayClusterConfig"),
		}
	},
}
