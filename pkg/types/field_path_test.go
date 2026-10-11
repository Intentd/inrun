package types

import "testing"

func TestDottedLabelPath(t *testing.T) {
	data := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{"app.kubernetes.io/name": "web"},
		},
	}
	path := `metadata.labels.app\.kubernetes\.io/name`

	if v, ok := ResolveScalarField(data, path); !ok || v != "web" {
		t.Errorf("ResolveScalarField = %q, %v", v, ok)
	}
	if v := NavigateDotPath(data, path); v != "web" {
		t.Errorf("NavigateDotPath = %q", v)
	}
	cond := []Condition{{Field: path, Equals: "web"}}
	if !EvaluateConditions(data, cond, nil, nil) {
		t.Error("condition on a dotted label path did not match")
	}
}

// The rules generated for serve labels and annotations read and write the
// label itself, dots in the key included.
func TestServeLabelRules_TargetLabelPath(t *testing.T) {
	c := &CRDEntry{Serve: &ServeConfig{Labels: map[string]ServeFieldConfig{
		"app.kubernetes.io/name": {Required: true, Default: "web"},
	}}}
	want := `metadata.labels.app\.kubernetes\.io/name`

	if r := c.RequiredServeFieldRules(); len(r) != 1 || r[0].Field != want || r[0].Link != "app.kubernetes.io/name" {
		t.Errorf("required rule = %+v", r)
	}
	if r := c.ServeFieldMutationRules(); len(r) != 1 || r[0].Field != want {
		t.Errorf("mutation rule = %+v", r)
	}
}
