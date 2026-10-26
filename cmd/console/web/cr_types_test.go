package web

import (
	"encoding/json"
	"testing"
)

func TestCountReady(t *testing.T) {
	items := []ChildSummary{{Ready: true}, {Ready: false}, {Ready: true}}
	if got := countReady(items); got != 2 {
		t.Errorf("countReady = %d, want 2", got)
	}
	if got := countReady(nil); got != 0 {
		t.Errorf("countReady(nil) = %d, want 0", got)
	}
}

func TestNormalizeChildGroups(t *testing.T) {
	t.Run("empty input returns nil", func(t *testing.T) {
		if got := normalizeChildGroups(nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("single object — flat, not array", func(t *testing.T) {
		raw := map[string]json.RawMessage{
			"deployment": json.RawMessage(`{"name":"web","ready":true}`),
		}
		groups := normalizeChildGroups(raw)
		if len(groups) != 1 {
			t.Fatalf("got %d groups, want 1", len(groups))
		}
		g := groups[0]
		if g.Kind != "deployment" || g.IsMultiple || g.ReadyCount != 1 || len(g.Items) != 1 {
			t.Errorf("got %+v, want a single ready deployment", g)
		}
	})

	t.Run("array — multiple children of the same kind", func(t *testing.T) {
		raw := map[string]json.RawMessage{
			"application": json.RawMessage(`[{"name":"b","ready":false},{"name":"a","ready":true}]`),
		}
		groups := normalizeChildGroups(raw)
		if len(groups) != 1 {
			t.Fatalf("got %d groups, want 1", len(groups))
		}
		g := groups[0]
		if !g.IsMultiple || g.ReadyCount != 1 || len(g.Items) != 2 {
			t.Errorf("got %+v, want IsMultiple=true ReadyCount=1 with 2 items", g)
		}
		// Sorted by name ascending within the group.
		if g.Items[0].Name != "a" || g.Items[1].Name != "b" {
			t.Errorf("items not sorted by name: %+v", g.Items)
		}
	})

	t.Run("groups sorted by kind", func(t *testing.T) {
		raw := map[string]json.RawMessage{
			"service":    json.RawMessage(`{"name":"svc"}`),
			"deployment": json.RawMessage(`{"name":"dep"}`),
		}
		groups := normalizeChildGroups(raw)
		if len(groups) != 2 || groups[0].Kind != "deployment" || groups[1].Kind != "service" {
			t.Errorf("got %+v, want [deployment, service] order", groups)
		}
	})

	t.Run("empty array is skipped, not rendered as a zero-value single object", func(t *testing.T) {
		raw := map[string]json.RawMessage{
			"deployment": json.RawMessage(`[]`),
		}
		groups := normalizeChildGroups(raw)
		// An empty array unmarshals into an empty (non-nil) slice, which fails
		// the len(arr) > 0 check and falls through to the single-object arm.
		// json.Unmarshal([]byte("[]"), &ChildSummary{}) errors (type mismatch),
		// so the kind is dropped entirely rather than rendered as a blank card.
		if len(groups) != 0 {
			t.Errorf("got %+v, want the empty-array kind dropped entirely", groups)
		}
	})
}
