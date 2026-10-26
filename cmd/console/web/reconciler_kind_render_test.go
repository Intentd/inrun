package web

import (
	"bytes"
	"html/template"
	"io/fs"
	"strings"
	"testing"
)

// Every embedded template parses with the console's functions.
func TestTemplates_Parse(t *testing.T) {
	names, err := fs.Glob(Assets, TemplateDir+"/*.html")
	if err != nil || len(names) == 0 {
		t.Fatalf("no templates: %v", err)
	}
	for _, path := range names {
		name := path[strings.LastIndex(path, "/")+1:]
		if _, err := template.New(name).Funcs(templateFuncs).ParseFS(Assets, path); err != nil {
			t.Errorf("parse %s: %v", name, err)
		}
	}
}

func render(t *testing.T, name string, data interface{}) string {
	t.Helper()
	tmpl, err := template.New(name).Funcs(templateFuncs).ParseFS(Assets, TemplateDir+"/"+name)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return buf.String()
}

// The CRD page and its docs page name the reconciler kind /blueprint reports,
// the targets that differ, and the finalizers the runtime applies.
func TestCRDPages_ShowReconcilerKind(t *testing.T) {
	cases := map[string]string{
		"declarative": "Declarative reconciler",
		"hooks":       "Go hooks reconciler",
		"constructor": "Go constructor reconciler",
		"remote":      "Remote reconciler",
	}
	for kind, badge := range cases {
		t.Run(kind, func(t *testing.T) {
			crd := &CRDDetail{
				Name: "website",
				Mode: "dynamic",
				OperatorBox: map[string]interface{}{
					"type":        kind,
					"targets":     map[string]interface{}{"v2": "remote"},
					"finalizers":  map[string]interface{}{"source": "runtime", "values": []interface{}{"inrun.dev/cleanup"}},
					"hooks":       map[string]interface{}{"configured": false},
					"constructor": map[string]interface{}{"configured": false},
				},
			}
			page := render(t, "crd.html", map[string]interface{}{"CRD": crd, "BlueprintName": "demo"})
			if !strings.Contains(page, badge) {
				t.Errorf("crd.html: missing %q", badge)
			}
			if !strings.Contains(page, "inrun.dev/cleanup") {
				t.Error("crd.html: missing the runtime finalizer")
			}
			docs := render(t, "crd_docs.html", CRDDocsData{BlueprintName: "demo", CRD: crd})
			if !strings.Contains(docs, "the runtime owns the lifecycle") {
				t.Error("crd_docs.html: missing the lifecycle paragraph")
			}
			if !strings.Contains(docs, "<code>v2</code> (remote)") {
				t.Error("crd_docs.html: missing the target that differs")
			}
		})
	}
}
