package types_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/intentd/inrun/pkg/types"
)

// TestExamplesMatchSchemas validates every Inrun document under examples/
// against the editor schemas in website/static/schema/v1, which
// hack/generate-schema reflects from these types. A failure means a schema
// rejects a file the loader accepts: fix the shorthands in the generator, or
// the example.
func TestExamplesMatchSchemas(t *testing.T) {
	const root = "../.."
	schemaDir := filepath.Join(root, "website/static/schema/v1")
	c := jsonschema.NewCompiler()
	schemas := map[string]*jsonschema.Schema{}
	for kind, file := range types.SchemaFiles {
		f, err := os.Open(filepath.Join(schemaDir, file))
		if err != nil {
			t.Fatalf("%s: %v (run make generate-schema)", file, err)
		}
		doc, err := jsonschema.UnmarshalJSON(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if err := c.AddResource(file, doc); err != nil {
			t.Fatal(err)
		}
		if schemas[kind], err = c.Compile(file); err != nil {
			t.Fatalf("compiling %s: %v", file, err)
		}
	}

	// The schemas reject what the loader rejects: an unknown field, and a
	// kind that doesn't match the file's schema.
	for _, bad := range []string{
		`{"apiVersion": "inrun.dev/v1", "kind": "Blueprint", "spec": {"crdz": {}}}`,
		`{"apiVersion": "inrun.dev/v1", "kind": "Stack", "spec": {}}`,
	} {
		v, _ := jsonschema.UnmarshalJSON(strings.NewReader(bad))
		if schemas["Blueprint"].Validate(v) == nil {
			t.Errorf("blueprint schema accepted %s", bad)
		}
	}

	// Every shorthand form the loader accepts, including ones no example
	// uses yet. Each must also load (see TestShorthandsLoad).
	for _, good := range shorthandDocs {
		v, _ := jsonschema.UnmarshalJSON(strings.NewReader(good.json))
		if err := schemas[good.kind].Validate(v); err != nil {
			t.Errorf("%s schema rejected %s: %v", good.kind, good.json, err)
		}
	}

	checked := 0
	err := filepath.WalkDir(filepath.Join(root, "examples"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".yaml") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dec := yaml.NewDecoder(bytes.NewReader(data))
		for {
			var doc map[string]any
			if err := dec.Decode(&doc); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return nil // not YAML an Inrun loader reads
			}
			kind, _ := doc["kind"].(string)
			sch := schemas[kind]
			if sch == nil || doc["apiVersion"] != "inrun.dev/v1" {
				continue
			}
			// Round-trip through JSON so values have the JSON types a
			// validator expects.
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Errorf("%s: %v", p, err)
				continue
			}
			v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				t.Errorf("%s: %v", p, err)
				continue
			}
			if err := sch.Validate(v); err != nil {
				t.Errorf("%s (%s): %v", p, kind, err)
			}
			checked++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no Inrun documents found under examples/")
	}
	t.Logf("validated %d documents", checked)
}

// shorthandDocs use the shorthand forms of the multi-shape types.
var shorthandDocs = []struct{ kind, json string }{
	{"Blueprint", `{"apiVersion": "inrun.dev/v1", "kind": "Blueprint", "spec": {"crds": {"app": {
		"enrich": ["pods", {"events": {"when": [{"field": "spec.debug", "equals": "true"}]}}],
		"dependsOn": ["database"]}}}}`},
	{"Blueprint", `{"apiVersion": "inrun.dev/v1", "kind": "Blueprint", "spec": {"crds": {"app": {
		"dependsOn": {"database": "healthy", "cache": {"condition": "started"}}}}}}`},
}

// TestShorthandsLoad checks the shorthand documents load, so the schema test
// above is testing forms the loader really accepts.
func TestShorthandsLoad(t *testing.T) {
	for _, d := range shorthandDocs {
		var doc map[string]any
		if err := json.Unmarshal([]byte(d.json), &doc); err != nil {
			t.Fatal(err)
		}
		crds := doc["spec"].(map[string]any)["crds"].(map[string]any)
		raw, _ := yaml.Marshal(crds["app"])
		var entry types.CRDEntry
		if err := yaml.Unmarshal(raw, &entry); err != nil {
			t.Errorf("loader rejected %s: %v", raw, err)
		}
	}
}
