package expect

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func TestLoadCRData(t *testing.T) {
	t.Run("nil spec returns nil", func(t *testing.T) {
		if got := loadCRData(nil, "/any"); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("empty CR field returns nil", func(t *testing.T) {
		spec := &types.E2ESpec{}
		if got := loadCRData(spec, "/any"); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("unreadable file returns nil", func(t *testing.T) {
		orig := readLocal
		readLocal = func(string) ([]byte, error) { return nil, fmt.Errorf("not found") }
		defer func() { readLocal = orig }()

		spec := &types.E2ESpec{CR: "cr.yaml"}
		if got := loadCRData(spec, "/tmp"); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("absolute path used as-is", func(t *testing.T) {
		var captured string
		orig := readLocal
		readLocal = func(p string) ([]byte, error) {
			captured = p
			return []byte("kind: Foo\nspec:\n  replicas: 3\n"), nil
		}
		defer func() { readLocal = orig }()

		spec := &types.E2ESpec{CR: "/abs/cr.yaml"}
		got := loadCRData(spec, "/work")
		if captured != "/abs/cr.yaml" {
			t.Fatalf("expected absolute path passed through, got %s", captured)
		}
		if got == nil {
			t.Fatal("expected non-nil map")
		}
	})

	t.Run("relative path joined with workDir", func(t *testing.T) {
		var captured string
		orig := readLocal
		readLocal = func(p string) ([]byte, error) {
			captured = p
			return []byte("kind: Bar\n"), nil
		}
		defer func() { readLocal = orig }()

		spec := &types.E2ESpec{CR: "cr.yaml"}
		loadCRData(spec, "/work/dir")
		want := filepath.Join("/work/dir", "cr.yaml")
		if captured != want {
			t.Fatalf("want %s, got %s", want, captured)
		}
	})

	t.Run("yaml unmarshalled into map", func(t *testing.T) {
		orig := readLocal
		readLocal = func(string) ([]byte, error) {
			return []byte("apiVersion: v1\nkind: Foo\nspec:\n  replicas: 3\n"), nil
		}
		defer func() { readLocal = orig }()

		spec := &types.E2ESpec{CR: "cr.yaml"}
		got := loadCRData(spec, "/work")
		if got["kind"] != "Foo" {
			t.Fatalf("expected kind=Foo, got %v", got["kind"])
		}
		s, _ := got["spec"].(map[string]interface{})
		if s == nil {
			t.Fatal("expected spec map")
		}
	})
}
