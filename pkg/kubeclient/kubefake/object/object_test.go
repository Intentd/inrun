package object

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNew(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	u := New("demo.inrun.dev/v1", "Website", "web",
		InNamespace("team"),
		WithUID("uid-1"),
		WithLabels(map[string]string{"app": "web"}),
		WithAnnotations(map[string]string{"inrun.dev/owner": "web-app"}),
		WithFinalizers("inrun.dev/cleanup"),
		Deleting(at),
		ManagedBy("inrun"),
		WithSpec(map[string]any{"domain": "example.com"}),
		WithField("ready", "status", "phase"))

	assert.Equal(t, "demo.inrun.dev/v1", u.GetAPIVersion())
	assert.Equal(t, "Website", u.GetKind())
	assert.Equal(t, "team/web", u.GetNamespace()+"/"+u.GetName())
	assert.Equal(t, "uid-1", string(u.GetUID()))
	assert.Equal(t, map[string]string{"app": "web"}, u.GetLabels())
	assert.Equal(t, map[string]string{"inrun.dev/owner": "web-app"}, u.GetAnnotations())
	assert.Equal(t, []string{"inrun.dev/cleanup"}, u.GetFinalizers())
	assert.True(t, u.GetDeletionTimestamp().Time.Equal(at))
	assert.Equal(t, "inrun", u.GetManagedFields()[0].Manager)
	domain, _, _ := unstructured.NestedString(u.Object, "spec", "domain")
	phase, _, _ := unstructured.NestedString(u.Object, "status", "phase")
	assert.Equal(t, "example.com", domain)
	assert.Equal(t, "ready", phase)
}

func TestNew_EmptyFieldsUnset(t *testing.T) {
	u := New("", "", "")
	assert.Empty(t, u.Object, "an object with nothing set has no fields")
}

func TestOwner(t *testing.T) {
	o := Owner(WithUID("other"))
	assert.Equal(t, OwnerNamespace, o.GetNamespace())
	assert.Equal(t, "other", string(o.GetUID()), "options apply after the defaults")
	assert.Empty(t, ClusterOwner().GetNamespace())
}
