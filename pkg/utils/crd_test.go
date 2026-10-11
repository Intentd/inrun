package utils

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildAnnotationPatch(t *testing.T) {
	patch := BuildAnnotationPatch("inrun.dev/managed-by", "inrun")
	meta, ok := patch["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metadata map")
	}
	ann, ok := meta["annotations"].(map[string]interface{})
	if !ok {
		t.Fatal("expected annotations map")
	}
	if ann["inrun.dev/managed-by"] != "inrun" {
		t.Errorf("unexpected annotation value: %v", ann["inrun.dev/managed-by"])
	}
}

func TestBuildAnnotationPatch_DifferentKey(t *testing.T) {
	patch := BuildAnnotationPatch("foo", "bar")
	meta := patch["metadata"].(map[string]interface{})
	ann := meta["annotations"].(map[string]interface{})
	if ann["foo"] != "bar" {
		t.Errorf("expected bar, got %v", ann["foo"])
	}
}

func TestMerge_EmptyTarget(t *testing.T) {
	s := ""
	Merge(&s, "hello", ",")
	if s != "hello" {
		t.Errorf("expected hello, got %q", s)
	}
}

func TestMerge_NonEmptyTarget(t *testing.T) {
	s := "hello"
	Merge(&s, "world", ",")
	if s != "hello,world" {
		t.Errorf("expected hello,world, got %q", s)
	}
}

func TestMerge_SpaceSeparator(t *testing.T) {
	s := "a"
	Merge(&s, "b", " ")
	if s != "a b" {
		t.Errorf("expected 'a b', got %q", s)
	}
}

func TestRequireStrParams_AllPresent(t *testing.T) {
	err := RequireStrParams(map[string]string{"a": "1", "b": "2"})
	if err != nil {
		t.Errorf("all present must not error: %v", err)
	}
}

func TestRequireStrParams_MissingValue(t *testing.T) {
	err := RequireStrParams(map[string]string{"key": ""})
	if err == nil {
		t.Error("empty value must return error")
	}
}

// ── sortedKinds ──────────────────────────────────────────────────────────────

func TestSortedKinds_Empty(t *testing.T) {
	assert.Empty(t, sortedKinds(nil))
	assert.Empty(t, sortedKinds(map[string][]*unstructured.Unstructured{}))
}

func TestSortedKinds_SortsAscending(t *testing.T) {
	crs := map[string][]*unstructured.Unstructured{
		"Service":    nil,
		"Deployment": nil,
		"ConfigMap":  nil,
		"Ingress":    nil,
	}
	assert.Equal(t, []string{"ConfigMap", "Deployment", "Ingress", "Service"}, sortedKinds(crs))
}

func TestSortedKinds_DoesNotMutateInput(t *testing.T) {
	crs := map[string][]*unstructured.Unstructured{
		"B": nil,
		"A": nil,
	}
	_ = sortedKinds(crs)
	// Still has both keys and the same length; we can't assert order on a map,
	// but we can assert nothing was added or removed.
	assert.Len(t, crs, 2)
}

// ── DumpCRsJSON ──────────────────────────────────────────────────────────────

func TestDumpCRsJSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	DumpCRsJSON(&buf, nil)
	assert.Empty(t, buf.String())

	buf.Reset()
	DumpCRsJSON(&buf, map[string][]*unstructured.Unstructured{})
	assert.Empty(t, buf.String())
}

func TestDumpCRsJSON_SingleObject(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "hello-intent"},
	}}

	var buf bytes.Buffer
	DumpCRsJSON(&buf, map[string][]*unstructured.Unstructured{"Deployment": {u}})

	out := buf.String()
	assert.Contains(t, out, "Deployment[0]:")
	assert.Contains(t, out, `"kind": "Deployment"`)
	assert.Contains(t, out, `"name": "hello-intent"`)
}

func TestDumpCRsJSON_SortedByKind(t *testing.T) {
	crs := map[string][]*unstructured.Unstructured{
		"Service":    {{Object: map[string]interface{}{"kind": "Service"}}},
		"Deployment": {{Object: map[string]interface{}{"kind": "Deployment"}}},
	}

	var buf bytes.Buffer
	DumpCRsJSON(&buf, crs)

	out := buf.String()
	require.Contains(t, out, "Deployment[0]:")
	require.Contains(t, out, "Service[0]:")
	assert.Less(t, strings.Index(out, "Deployment[0]:"), strings.Index(out, "Service[0]:"),
		"Deployment should be dumped before Service")
}

func TestDumpCRsJSON_IndexesMultipleObjects(t *testing.T) {
	list := []*unstructured.Unstructured{
		{Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "a"}}},
		{Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "b"}}},
	}

	var buf bytes.Buffer
	DumpCRsJSON(&buf, map[string][]*unstructured.Unstructured{"Deployment": list})

	out := buf.String()
	assert.Contains(t, out, "Deployment[0]:")
	assert.Contains(t, out, "Deployment[1]:")
	assert.Less(t, strings.Index(out, `"name": "a"`), strings.Index(out, `"name": "b"`))
}

func TestDumpCRsJSON_MarshalErrorDoesNotStopWalk(t *testing.T) {
	// A channel is not JSON-representable, so the first object fails to marshal.
	// The second must still be dumped — that's the "best effort" contract.
	bad := &unstructured.Unstructured{Object: map[string]interface{}{"oops": make(chan int)}}
	good := &unstructured.Unstructured{Object: map[string]interface{}{"kind": "Service"}}

	var buf bytes.Buffer
	DumpCRsJSON(&buf, map[string][]*unstructured.Unstructured{
		"Broken":  {bad},
		"Service": {good},
	})

	out := buf.String()
	assert.Contains(t, out, "Broken[0]: marshal error:")
	assert.Contains(t, out, "Service[0]:")
	assert.Contains(t, out, `"kind": "Service"`)
}

// ── DumpCRsYAML ──────────────────────────────────────────────────────────────

func TestDumpCRsYAML_Empty(t *testing.T) {
	var buf bytes.Buffer
	DumpCRsYAML(&buf, nil)
	assert.Empty(t, buf.String())
}

func TestDumpCRsYAML_SingleObject(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "hello-intent"},
	}}

	var buf bytes.Buffer
	DumpCRsYAML(&buf, map[string][]*unstructured.Unstructured{"Deployment": {u}})

	out := buf.String()
	assert.Contains(t, out, "Deployment[0]:")
	assert.Contains(t, out, "kind: Deployment")
	assert.Contains(t, out, "name: hello-intent")
}

func TestDumpCRsYAML_SortedByKind(t *testing.T) {
	crs := map[string][]*unstructured.Unstructured{
		"Service":    {{Object: map[string]interface{}{"kind": "Service"}}},
		"Deployment": {{Object: map[string]interface{}{"kind": "Deployment"}}},
	}

	var buf bytes.Buffer
	DumpCRsYAML(&buf, crs)

	out := buf.String()
	require.Contains(t, out, "Deployment[0]:")
	require.Contains(t, out, "Service[0]:")
	assert.Less(t, strings.Index(out, "Deployment[0]:"), strings.Index(out, "Service[0]:"))
}

// ── Determinism ──────────────────────────────────────────────────────────────

func TestDumpCRs_DeterministicAcrossRuns(t *testing.T) {
	crs := map[string][]*unstructured.Unstructured{
		"Service":    {{Object: map[string]interface{}{"kind": "Service"}}},
		"Deployment": {{Object: map[string]interface{}{"kind": "Deployment"}}},
		"ConfigMap":  {{Object: map[string]interface{}{"kind": "ConfigMap"}}},
	}

	var first bytes.Buffer
	DumpCRsJSON(&first, crs)

	for i := 0; i < 10; i++ {
		var next bytes.Buffer
		DumpCRsJSON(&next, crs)
		require.Equal(t, first.String(), next.String(), "run %d differed", i)
	}
}
