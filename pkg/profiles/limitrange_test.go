package profiles_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/profiles"
	"github.com/intentd/inrun/pkg/types"
)

func TestLimitRangeProfileUserDefined(t *testing.T) {
	reg := types.ProfileRegistry{
		LimitRanges: []types.LimitRangeProfileDef{
			{
				Name: "default-container",
				Limits: []types.LimitRangeItem{
					{
						Type:           "Container",
						Default:        map[string]string{"cpu": "500m", "memory": "256Mi"},
						DefaultRequest: map[string]string{"cpu": "100m", "memory": "64Mi"},
						Max:            map[string]string{"cpu": "2", "memory": "1Gi"},
					},
				},
			},
		},
	}

	items, err := profiles.ApplyLimitRangeProfile("default-container", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 limit item got %d", len(items))
	}
	item := items[0]
	if item.Type != "Container" {
		t.Errorf("type: want Container got %q", item.Type)
	}
	if got := item.Default["cpu"]; got != "500m" {
		t.Errorf("default.cpu: want 500m got %q", got)
	}
	if got := item.DefaultRequest["memory"]; got != "64Mi" {
		t.Errorf("defaultRequest.memory: want 64Mi got %q", got)
	}
	if got := item.Max["cpu"]; got != "2" {
		t.Errorf("max.cpu: want 2 got %q", got)
	}
}

func TestLimitRangeProfileMultipleItems(t *testing.T) {
	reg := types.ProfileRegistry{
		LimitRanges: []types.LimitRangeProfileDef{
			{
				Name: "multi",
				Limits: []types.LimitRangeItem{
					{Type: "Container", Max: map[string]string{"cpu": "2"}},
					{Type: "Pod", Max: map[string]string{"cpu": "4", "memory": "4Gi"}},
				},
			},
		},
	}

	items, err := profiles.ApplyLimitRangeProfile("multi", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items got %d", len(items))
	}
	if items[0].Type != "Container" || items[1].Type != "Pod" {
		t.Errorf("unexpected item types: %v", []string{items[0].Type, items[1].Type})
	}
}

func TestLimitRangeProfileUnknownReturnsError(t *testing.T) {
	_, err := profiles.ApplyLimitRangeProfile("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown limitrange profile but got none")
	}
}

func TestIsValidLimitRangeProfile(t *testing.T) {
	reg := types.ProfileRegistry{
		LimitRanges: []types.LimitRangeProfileDef{
			{Name: "my-limits", Limits: []types.LimitRangeItem{{Type: "Container"}}},
		},
	}

	if !profiles.IsValidLimitRangeProfile("my-limits", &reg) {
		t.Error("expected my-limits to be valid")
	}

	if profiles.IsValidLimitRangeProfile("unknown", &reg) {
		t.Error("expected unknown to be invalid")
	}

	if profiles.IsValidLimitRangeProfile("my-limits", nil) {
		t.Error("expected my-limits to be invalid with empty registry")
	}
}
