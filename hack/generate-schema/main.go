// Command generate-schema writes a JSON Schema for each Inrun kind (Blueprint,
// Stack, Simulate, E2E, Module) into website/static/schema/v1/, served at
// https://inrun.dev/schema/v1/<kind>.json for code editors, and one for each
// include file (types.IncludeSchemas) under include/.
//
// The schemas are reflected from the same Go types the loader decodes into,
// with descriptions from their comments. Types that also accept a shorthand
// (a string for an object, a list for a map) get it added as an alternative
// in shorthands below, and loosen widens the scalars the way the YAML decoder
// does, so the schema accepts exactly what the loader does.
//
// Run from the repository root: go run ./hack/generate-schema
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/invopop/jsonschema"

	"github.com/intentd/inrun/pkg/types"
)

const (
	module = "github.com/intentd/inrun"
	outDir = "website/static/schema/v1"
)

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

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate-schema:", err)
		os.Exit(1)
	}
}

func run() error {
	comments, err := goComments()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for _, k := range kinds {
		file := types.SchemaFiles[k.kind]
		if err := write(comments, file, "Inrun "+k.kind, k.v, func(s *jsonschema.Schema) error {
			return pinKind(s, k.kind)
		}); err != nil {
			return err
		}
	}
	// An include file has no apiVersion or kind: its schema is the type alone.
	for _, file := range sortedKeys(types.IncludeSchemas) {
		name := strings.TrimSuffix(path.Base(file), ".json")
		if err := write(comments, file, "Inrun include: "+name, types.IncludeSchemas[file], nil); err != nil {
			return err
		}
	}
	return nil
}

// write reflects v into outDir/file, runs pin on the root when given, then
// adds the shorthands and loosens the scalars.
func write(comments map[string]string, file, title string, v any, pin func(*jsonschema.Schema) error) error {
	r := &jsonschema.Reflector{
		FieldNameTag: "yaml",
		// The yaml tags mark few fields omitempty, so "required" is not
		// inferred from them; apiVersion and kind are required by pinKind.
		RequiredFromJSONSchemaTags: true,
		Namer:                      namer(),
		CommentMap:                 comments,
	}
	s := r.Reflect(v)
	s.ID = jsonschema.ID(types.SchemaBaseURL + file)
	s.Title = title
	tidy(s)
	if pin != nil {
		if err := pin(s); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	addShorthands(s)
	out, err := loosen(s)
	if err != nil {
		return err
	}
	dst := path.Join(outDir, file)
	if err := os.MkdirAll(path.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, append(out, '\n'), 0o644); err != nil {
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

// pinKind pins apiVersion and kind on the root and makes both required.
func pinKind(s *jsonschema.Schema, kind string) error {
	root := s.Definitions[strings.TrimPrefix(s.Ref, "#/$defs/")]
	if root == nil {
		return fmt.Errorf("root definition %q not found", s.Ref)
	}
	api, ok := root.Properties.Get("apiVersion")
	if !ok {
		return fmt.Errorf("no apiVersion field")
	}
	api.Enum = []any{"inrun.dev/v1"}
	k, ok := root.Properties.Get("kind")
	if !ok {
		return fmt.Errorf("no kind field")
	}
	k.Const = kind
	root.Required = []string{"apiVersion", "kind"}
	return nil
}

// addShorthands adds each shorthand form to the types the schema reaches.
func addShorthands(s *jsonschema.Schema) {
	for name, alt := range shorthands {
		def := s.Definitions[name]
		if def == nil {
			continue // this schema doesn't reach the type
		}
		s.Definitions[name] = alt(def)
	}
}

// divider matches a section-divider comment such as "── OperatorBox ───",
// which reads as a heading in code but not as a field's description.
var divider = regexp.MustCompile(`^\s*─+.*─+\s*$`)

// tidy removes divider lines from every description.
func tidy(s *jsonschema.Schema) {
	clean := func(d string) string {
		var keep []string
		for _, line := range strings.Split(d, "\n") {
			if !divider.MatchString(line) {
				keep = append(keep, line)
			}
		}
		return strings.TrimSpace(strings.Join(keep, "\n"))
	}
	for _, def := range s.Definitions {
		def.Description = clean(def.Description)
		if def.Properties == nil {
			continue
		}
		for pair := def.Properties.Oldest(); pair != nil; pair = pair.Next() {
			pair.Value.Description = clean(pair.Value.Description)
		}
	}
}

var one uint64 = 1

// loosen accepts what the YAML decoder accepts beyond the Go types: any scalar
// for a string field (replicas: 1 decodes to "1"), and null for every field
// (an empty key decodes to the zero value). A string with an enum or const
// keeps its exact values.
func loosen(s *jsonschema.Schema) ([]byte, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case []any:
			for _, v := range n {
				walk(v)
			}
		case map[string]any:
			_, enum := n["enum"]
			_, cnst := n["const"]
			if n["type"] == "string" && !enum && !cnst {
				n["type"] = []any{"string", "number", "boolean"}
			}
			for _, v := range n {
				walk(v)
			}
			props, _ := n["properties"].(map[string]any)
			for name, p := range props {
				props[name] = nullable(p)
			}
		}
	}
	walk(doc)
	return json.MarshalIndent(doc, "", "  ")
}

// nullable lets a property schema also be null.
func nullable(p any) any {
	m, ok := p.(map[string]any)
	if !ok {
		return p
	}
	null := map[string]any{"type": "null"}
	switch t := m["type"].(type) {
	case string:
		m["type"] = []any{t, "null"}
		return m
	case []any:
		m["type"] = append(t, "null")
		return m
	}
	if ref, ok := m["$ref"]; ok {
		delete(m, "$ref")
		m["anyOf"] = []any{map[string]any{"$ref": ref}, null}
		return m
	}
	if alts, ok := m["anyOf"].([]any); ok {
		m["anyOf"] = append(alts, null)
	}
	return m
}

func ref(name string) *jsonschema.Schema { return &jsonschema.Schema{Ref: "#/$defs/" + name} }

func str(desc string) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "string", Description: desc}
}

// either keeps the reflected object form and adds a shorthand beside it.
func either(short *jsonschema.Schema) func(*jsonschema.Schema) *jsonschema.Schema {
	return func(full *jsonschema.Schema) *jsonschema.Schema {
		return &jsonschema.Schema{Description: full.Description, AnyOf: []*jsonschema.Schema{short, full}}
	}
}

// shorthands are the types whose UnmarshalYAML accepts more than the
// reflected struct. Keep in step with those decoders in pkg/types.
var shorthands = map[string]func(*jsonschema.Schema) *jsonschema.Schema{
	"SetupApplyEntry":    either(str("Path to a manifest")),
	"E2EKubectlApply":    either(str("Path to a manifest to apply (shorthand for file)")),
	"E2EKubectlDelete":   either(str("Path to a manifest whose resources to delete (shorthand for file)")),
	"E2EKubectlPatch":    either(str("A file that is both the target and the merge patch: one partial object (shorthand for file and patchFile)")),
	"E2EImport":          either(str("Path to another E2E file")),
	"FileSource":         either(str("Path or URL of the file")),
	"RegistrySource":     either(str("Pattern reference, e.g. oci://registry/path:tag")),
	"RetryBackoffConfig": either(str("Initial delay, e.g. 500ms")),
	"DependsOnCondition": either(str("started or healthy")),
	"IdempotencyKeyConfig": either(&jsonschema.Schema{
		Type: "boolean", Const: false, Description: "false sends no idempotency key",
	}),
	"SetupConfig": either(&jsonschema.Schema{
		Type: "array", Items: str("Path to a manifest"), Description: "Shorthand for apply",
	}),
	// EnrichTarget is "pods", or {events: {when: [...], or: [...]}}: one key,
	// the enrichment's name.
	"EnrichTarget": func(d *jsonschema.Schema) *jsonschema.Schema {
		gate := jsonschema.NewProperties()
		gate.Set("when", &jsonschema.Schema{Type: "array", Items: ref("Condition"), Description: "Enrich when every condition passes"})
		gate.Set("or", &jsonschema.Schema{Type: "array", Items: ref("Condition"), Description: "Enrich when at least one condition passes"})
		return &jsonschema.Schema{
			Description: d.Description,
			AnyOf: []*jsonschema.Schema{
				str("An enrichment to turn on, always: pods, events, replicasets, ..."),
				{
					Type: "object", MinProperties: &one, MaxProperties: &one,
					AdditionalProperties: &jsonschema.Schema{
						Type: "object", Properties: gate, AdditionalProperties: jsonschema.FalseSchema,
					},
				},
			},
		}
	},
	"Duration": func(d *jsonschema.Schema) *jsonschema.Schema {
		return str("A duration: Go units (15s, 2m, 1h) or d, w, mo, y")
	},
	"DependsOnMap": func(d *jsonschema.Schema) *jsonschema.Schema {
		return &jsonschema.Schema{
			Description: d.Description,
			AnyOf: []*jsonschema.Schema{
				{Type: "array", Items: str("A CRD name; waits until it has started")},
				{Type: "object", AdditionalProperties: ref("DependsOnCondition")},
			},
		}
	},
	"ServeTargetValue": func(d *jsonschema.Schema) *jsonschema.Schema {
		return &jsonschema.Schema{
			Description: d.Description,
			AnyOf: []*jsonschema.Schema{
				str("The one target's name"),
				{Type: "object", AdditionalProperties: ref("ServeTargetConfig")},
			},
		}
	},
	"GatewayClustersConfig": func(d *jsonschema.Schema) *jsonschema.Schema {
		props := jsonschema.NewProperties()
		props.Set("include", str("Path to a file whose clusters: key holds more entries"))
		return &jsonschema.Schema{
			Description:          d.Description,
			Type:                 "object",
			Properties:           props,
			AdditionalProperties: ref("GatewayClusterConfig"),
		}
	},
}

// namer names definitions by type, prefixing types from other packages with
// their package name, and fails on two types that would share a name.
func namer() func(reflect.Type) string {
	seen := map[string]string{}
	return func(t reflect.Type) string {
		name := t.Name()
		if p := t.PkgPath(); p != module+"/pkg/types" && p != "" {
			name = path.Base(p) + "." + name
			if strings.HasPrefix(p, "github.com/intentd/kubewright/core/types") {
				name = "kubewright." + t.Name()
			}
		}
		if prev, ok := seen[name]; ok && prev != t.PkgPath() {
			panic(fmt.Sprintf("definition name %q used by %s and %s", name, prev, t.PkgPath()))
		}
		seen[name] = t.PkgPath()
		return name
	}
}

// goComments reads the Go comments of Inrun's and kubewright's types, so each
// field's description in the editor is its comment in the code.
func goComments() (map[string]string, error) {
	r := &jsonschema.Reflector{}
	for _, dir := range []string{"pkg/types", "pkg/blueprint", "domain"} {
		if err := r.AddGoComments(module, dir); err != nil {
			return nil, err
		}
	}
	kw, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/intentd/kubewright").Output()
	if err != nil {
		return nil, fmt.Errorf("finding kubewright: %w", err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(strings.TrimSpace(string(kw))); err != nil {
		return nil, err
	}
	defer os.Chdir(wd)
	for _, dir := range []string{"core/types", "presets"} {
		if err := r.AddGoComments("github.com/intentd/kubewright", dir); err != nil {
			return nil, err
		}
	}
	return r.CommentMap, nil
}
