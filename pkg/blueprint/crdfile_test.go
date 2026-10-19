package blueprint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"gopkg.in/yaml.v3"
)

// TestResolveCRDFiles_PreservesTypedModeFields verifies that ResolveCRDFiles
// merges structural fields from the CRD YAML into the existing apiTypes block
// rather than replacing it — so location, object, objectList, and alias survive.
func TestResolveCRDFiles_PreservesTypedModeFields(t *testing.T) {
	dir := t.TempDir()

	crdYAML := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.io
spec:
  group: example.io
  names:
    kind: Widget
    plural: widgets
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
`
	crdPath := filepath.Join(dir, "crd.yaml")
	if err := os.WriteFile(crdPath, []byte(crdYAML), 0644); err != nil {
		t.Fatal(err)
	}

	blueprintYAML := `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: test
spec:
  crds:
    widget:
      crdFile: ./crd.yaml
      apiTypes:
        group: example.io
        version: v1alpha1
        kind: Widget
        plural: widgets
        object: Widget
        objectList: WidgetList
        alias: widgetv1
        location: github.com/example/widget/api/v1alpha1
      operatorBox:
        reconciler:
          default: true
`
	blueprintPath := filepath.Join(dir, "blueprint.yaml")
	if err := os.WriteFile(blueprintPath, []byte(blueprintYAML), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := blueprint.ResolveCRDFiles(blueprintPath)
	if err != nil {
		t.Fatalf("ResolveCRDFiles: %v", err)
	}

	var out map[string]interface{}
	if err := yaml.Unmarshal(resolved, &out); err != nil {
		t.Fatalf("unmarshal resolved: %v", err)
	}

	spec := out["spec"].(map[string]interface{})
	crds := spec["crds"].(map[string]interface{})
	widget := crds["widget"].(map[string]interface{})
	apiTypes, ok := widget["apiTypes"].(map[string]interface{})
	if !ok {
		t.Fatal("apiTypes missing from resolved blueprint")
	}

	// crdFile should be removed
	if _, has := widget["crdFile"]; has {
		t.Error("crdFile was not removed after resolution")
	}

	// Structural fields come from the CRD YAML
	for field, want := range map[string]string{
		"group":   "example.io",
		"version": "v1alpha1",
		"kind":    "Widget",
		"plural":  "widgets",
	} {
		if got, _ := apiTypes[field].(string); got != want {
			t.Errorf("apiTypes.%s = %q, want %q", field, got, want)
		}
	}

	// Typed-mode fields must be preserved from the original inline declaration
	for field, want := range map[string]string{
		"location":   "github.com/example/widget/api/v1alpha1",
		"object":     "Widget",
		"objectList": "WidgetList",
		"alias":      "widgetv1",
	} {
		if got, _ := apiTypes[field].(string); got != want {
			t.Errorf("apiTypes.%s = %q, want %q (typed-mode field was dropped)", field, got, want)
		}
	}
}

// TestResolveCRDFiles_NoCRDFile passes through a blueprint with no crdFile unchanged.
func TestResolveCRDFiles_NoCRDFile(t *testing.T) {
	dir := t.TempDir()

	blueprintYAML := `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: test
spec:
  crds:
    widget:
      apiTypes:
        group: example.io
        version: v1alpha1
        kind: Widget
        plural: widgets
        location: github.com/example/widget/api/v1alpha1
      operatorBox:
        reconciler:
          default: true
`
	blueprintPath := filepath.Join(dir, "blueprint.yaml")
	if err := os.WriteFile(blueprintPath, []byte(blueprintYAML), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := blueprint.ResolveCRDFiles(blueprintPath)
	if err != nil {
		t.Fatalf("ResolveCRDFiles: %v", err)
	}

	var out map[string]interface{}
	if err := yaml.Unmarshal(resolved, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	spec := out["spec"].(map[string]interface{})
	crds := spec["crds"].(map[string]interface{})
	widget := crds["widget"].(map[string]interface{})
	apiTypes := widget["apiTypes"].(map[string]interface{})

	if got, _ := apiTypes["location"].(string); got != "github.com/example/widget/api/v1alpha1" {
		t.Errorf("apiTypes.location = %q, want preserved", got)
	}
}
