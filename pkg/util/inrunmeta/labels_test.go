package inrunmeta_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/util/inrunmeta"
	"github.com/intentd/tools/kubeclient/kubefake/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWithDeletionProtection(t *testing.T) {
	base := map[string]string{
		"app.kubernetes.io/name": "inrun",
		"app.kubernetes.io/tag":  "inrun-internal",
	}

	got := inrunmeta.WithDeletionProtection(base)

	if got[inrunmeta.DeletionProtectionLabel] != "true" {
		t.Errorf("expected %s=true, got %q", inrunmeta.DeletionProtectionLabel, got[inrunmeta.DeletionProtectionLabel])
	}

	// Original map must not be mutated.
	if _, ok := base[inrunmeta.DeletionProtectionLabel]; ok {
		t.Error("WithDeletionProtection mutated the input map")
	}

	// Existing labels must be preserved.
	for k, v := range base {
		if got[k] != v {
			t.Errorf("key %q: expected %q, got %q", k, v, got[k])
		}
	}
}

func TestWithDeletionProtectionNilInput(t *testing.T) {
	got := inrunmeta.WithDeletionProtection(nil)
	if got[inrunmeta.DeletionProtectionLabel] != "true" {
		t.Errorf("expected label on nil input, got %v", got)
	}
}

func TestCheckReserved(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]string
		bad  string
	}{
		{name: "none", in: nil},
		{name: "plain key", in: map[string]string{"tier": "gold"}},
		{name: "other domain", in: map[string]string{"example.com/owner": "a"}},
		{name: "lookalike domain", in: map[string]string{"notinrun.dev/x": "a"}},
		{name: "reserved", in: map[string]string{"inrun.dev/managed": "true"}, bad: "inrun.dev/managed"},
		{name: "reserved subdomain", in: map[string]string{"cleanup.inrun.dev/x": "1"}, bad: "cleanup.inrun.dev/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := inrunmeta.CheckReserved("labels", tc.in)
			if tc.bad == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.bad)
		})
	}
}

func owner(ns, name, uid string) *unstructured.Unstructured {
	return object.New("", "", name, object.InNamespace(ns), object.WithUID(uid))
}

func TestStampInrunLabels_OwnerUID(t *testing.T) {
	got := inrunmeta.StampInrunLabels(nil, owner("team", "app", "uid-1"))
	assert.Equal(t, "uid-1", got[inrunmeta.OwnerUID])
	assert.Equal(t, "app", got[inrunmeta.InrunOwner])
	assert.Equal(t, inrunmeta.ManagedValue, got[inrunmeta.ManagedKey])
}

// Two CRs with the same name in different namespaces never match each
// other's children.
func TestOwnedBy_ByUID(t *testing.T) {
	a, b := owner("team-a", "app", "uid-a"), owner("team-b", "app", "uid-b")
	child := object.Empty(object.WithLabels(inrunmeta.StampInrunLabels(nil, a)))

	assert.True(t, inrunmeta.OwnedBy(a)(child))
	assert.False(t, inrunmeta.OwnedBy(b)(child), "same name, other namespace")
	assert.False(t, inrunmeta.OwnedBy(owner("team-a", "app", ""))(child), "an owner without a UID owns nothing")
}
