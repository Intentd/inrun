package web

import (
	"encoding/json"
	"testing"
)

func TestBuildServeField_InputTypes(t *testing.T) {
	cases := []struct {
		name string
		hint serveFieldHint
		want string
	}{
		{"enum with values", serveFieldHint{Type: "enum", Enum: []string{"a", "b"}}, "select"},
		{"enum type but no values falls through", serveFieldHint{Type: "enum"}, "text"},
		{"boolean", serveFieldHint{Type: "boolean"}, "checkbox"},
		{"integer", serveFieldHint{Type: "integer"}, "number"},
		{"number", serveFieldHint{Type: "number"}, "number"},
		{"string", serveFieldHint{Type: "string"}, "text"},
		{"type omitted defaults to text", serveFieldHint{}, "text"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := buildServeField("field", c.hint)
			if f.InputType != c.want {
				t.Errorf("InputType = %q, want %q", f.InputType, c.want)
			}
		})
	}
}

func TestBuildServeField_LabelFallback(t *testing.T) {
	f := buildServeField("workloadType", serveFieldHint{})
	if f.Label != "WorkloadType" {
		t.Errorf("Label = %q, want auto-capitalized field name %q", f.Label, "WorkloadType")
	}

	f = buildServeField("workloadType", serveFieldHint{Label: "Workload Type"})
	if f.Label != "Workload Type" {
		t.Errorf("Label = %q, want hint.Label to override the fallback", f.Label)
	}
}

func TestBuildServeField_Required(t *testing.T) {
	f := buildServeField("team", serveFieldHint{Required: true})
	if !f.Required {
		t.Error("Required = false, want true when hint.Required is set")
	}

	f = buildServeField("team", serveFieldHint{})
	if f.Required {
		t.Error("Required = true, want false when hint.Required is unset")
	}
}

func TestBuildServeField_WhenOrEncodedAsJSON(t *testing.T) {
	hint := serveFieldHint{
		When: []json.RawMessage{json.RawMessage(`{"field":"spec.workloadType","equals":"app"}`)},
		Or:   []json.RawMessage{json.RawMessage(`{"field":"spec.workloadType","equals":"cert"}`)},
	}
	f := buildServeField("repoURL", hint)

	var when []map[string]string
	if err := json.Unmarshal([]byte(f.WhenJSON), &when); err != nil {
		t.Fatalf("WhenJSON did not round-trip as JSON: %v", err)
	}
	if len(when) != 1 || when[0]["equals"] != "app" {
		t.Errorf("WhenJSON decoded to %v, want the single when: condition", when)
	}

	var or []map[string]string
	if err := json.Unmarshal([]byte(f.OrJSON), &or); err != nil {
		t.Fatalf("OrJSON did not round-trip as JSON: %v", err)
	}
	if len(or) != 1 || or[0]["equals"] != "cert" {
		t.Errorf("OrJSON decoded to %v, want the single or: condition", or)
	}
}

func TestBuildServeField_CategoryAndDisabled(t *testing.T) {
	f := buildServeField("team", serveFieldHint{Category: "Ownership", Disabled: "locked for maintenance"})
	if f.Category != "Ownership" {
		t.Errorf("Category = %q, want %q", f.Category, "Ownership")
	}
	if f.Disabled != "locked for maintenance" {
		t.Errorf("Disabled = %q, want %q", f.Disabled, "locked for maintenance")
	}
}
