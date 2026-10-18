package fileio

import (
	"os"
	"testing"
)

func TestCacheFileBytes_RoundTrip(t *testing.T) {
	url := "https://example.com/test-catalog.yaml"
	data := []byte("apiVersion: inrun.dev/v1\nkind: Catalog\n")

	// Ensure clean state
	InvalidateFileCache(url)

	// Nothing cached yet
	if _, ok := CachedFileBytes(url); ok {
		t.Fatal("expected cache miss before first write")
	}

	// Write to cache
	if err := CacheFileBytes(url, data); err != nil {
		t.Fatalf("CacheFileBytes: %v", err)
	}

	// Should hit now
	got, ok := CachedFileBytes(url)
	if !ok {
		t.Fatal("expected cache hit after write")
	}
	if string(got) != string(data) {
		t.Fatalf("cached content mismatch: got %q, want %q", got, data)
	}

	// Invalidate
	InvalidateFileCache(url)
	if _, ok := CachedFileBytes(url); ok {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestCacheFileBytes_DifferentURLsDifferentEntries(t *testing.T) {
	urlA := "https://example.com/a.yaml"
	urlB := "https://example.com/b.yaml"

	InvalidateFileCache(urlA)
	InvalidateFileCache(urlB)

	if err := CacheFileBytes(urlA, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := CacheFileBytes(urlB, []byte("b")); err != nil {
		t.Fatal(err)
	}

	gotA, _ := CachedFileBytes(urlA)
	gotB, _ := CachedFileBytes(urlB)

	if string(gotA) != "a" {
		t.Errorf("urlA: got %q, want %q", gotA, "a")
	}
	if string(gotB) != "b" {
		t.Errorf("urlB: got %q, want %q", gotB, "b")
	}

	InvalidateFileCache(urlA)
	InvalidateFileCache(urlB)
}

func TestLoadFileWithAuthRefresh_LocalFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("hello: world")
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := LoadFileWithAuthRefresh(f.Name(), nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("got %q, want %q", got, content)
	}
}

func TestLoadFileWithAuthRefresh_CacheHit(t *testing.T) {
	url := "https://example.com/cached-source.yaml"
	cached := []byte("from: cache")

	InvalidateFileCache(url)
	if err := CacheFileBytes(url, cached); err != nil {
		t.Fatal(err)
	}
	defer InvalidateFileCache(url)

	// Should return cached bytes without making a network call
	got, err := LoadFileWithAuthRefresh(url, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(cached) {
		t.Fatalf("got %q, want %q", got, cached)
	}
}
