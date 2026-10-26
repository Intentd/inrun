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

// TestIncludeFilesMatchSchemas validates every file whose schema line names
// an include schema against it, so the schema line is right and the schema
// accepts what the include loaders accept.
func TestIncludeFilesMatchSchemas(t *testing.T) {
	const root = "../.."
	schemaDir := filepath.Join(root, "website/static/schema/v1")
	c := jsonschema.NewCompiler()
	schemas := map[string]*jsonschema.Schema{}
	for file := range types.IncludeSchemas {
		f, err := os.Open(filepath.Join(schemaDir, file))
		if err != nil {
			t.Fatalf("%s: %v (run make generate-schema)", file, err)
		}
		doc, err := jsonschema.UnmarshalJSON(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if err := c.AddResource(types.SchemaBaseURL+file, doc); err != nil {
			t.Fatal(err)
		}
		if schemas[file], err = c.Compile(types.SchemaBaseURL + file); err != nil {
			t.Fatalf("compiling %s: %v", file, err)
		}
	}

	prefix := "# yaml-language-server: $schema=" + types.SchemaBaseURL
	checked := 0
	for _, dir := range []string{"examples", "pkg", "charts", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".yaml") {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			first, _, _ := strings.Cut(string(data), "\n")
			file, ok := strings.CutPrefix(strings.TrimSpace(first), prefix)
			if !ok || !strings.HasPrefix(file, "include/") {
				return nil
			}
			sch := schemas[file]
			if sch == nil {
				t.Errorf("%s: no include schema %s", p, file)
				return nil
			}
			var doc any
			if err := yaml.Unmarshal(data, &doc); err != nil {
				t.Errorf("%s: %v", p, err)
				return nil
			}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Errorf("%s: %v", p, err)
				return nil
			}
			v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				t.Errorf("%s: %v", p, err)
				return nil
			}
			if err := sch.Validate(v); err != nil {
				t.Errorf("%s (%s): %v", p, file, err)
			}
			checked++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked == 0 {
		t.Fatal("no include files with an include schema line found")
	}
	t.Logf("validated %d include files", checked)
}

// A kubectl apply, delete or patch entry may be a plain string: shorthand for
// file (and for patch, patchFile too):
// the loader decodes it, and the e2e schema accepts it. The long form is still
// read strictly.
func TestKubectlApplyDeleteShorthand(t *testing.T) {
	const doc = `
apply:
  - ./cr.yaml
  - inline: "kind: ConfigMap"
delete:
  - ./cr.yaml
  - kind: ConfigMap
    name: app
patch:
  - ./scale-up.yaml
  - kind: ConfigMap
    name: app
    patch: "{}"
`
	var k types.E2EKubectl
	if err := yaml.Unmarshal([]byte(doc), &k); err != nil {
		t.Fatal(err)
	}
	if k.Apply[0].File != "./cr.yaml" || k.Apply[1].Inline == "" {
		t.Errorf("apply = %+v", k.Apply)
	}
	if k.Delete[0].File != "./cr.yaml" || k.Delete[1].Name != "app" {
		t.Errorf("delete = %+v", k.Delete)
	}
	if p := k.Patch[0]; p.File != "./scale-up.yaml" || p.PatchFile != "./scale-up.yaml" || k.Patch[1].Patch != "{}" {
		t.Errorf("patch = %+v", k.Patch)
	}
	var bad types.E2EKubectl
	if err := yaml.Unmarshal([]byte("apply:\n  - fil: ./cr.yaml\n"), &bad); err == nil {
		t.Error("an unknown key in the long form was accepted")
	}

	f, err := os.Open("../../website/static/schema/v1/include/e2e-expect.json")
	if err != nil {
		t.Fatal(err)
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("e2e-expect.json", schemaDoc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("e2e-expect.json")
	if err != nil {
		t.Fatal(err)
	}
	v, _ := jsonschema.UnmarshalJSON(strings.NewReader(`{"expect": [{"name": "x", "kubectl": {
		"apply": ["./cr.yaml", {"inline": "kind: ConfigMap"}],
		"delete": ["./cr.yaml", {"kind": "ConfigMap", "name": "app"}],
		"patch": ["./scale-up.yaml", {"kind": "ConfigMap", "name": "app", "patch": "{}"}]}}]}`))
	if err := sch.Validate(v); err != nil {
		t.Errorf("schema rejected the shorthand: %v", err)
	}
}
