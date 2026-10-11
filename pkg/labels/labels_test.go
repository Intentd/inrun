package labels_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithDeletionProtection(t *testing.T) {
	base := map[string]string{
		"app.kubernetes.io/name": "inrun",
		"app.kubernetes.io/tag":  "inrun-internal",
	}

	got := labels.WithDeletionProtection(base)

	if got[labels.DeletionProtectionLabel] != "true" {
		t.Errorf("expected %s=true, got %q", labels.DeletionProtectionLabel, got[labels.DeletionProtectionLabel])
	}

	// Original map must not be mutated.
	if _, ok := base[labels.DeletionProtectionLabel]; ok {
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
	got := labels.WithDeletionProtection(nil)
	if got[labels.DeletionProtectionLabel] != "true" {
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
			err := labels.CheckReserved("labels", tc.in)
			if tc.bad == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.bad)
		})
	}
}
