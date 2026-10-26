package types_test

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/types"
)

func buildData(spec map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "demo.inrun.dev/v1alpha1",
		"kind":       "Website",
		"metadata": map[string]interface{}{
			"name":      "my-website",
			"namespace": "default",
		},
		"spec": spec,
	}
}

func evalConds(data map[string]interface{}, conds []types.Condition) bool {
	return types.EvaluateConditions(data, conds, nil, nil)
}

func TestEvaluateConditions_Empty(t *testing.T) {
	data := buildData(map[string]interface{}{})
	if !evalConds(data, nil) {
		t.Error("nil conditions should always return true")
	}
	if !evalConds(data, []types.Condition{}) {
		t.Error("empty slice should always return true")
	}
}

func TestEvaluateConditions_EqualsShorthand(t *testing.T) {
	data := buildData(map[string]interface{}{"environment": "production"})

	if !evalConds(data, []types.Condition{{Field: "spec.environment", Equals: "production"}}) {
		t.Error("equals shorthand should pass when value matches")
	}
	if evalConds(data, []types.Condition{{Field: "spec.environment", Equals: "staging"}}) {
		t.Error("equals shorthand should fail when value does not match")
	}
}

func TestEvaluateConditions_AllMustPass(t *testing.T) {
	data := buildData(map[string]interface{}{
		"environment": "production",
		"enabled":     "true",
	})

	if !evalConds(data, []types.Condition{
		{Field: "spec.environment", Equals: "production"},
		{Field: "spec.enabled", Equals: "true"},
	}) {
		t.Error("all conditions pass — should return true")
	}
	if evalConds(data, []types.Condition{
		{Field: "spec.environment", Equals: "production"},
		{Field: "spec.enabled", Equals: "false"},
	}) {
		t.Error("one condition fails — should return false")
	}
}

func TestEvaluateConditions_Operators(t *testing.T) {
	data := buildData(map[string]interface{}{
		"image":    "myorg/nginx:1.25",
		"replicas": int64(5),
		"logLevel": "info",
	})

	tests := []struct {
		name   string
		cond   types.Condition
		expect bool
	}{
		{"prefix matches", types.Condition{Field: "spec.image", Prefix: "myorg/"}, true},
		{"prefix no match", types.Condition{Field: "spec.image", Prefix: "other/"}, false},
		{"suffix matches", types.Condition{Field: "spec.image", Suffix: ":1.25"}, true},
		{"contains matches", types.Condition{Field: "spec.image", Contains: "nginx"}, true},
		{"notEquals matches", types.Condition{Field: "spec.logLevel", NotEquals: "debug"}, true},
		{"gt matches — 5 > 3", types.Condition{Field: "spec.replicas", GreaterThan: "3"}, true},
		{"gt fails — 5 not > 10", types.Condition{Field: "spec.replicas", GreaterThan: "10"}, false},
		{"lt matches — 5 < 10", types.Condition{Field: "spec.replicas", LessThan: "10"}, true},
		{"exists — present field", types.Condition{Field: "spec.image", Exists: ptr.To(true)}, true},
		{"exists — absent field", types.Condition{Field: "spec.missing", Exists: ptr.To(true)}, false},
		{"notExists — absent field", types.Condition{Field: "spec.missing", NotExists: ptr.To(true)}, true},
		{"notExists — present field", types.Condition{Field: "spec.image", NotExists: ptr.To(true)}, false},
		{"in — value in list", types.Condition{Field: "spec.logLevel", In: "debug,info,warn"}, true},
		{"in — value not in list", types.Condition{Field: "spec.logLevel", In: "debug,warn"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if evalConds(data, []types.Condition{tt.cond}) != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, !tt.expect)
			}
		})
	}
}

func TestEvaluateConditions_NestedField(t *testing.T) {
	data := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"tier": "premium",
			},
		},
		"spec": map[string]interface{}{
			"database": map[string]interface{}{
				"engine": "postgres",
			},
		},
	}

	tests := []struct {
		name   string
		field  string
		equals string
		expect bool
	}{
		{"nested metadata label", "metadata.labels.tier", "premium", true},
		{"nested spec field", "spec.database.engine", "postgres", true},
		{"nested field no match", "spec.database.engine", "mysql", false},
		{"nested field missing", "spec.database.version", "14", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := types.Condition{Field: tt.field, Equals: tt.equals}
			if evalConds(data, []types.Condition{cond}) != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, !tt.expect)
			}
		})
	}
}

func TestEvaluateConditions_Or(t *testing.T) {
	data := buildData(map[string]interface{}{"phase": "Failed"})

	or := []types.Condition{
		{Field: "spec.phase", Equals: "Failed"},
		{Field: "spec.phase", Equals: "Succeeded"},
	}
	if !types.EvaluateConditions(data, nil, or, nil) {
		t.Error("or should pass when at least one condition matches")
	}

	or = []types.Condition{
		{Field: "spec.phase", Equals: "Pending"},
		{Field: "spec.phase", Equals: "Running"},
	}
	if types.EvaluateConditions(data, nil, or, nil) {
		t.Error("or should fail when no conditions match")
	}
}
