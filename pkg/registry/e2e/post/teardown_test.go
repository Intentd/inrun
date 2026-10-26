package post

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsCustomGroup(t *testing.T) {
	cases := map[string]bool{
		"v1":                           false,
		"apps/v1":                      false,
		"rbac.authorization.k8s.io/v1": false,
		"apiextensions.k8s.io/v1":      false,
		"demo.inrun.dev/v1alpha1":      true,
		"cert-manager.io/v1":           true,
	}
	for apiVersion, want := range cases {
		if got := isCustomGroup(apiVersion); got != want {
			t.Errorf("isCustomGroup(%q) = %v, want %v", apiVersion, got, want)
		}
	}
}

func TestManifestDocs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.yaml")
	manifest := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: websites.demo.inrun.dev
---
# comment only
---
apiVersion: demo.inrun.dev/v1alpha1
kind: Website
metadata:
  name: hello
`
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	docs, err := manifestDocs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("got %d docs, want 2", len(docs))
	}
	if docs[0].kind != "CustomResourceDefinition" || docs[0].name != "websites.demo.inrun.dev" {
		t.Errorf("doc 0 = %s %s", docs[0].kind, docs[0].name)
	}
	if docs[1].apiVersion != "demo.inrun.dev/v1alpha1" || docs[1].name != "hello" {
		t.Errorf("doc 1 = %s %s", docs[1].apiVersion, docs[1].name)
	}
}
