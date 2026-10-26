package structured

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRequireErr(t *testing.T) {
	t.Run("matches", func(t *testing.T) {
		errs := []error{errors.New("foo"), errors.New("bar baz")}
		RequireErr(t, errs, "bar")
	})

	t.Run("no match fails", func(t *testing.T) {
		ft := &testing.T{}
		errs := []error{errors.New("foo"), errors.New("bar")}
		RequireErr(ft, errs, "nope")
		if !ft.Failed() {
			t.Fatal("expected RequireErr to fail")
		}
	})

	t.Run("empty slice fails", func(t *testing.T) {
		ft := &testing.T{}
		RequireErr(ft, nil, "anything")
		if !ft.Failed() {
			t.Fatal("expected RequireErr to fail")
		}
	})
}

func TestWriteFile(t *testing.T) {
	t.Run("writes content", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.txt")
		WriteFile(t, path, "hello")
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "hello" {
			t.Fatalf("got %q, want %q", got, "hello")
		}
	})

	t.Run("fails on bad path", func(t *testing.T) {
		ft := &testing.T{}
		done := make(chan struct{})
		go func() {
			defer close(done)
			WriteFile(ft, filepath.Join(t.TempDir(), "missing", "out.txt"), "x")
		}()
		<-done
		if !ft.Failed() {
			t.Fatal("expected WriteFile to fail")
		}
	})
}
