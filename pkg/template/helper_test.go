package template

import (
	"context"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestValidResolverName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"camelCase", "databaseReady", false},
		{"PascalCase", "DatabaseReady", false},
		{"with digits", "database2Ready", false},
		{"underscore", "database_ready", false},
		{"multiple underscores", "database__ready", false},

		{"empty", "", true},
		{"leading digit", "2database", true},
		{"only digits", "123", true},
		{"leading underscore", "_database", true},
		{"hyphen", "database-ready", true},
		{"space", "database ready", true},
		{"hash", "#database", true},
		{"dot", "database.ready", true},
		{"slash", "database/ready", true},
		{"emoji", "database🚀", true},
		{"unicode", "dátabase", true},
		{"newline", "database\nready", true},
		{"tab", "database\tready", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidResolverName(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewResolver_LeavesTheObjectUnchanged(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web"},
	}}
	r, err := NewResolver(context.Background(), obj)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Data()["inrun"]; !ok {
		t.Fatal("resolver data has no .inrun")
	}
	for _, key := range []string{"inputs", "inrun"} {
		if _, ok := obj.Object[key]; ok {
			t.Errorf("NewResolver added %q to the object it was given", key)
		}
	}
}

func TestResolveEnvVarsLenient(t *testing.T) {
	r := NewResolverFromMap(map[string]interface{}{"metadata": map[string]interface{}{"name": "app"}})
	got := r.ResolveEnvVarsLenient(types.EnvVarList{
		{Name: "{{ .metadata.name }}", Value: "{{ .metadata.name }}"},
		{Name: "PLAIN", Value: "x"},
		{Name: "BAD", Value: "{{ .metadata.name }"},
	})
	want := types.EnvVarList{
		{Name: "{{ .metadata.name }}", Value: "app"},
		{Name: "PLAIN", Value: "x"},
		{Name: "BAD", Value: ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if out := r.ResolveEnvVarsLenient(nil); out != nil {
		t.Errorf("nil in, got %+v", out)
	}
}
