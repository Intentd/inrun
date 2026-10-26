package types

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPruneConfig_Decode(t *testing.T) {
	var fromYAML ReconcileConfig
	if err := yaml.Unmarshal([]byte("prune:\n  enabled: true\n  allowEmpty: true\n  keep: [persistentvolumeclaim]\n"), &fromYAML); err != nil {
		t.Fatal(err)
	}
	var fromJSON ReconcileConfig
	if err := json.Unmarshal([]byte(`{"prune":{"enabled":true,"allowEmpty":true,"keep":["persistentvolumeclaim"]}}`), &fromJSON); err != nil {
		t.Fatal(err)
	}
	for name, rc := range map[string]ReconcileConfig{"yaml": fromYAML, "json": fromJSON} {
		p := rc.Prune
		if !p.IsEnabled() || !p.AllowEmpty || len(p.Keep) != 1 || p.Keep[0] != "persistentvolumeclaim" {
			t.Errorf("%s: got %+v", name, p)
		}
	}
	if (*PruneConfig)(nil).IsEnabled() {
		t.Error("nil config must be off")
	}
}
