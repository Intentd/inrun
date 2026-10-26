package web

import (
	"strings"
	"testing"
	"time"
)

func TestHumanDuration(t *testing.T) {
	t.Run("empty and sentinel strings return empty", func(t *testing.T) {
		for _, in := range []string{"", "not started", "no reconciles yet"} {
			if got := humanDuration(in); got != "" {
				t.Errorf("humanDuration(%q) = %q, want empty", in, got)
			}
		}
	})

	t.Run("unparseable string returns empty", func(t *testing.T) {
		if got := humanDuration("garbage"); got != "" {
			t.Errorf("humanDuration(garbage) = %q, want empty", got)
		}
	})

	t.Run("seconds ago", func(t *testing.T) {
		ts := time.Now().Add(-30 * time.Second).Format(time.RFC3339)
		got := humanDuration(ts)
		if !strings.HasSuffix(got, "s ago") {
			t.Errorf("humanDuration(30s ago) = %q, want a *s ago suffix", got)
		}
	})

	t.Run("minutes ago", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
		got := humanDuration(ts)
		if !strings.HasSuffix(got, "m ago") {
			t.Errorf("humanDuration(5m ago) = %q, want a *m ago suffix", got)
		}
	})

	t.Run("hours ago", func(t *testing.T) {
		ts := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
		got := humanDuration(ts)
		if !strings.HasSuffix(got, "h ago") {
			t.Errorf("humanDuration(3h ago) = %q, want a *h ago suffix", got)
		}
	})
}
