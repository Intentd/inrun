package simulate

import (
	"context"
	"testing"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
)

const fanoutCRD = "../../../examples/migration/field/fanout/manifests/crd.yaml"

func cronJobCR(name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("demo.inrun.dev/v1")
	u.SetKind("CronJob")
	u.SetName(name)
	u.SetNamespace("default")
	_ = unstructured.SetNestedField(u.Object, "busybox:latest", "spec", "image")
	return u
}

// TestCRDDefaults_AppliedOnCreate: the CR is seeded with its CRD's schema
// defaults, as the API server stores it, so a mutation rule that defaults
// the same field finds it set — the fanout example's spec.suspend.
func TestCRDDefaults_AppliedOnCreate(t *testing.T) {
	defaults := readCRDs([]string{fanoutCRD}).Defaults
	gvk := schema.GroupVersionKind{Group: "demo.inrun.dev", Version: "v1", Kind: "CronJob"}
	require.Contains(t, defaults, gvk, "the CRD's structural schema is read")

	cr := created(cronJobCR("daily-backup"), defaults)
	suspend, found, _ := unstructured.NestedFieldNoCopy(cr.Object, "spec", "suspend")
	require.True(t, found)
	assert.Equal(t, false, suspend, "a boolean default stays a boolean")
	limit, _, _ := unstructured.NestedFieldNoCopy(cr.Object, "spec", "successfulJobsHistoryLimit")
	assert.EqualValues(t, 3, limit)
	policy, _, _ := unstructured.NestedString(cr.Object, "spec", "concurrencyPolicy")
	assert.Equal(t, "Allow", policy)
}

// TestCRDDefaults_AppliedOnWrite: a CR of the kind created during the run is
// defaulted when it is stored, as the API server defaults every write.
func TestCRDDefaults_AppliedOnWrite(t *testing.T) {
	f := kubefake.New(runtime.NewScheme())
	for gvk, d := range readCRDs([]string{fanoutCRD}).Defaults {
		f.SetDefaults(gvk, d)
	}
	gvr := schema.GroupVersionResource{Group: "demo.inrun.dev", Version: "v1", Resource: "cronjobs"}
	_, err := f.DynamicClient().Resource(gvr).Namespace("default").Create(context.Background(), cronJobCR("nightly"), metav1.CreateOptions{})
	require.NoError(t, err)

	stored, err := f.Object(context.Background(), gvr, "default", "nightly")
	require.NoError(t, err)
	suspend, found, _ := unstructured.NestedFieldNoCopy(stored.Object, "spec", "suspend")
	require.True(t, found, "the stored object carries the schema default")
	assert.Equal(t, false, suspend)
}

func TestCRDDefaults_NoFilesNoDefaults(t *testing.T) {
	assert.Empty(t, readCRDs(nil).Defaults)
	assert.Empty(t, readCRDs([]string{"does-not-exist.yaml"}).Scopes, "an unreadable file contributes nothing")
}

// TestScopes_ClusterScopedChildIsStoredClusterWide: a CRD's spec.scope
// reaches the fake's mapper, so a client sends a cluster-scoped child's
// request cluster-wide, and the store drops the namespace it was built in —
// the children fixture's TenantPlan.
func TestScopes_ClusterScopedChildIsStoredClusterWide(t *testing.T) {
	crds := readCRDs([]string{"../../children/fixtures/crd.yaml"})
	gk := schema.GroupKind{Group: "", Kind: "TenantPlan"}
	for k := range crds.Scopes {
		if k.Kind == "TenantPlan" {
			gk = k
		}
	}
	require.Contains(t, crds.Scopes, gk)
	assert.False(t, crds.Scopes[gk], "TenantPlan is cluster-scoped")

	f := kubefake.New(runtime.NewScheme())
	for k, n := range crds.Scopes {
		f.SetScope(k, n)
	}
	mapping, err := f.RESTMapper().RESTMapping(gk, "v1alpha1")
	require.NoError(t, err)
	assert.Equal(t, "root", string(mapping.Scope.Name()), "the mapper answers with the CRD's scope")

	plan := &unstructured.Unstructured{}
	plan.SetAPIVersion(gk.Group + "/v1alpha1")
	plan.SetKind("TenantPlan")
	plan.SetName("acme-plan")
	plan.SetNamespace("default") // built in its owner's namespace
	_, err = f.DynamicClient().Resource(mapping.Resource).Create(context.Background(), plan, metav1.CreateOptions{})
	require.NoError(t, err)
	stored, err := f.Object(context.Background(), mapping.Resource, "", "acme-plan")
	require.NoError(t, err)
	assert.Empty(t, stored.GetNamespace(), "stored without a namespace, as the API server stores it")
}

// TestSecret_StringDataFoldedOnWrite: a Secret written during the run has
// its stringData folded into data, as the API server does on every write.
func TestSecret_StringDataFoldedOnWrite(t *testing.T) {
	f := kubefake.New(runtime.NewScheme())
	sec := &unstructured.Unstructured{}
	sec.SetAPIVersion("v1")
	sec.SetKind("Secret")
	sec.SetName("creds")
	sec.SetNamespace("team")
	_ = unstructured.SetNestedStringMap(sec.Object, map[string]string{"token": "s3cr3t"}, "stringData")
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	_, err := f.DynamicClient().Resource(gvr).Namespace("team").Create(context.Background(), sec, metav1.CreateOptions{})
	require.NoError(t, err)

	stored, err := f.Object(context.Background(), gvr, "team", "creds")
	require.NoError(t, err)
	_, has := stored.Object["stringData"]
	assert.False(t, has, "stringData is not stored")
	data, _, _ := unstructured.NestedStringMap(stored.Object, "data")
	assert.Equal(t, "czNjcjN0", data["token"], "it is in data, base64 as the API returns it")
}

// TestSecret_StringDataFoldedOnTypedWrite: Inrun writes Secrets through the
// typed clientset — created, and server-side applied — and both are folded.
func TestSecret_StringDataFoldedOnTypedWrite(t *testing.T) {
	f := kubefake.New(runtime.NewScheme())
	ctx := context.Background()
	secrets := f.Clientset().CoreV1().Secrets("team")
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}

	_, err := secrets.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "created", Namespace: "team"},
		StringData: map[string]string{"token": "s3cr3t"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = secrets.Apply(ctx, applycorev1.Secret("applied", "team").WithStringData(map[string]string{"token": "s3cr3t"}),
		metav1.ApplyOptions{FieldManager: "inrun"})
	require.NoError(t, err)

	for _, name := range []string{"created", "applied"} {
		stored, err := f.Object(ctx, gvr, "team", name)
		require.NoError(t, err)
		_, has := stored.Object["stringData"]
		assert.False(t, has, "%s: stringData is not stored", name)
		data, _, _ := unstructured.NestedStringMap(stored.Object, "data")
		assert.Equal(t, "czNjcjN0", data["token"], "%s: it is in data", name)
	}
}
