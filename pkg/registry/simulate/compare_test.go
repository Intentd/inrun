package simulate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func testObj(kind, name string, fields map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       kind,
		"metadata":   map[string]any{"name": name, "namespace": "team"},
	}}
	for k, v := range fields {
		u.Object[k] = v
	}
	return u
}

func ranWith(final []*unstructured.Unstructured, ops ...Op) *Result {
	return &Result{AllOps: ops, Final: final}
}

func testWrite(cycle int, verb, resource, name string) Op {
	return Op{Cycle: cycle, Verb: verb, Resource: resource, Namespace: "team", Name: name}
}

func TestCompare_Identical(t *testing.T) {
	dep := testObj("Deployment", "web", map[string]any{"spec": map[string]any{"replicas": int64(2)}})
	ops := Ops{testWrite(1, "apply", "deployments", "web"), testWrite(1, "patch", "status", "app")}
	c := Compare("app", ranWith([]*unstructured.Unstructured{dep}, ops...), ranWith([]*unstructured.Unstructured{dep.DeepCopy()}, ops...), nil, nil)

	assert.True(t, c.Equivalent())
	assert.Equal(t, 2, c.Writes.Matched)
	require.Len(t, c.Objects, 1)
	assert.True(t, c.Objects[0].Matches())
}

func TestCompare_ReadsAndCyclesAreNotDifferences(t *testing.T) {
	mem := ranWith(nil, Op{Cycle: 1, Verb: "get", Resource: "deployments", Name: "web"}, testWrite(1, "apply", "deployments", "web"))
	env := ranWith(nil, testWrite(2, "apply", "deployments", "web"))
	assert.True(t, Compare("app", mem, env, nil, nil).Equivalent(), "the envtest recorder sees no reads, and cycles are diagnostic")
}

func TestCompare_MissingAndExtraWrites(t *testing.T) {
	mem := ranWith(nil, testWrite(1, "create", "services", "web"), testWrite(1, "apply", "deployments", "web"))
	env := ranWith(nil, testWrite(1, "apply", "deployments", "web"), testWrite(1, "apply", "deployments", "web"), testWrite(1, "patch", "labels", "app"))
	c := Compare("app", mem, env, nil, nil)

	assert.Equal(t, 3, c.Differences())
	got := map[string][2]int{}
	for _, d := range c.Writes.Differences {
		got[d.Verb+" "+d.Resource] = [2]int{d.Memory, d.Envtest}
	}
	assert.Equal(t, map[string][2]int{
		"create services":   {1, 0},
		"apply deployments": {1, 2},
		"patch labels":      {0, 1},
	}, got)
}

func TestCompare_ObjectsOnlyInOneRun(t *testing.T) {
	c := Compare("app",
		ranWith([]*unstructured.Unstructured{testObj("Deployment", "a", nil)}),
		ranWith([]*unstructured.Unstructured{testObj("Deployment", "b", nil)}), nil, nil)

	require.Len(t, c.Objects, 2)
	assert.True(t, c.Objects[0].InMemory && !c.Objects[0].InEnvtest)
	assert.True(t, !c.Objects[1].InMemory && c.Objects[1].InEnvtest)
	assert.Equal(t, 2, c.Differences())
}

func TestCompare_FieldKinds(t *testing.T) {
	mem := testObj("Deployment", "web", map[string]any{"spec": map[string]any{
		"replicas": int64(1),
		"extra":    "dropped by the schema",
		"template": map[string]any{"spec": map[string]any{"containers": []any{
			map[string]any{"name": "web", "image": "nginx:1"},
		}}},
	}})
	env := testObj("Deployment", "web", map[string]any{"spec": map[string]any{
		"replicas":             int64(3),
		"revisionHistoryLimit": int64(10),
		"template": map[string]any{"spec": map[string]any{"containers": []any{
			map[string]any{"name": "web", "image": "nginx:1", "imagePullPolicy": "IfNotPresent"},
		}}},
	}})
	c := Compare("app", ranWith([]*unstructured.Unstructured{mem}), ranWith([]*unstructured.Unstructured{env}), nil, nil)
	require.Len(t, c.Objects, 1)
	o := c.Objects[0]

	assert.Equal(t, []FieldDiff{
		{Path: "spec.extra", Kind: FieldPruned, Memory: "dropped by the schema"},
		{Path: "spec.replicas", Kind: FieldChanged, Memory: float64(1), Envtest: float64(3)},
	}, o.Differences, "several differences on one object, each with its field")
	assert.Equal(t, []FieldDiff{
		{Path: "spec.revisionHistoryLimit", Kind: FieldDefaulted, Envtest: float64(10)},
		{Path: "spec.template.spec.containers[0].imagePullPolicy", Kind: FieldDefaulted, Envtest: "IfNotPresent"},
	}, o.Defaulted)
	assert.Equal(t, 1, c.Differences(), "one object differs; defaults are not differences")
}

func TestCompare_IgnoresClusterAssignedMetadata(t *testing.T) {
	// Each run's cluster gives the owning CR its own uid; the child names
	// it in an owner reference and a label. Both name the same object.
	owned := func(crUID, rv string) []*unstructured.Unstructured {
		cr := testObj("App", "app", nil)
		cr.SetUID(types.UID(crUID))
		dep := testObj("Deployment", "web", map[string]any{"status": map[string]any{}})
		dep.SetResourceVersion(rv)
		dep.SetLabels(map[string]string{"inrun.dev/owner-uid": crUID})
		_ = unstructured.SetNestedSlice(dep.Object, []any{map[string]any{"kind": "App", "name": "app", "uid": crUID}}, "metadata", "ownerReferences")
		return []*unstructured.Unstructured{cr, dep}
	}
	mem := owned("mem-uid", "")
	env := owned("env-uid", "812")
	env[1].SetUID("dep-uid")
	env[1].SetGeneration(1)
	unstructured.SetNestedField(env[1].Object, "2026-11-03T10:00:00Z", "metadata", "creationTimestamp")
	unstructured.SetNestedField(env[1].Object, []any{map[string]any{"manager": "inrun"}}, "metadata", "managedFields")

	c := Compare("app", ranWith(mem), ranWith(env), nil, nil)
	assert.True(t, c.Equivalent(), "%+v", c.Objects)
}

func TestCompare_OwnerNamingADifferentObjectDiffers(t *testing.T) {
	ref := func(name, uid string) *unstructured.Unstructured {
		dep := testObj("Deployment", "web", nil)
		_ = unstructured.SetNestedSlice(dep.Object, []any{map[string]any{"kind": "App", "name": name, "uid": uid}}, "metadata", "ownerReferences")
		return dep
	}
	a, b := testObj("App", "a", nil), testObj("App", "b", nil)
	a.SetUID("mem-a")
	b.SetUID("env-b")
	c := Compare("app",
		ranWith([]*unstructured.Unstructured{a, ref("a", "mem-a")}),
		ranWith([]*unstructured.Unstructured{b, ref("a", "env-b")}), nil, nil)
	assert.False(t, c.Equivalent(), "an owner uid naming another object is a difference")
}

// TestCompare_DefaultedOrMissing tells the two apart by managedFields: a
// field only envtest holds is a write the in-memory run lacks when a
// server-side apply owns it — in a list element too — and otherwise a
// default, even when a create's Update entry owns it.
func TestCompare_DefaultedOrMissing(t *testing.T) {
	mem := testObj("Deployment", "web", map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
		"containers": []any{map[string]any{"name": "web", "image": "nginx:1"}},
	}}}})
	env := testObj("Deployment", "web", map[string]any{"spec": map[string]any{
		"revisionHistoryLimit": int64(10),
		"template": map[string]any{"spec": map[string]any{
			"containers": []any{map[string]any{"name": "web", "image": "nginx:1", "imagePullPolicy": "IfNotPresent", "args": []any{"--serve"}}},
		}},
	}})
	env.SetLabels(map[string]string{"inrun.dev/owner-uid": "x"})
	fields := `{"f:metadata":{"f:labels":{"f:inrun.dev/owner-uid":{}}},"f:spec":{"f:template":{"f:spec":{"f:containers":{"k:{\"name\":\"web\"}":{".":{},"f:args":{},"f:image":{},"f:name":{}}}}}}}`
	// The create's Update entry owns the defaults too: it must not count.
	created := `{"f:spec":{"f:revisionHistoryLimit":{},"f:template":{"f:spec":{"f:containers":{"k:{\"name\":\"web\"}":{"f:imagePullPolicy":{}}}}}}}`
	env.SetManagedFields([]metav1.ManagedFieldsEntry{
		{Manager: "inrun", Operation: metav1.ManagedFieldsOperationUpdate, FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(created)}},
		{Manager: "inrun", Operation: metav1.ManagedFieldsOperationApply, FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(fields)}},
	})

	c := Compare("app", ranWith([]*unstructured.Unstructured{mem}), ranWith([]*unstructured.Unstructured{env}), nil, nil)
	require.Len(t, c.Objects, 1)
	o := c.Objects[0]
	assert.Equal(t, []FieldDiff{
		{Path: `metadata.labels["inrun.dev/owner-uid"]`, Kind: FieldMissing, Envtest: "x"},
		{Path: "spec.template.spec.containers[0].args", Kind: FieldMissing, Envtest: []any{"--serve"}},
	}, o.Differences)
	assert.Equal(t, []FieldDiff{
		{Path: "spec.revisionHistoryLimit", Kind: FieldDefaulted, Envtest: float64(10)},
		{Path: "spec.template.spec.containers[0].imagePullPolicy", Kind: FieldDefaulted, Envtest: "IfNotPresent"},
	}, o.Defaulted)
}

func TestCompare_TimesAndEmbeddedJSON(t *testing.T) {
	mem := testObj("App", "app", map[string]any{
		"metadata": map[string]any{"name": "app", "namespace": "team", "annotations": map[string]any{
			"inrun.dev/health": `{"healthy":true,"lastReconcile":"2026-11-03T10:00:00Z"}`,
		}},
		"status": map[string]any{"since": "2026-11-03T10:00:00Z"},
	})
	env := testObj("App", "app", map[string]any{
		"metadata": map[string]any{"name": "app", "namespace": "team", "annotations": map[string]any{
			"inrun.dev/health": `{"healthy":false,"lastReconcile":"2026-11-03T10:00:09Z"}`,
		}},
		"status": map[string]any{"since": "2026-11-03T10:00:09Z"},
	})
	c := Compare("app", ranWith([]*unstructured.Unstructured{mem}), ranWith([]*unstructured.Unstructured{env}), nil, nil)
	require.Len(t, c.Objects, 1)
	assert.Equal(t, []FieldDiff{
		{Path: `metadata.annotations["inrun.dev/health"].healthy`, Kind: FieldChanged, Memory: true, Envtest: false},
	}, c.Objects[0].Differences, "times differ by run, not outcome; the JSON field that does differ is named")
}

func TestCompare_IncompleteWhenARunFails(t *testing.T) {
	c := Compare("app", ranWith(nil, testWrite(1, "apply", "deployments", "web")), nil, nil, errors.New("envtest: starting API server"))

	assert.False(t, c.Complete())
	assert.False(t, c.Equivalent(), "an incomplete comparison is never equivalent")
	assert.Zero(t, c.Differences())
	assert.Empty(t, c.Writes.Differences)
	assert.Empty(t, c.Objects)
}
