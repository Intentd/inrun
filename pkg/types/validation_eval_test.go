package types

import (
	"fmt"
	"testing"
)

// ── ScalarToString ──────────────────────────────────────────────────────────

func TestScalarToString(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{name: "string", input: "hello", want: "hello"},
		{name: "empty string", input: "", want: ""},
		{name: "bool true", input: true, want: "true"},
		{name: "bool false", input: false, want: "false"},
		{name: "int64 positive", input: int64(42), want: "42"},
		{name: "int64 negative", input: int64(-7), want: "-7"},
		{name: "int64 zero", input: int64(0), want: "0"},
		{name: "float64 integer value", input: float64(5), want: "5"},
		{name: "float64 decimal value", input: float64(3.14), want: "3.14"},
		{name: "float64 zero", input: float64(0), want: "0"},
		{name: "nil", input: nil, want: ""},
		{name: "other type falls back to fmt", input: []string{"a", "b"}, want: "[a b]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScalarToString(tc.input); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ── ResolveScalarField ──────────────────────────────────────────────────────

func TestResolveScalarField(t *testing.T) {
	data := map[string]interface{}{
		"apiVersion": "demo.inrun.dev/v1alpha1",
		"metadata": map[string]interface{}{
			"name":      "my-site",
			"namespace": "default",
			"labels": map[string]interface{}{
				"team": "platform",
			},
		},
		"spec": map[string]interface{}{
			"stringVal":  "hello",
			"intVal":     int64(42),
			"floatVal":   float64(3.14),
			"boolTrue":   true,
			"boolFalse":  false,
			"intAsFloat": float64(10),
			"image":      "myorg/nginx:1.25",
			"replicas":   float64(3),
			"enabled":    true,
			"db": map[string]interface{}{
				"engine": "postgres",
			},
		},
	}

	tests := []struct {
		name      string
		path      string
		wantValue string
		wantFound bool
	}{
		{"top-level string", "apiVersion", "demo.inrun.dev/v1alpha1", true},
		{"nested string", "spec.stringVal", "hello", true},
		{"nested int", "spec.intVal", "42", true},
		{"nested float", "spec.floatVal", "3.14", true},
		{"nested bool true", "spec.boolTrue", "true", true},
		{"nested bool false", "spec.boolFalse", "false", true},
		{"float that is a whole number", "spec.intAsFloat", "10", true},
		{"nested float64 (int)", "spec.replicas", "3", true},
		{"nested bool", "spec.enabled", "true", true},
		{"deeply nested", "spec.db.engine", "postgres", true},
		{"nested under metadata", "metadata.name", "my-site", true},
		{"three levels deep", "metadata.labels.team", "platform", true},
		{"missing top-level", "status", "", false},
		{"missing nested", "spec.port", "", false},
		{"missing deeply nested", "spec.db.host", "", false},
		{"intermediate is a leaf not a map", "spec.image.subfield", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := ResolveScalarField(data, tt.path)
			if found != tt.wantFound {
				t.Fatalf("found = %v, want %v", found, tt.wantFound)
			}
			if got != tt.wantValue {
				t.Errorf("value = %q, want %q", got, tt.wantValue)
			}
		})
	}
}

func TestResolveScalarField_EmptyObject(t *testing.T) {
	if val, found := ResolveScalarField(map[string]interface{}{}, "spec.image"); found || val != "" {
		t.Errorf("got (%q, %v), want (\"\", false)", val, found)
	}
}

// ── ResolveValidationOp ──────────────────────────────────────────────────────

func TestResolveValidationOp_Shorthands(t *testing.T) {
	tests := []struct {
		name    string
		rule    ValidationRule
		wantOp  ConditionOperator
		wantVal string
	}{
		{"equals", ValidationRule{Equals: "production"}, ConditionEquals, "production"},
		{"notEquals", ValidationRule{NotEquals: "test"}, ConditionNotEquals, "test"},
		{"prefix", ValidationRule{Prefix: "myorg/"}, ConditionPrefix, "myorg/"},
		{"suffix", ValidationRule{Suffix: ":latest"}, ConditionSuffix, ":latest"},
		{"contains", ValidationRule{Contains: "nginx"}, ConditionContains, "nginx"},
		{"min maps to gte", ValidationRule{Min: "1"}, ConditionGte, "1"},
		{"max maps to lte", ValidationRule{Max: "10"}, ConditionLte, "10"},
		{"greaterThan maps to gt", ValidationRule{GreaterThan: "5"}, ConditionGt, "5"},
		{"lessThan maps to lt", ValidationRule{LessThan: "5"}, ConditionLt, "5"},
		{"explicit operator", ValidationRule{In: "a,b"}, ConditionIn, "a,b"},
		{"no shorthand defaults to exists", ValidationRule{Field: "spec.image"}, ConditionExists, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, val := ResolveValidationOp(tt.rule)
			if op != tt.wantOp || val != tt.wantVal {
				t.Errorf("got (%q, %q), want (%q, %q)", op, val, tt.wantOp, tt.wantVal)
			}
		})
	}
}

// ── EvaluateValidationRule ────────────────────────────────────────────────────

func specData(fields map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"spec": fields}
}

func TestEvaluateValidationRule_Operators(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		rule     ValidationRule
		wantPass bool
	}{
		// ── exists ──────────────────────────────────────────────────────────
		{"exists: field present with value — passes", specData(map[string]interface{}{"image": "myorg/app:v1"}), ValidationRule{Field: "spec.image", Exists: boolPtr(true), Message: "required"}, true},
		{"exists: field missing — fails", specData(map[string]interface{}{}), ValidationRule{Field: "spec.image", Exists: boolPtr(true), Message: "required"}, false},
		{"exists: field present but empty string — fails", specData(map[string]interface{}{"image": ""}), ValidationRule{Field: "spec.image", Exists: boolPtr(true), Message: "required"}, false},

		// ── notExists ───────────────────────────────────────────────────────
		{"notExists: field absent — passes", specData(map[string]interface{}{}), ValidationRule{Field: "spec.debug", NotExists: boolPtr(true), Message: "debug not allowed"}, true},
		{"notExists: field present with value — fails", specData(map[string]interface{}{"debug": "true"}), ValidationRule{Field: "spec.debug", NotExists: boolPtr(true), Message: "debug not allowed"}, false},
		{"notExists: field present but empty — passes", specData(map[string]interface{}{"debug": ""}), ValidationRule{Field: "spec.debug", NotExists: boolPtr(true), Message: "debug not allowed"}, true},

		// ── equals ──────────────────────────────────────────────────────────
		{"equals: exact match — passes", specData(map[string]interface{}{"env": "production"}), ValidationRule{Field: "spec.env", Equals: "production", Message: "must be production"}, true},
		{"equals: no match — fails", specData(map[string]interface{}{"env": "staging"}), ValidationRule{Field: "spec.env", Equals: "production", Message: "must be production"}, false},
		{"equals: field missing — fails", specData(map[string]interface{}{}), ValidationRule{Field: "spec.env", Equals: "production", Message: "must be production"}, false},

		// ── notEquals ───────────────────────────────────────────────────────
		{"notEquals: different value — passes", specData(map[string]interface{}{"env": "staging"}), ValidationRule{Field: "spec.env", NotEquals: "production", Message: "must not be production"}, true},
		{"notEquals: matching value — fails", specData(map[string]interface{}{"env": "production"}), ValidationRule{Field: "spec.env", NotEquals: "production", Message: "must not be production"}, false},
		{"notEquals: field missing — passes", specData(map[string]interface{}{}), ValidationRule{Field: "spec.env", NotEquals: "production", Message: "must not be production"}, true},

		// ── prefix ──────────────────────────────────────────────────────────
		{"prefix: correct prefix — passes", specData(map[string]interface{}{"image": "myorg/nginx:1.25"}), ValidationRule{Field: "spec.image", Prefix: "myorg/", Message: "must use myorg registry"}, true},
		{"prefix: wrong prefix — fails", specData(map[string]interface{}{"image": "docker.io/nginx:1.25"}), ValidationRule{Field: "spec.image", Prefix: "myorg/", Message: "must use myorg registry"}, false},
		{"prefix: field missing — fails", specData(map[string]interface{}{}), ValidationRule{Field: "spec.image", Prefix: "myorg/", Message: "must use myorg registry"}, false},

		// ── suffix ──────────────────────────────────────────────────────────
		{"suffix: correct suffix — passes", specData(map[string]interface{}{"image": "myorg/app:v1.0"}), ValidationRule{Field: "spec.image", Suffix: ":v1.0", Message: "must use v1.0 tag"}, true},
		{"suffix: wrong suffix — fails", specData(map[string]interface{}{"image": "myorg/app:latest"}), ValidationRule{Field: "spec.image", Suffix: ":v1.0", Message: "must use v1.0 tag"}, false},

		// ── contains ────────────────────────────────────────────────────────
		{"contains: substring present — passes", specData(map[string]interface{}{"image": "registry.myorg.io/nginx:1.25"}), ValidationRule{Field: "spec.image", Contains: "myorg", Message: "must be from myorg"}, true},
		{"contains: substring absent — fails", specData(map[string]interface{}{"image": "docker.io/nginx:1.25"}), ValidationRule{Field: "spec.image", Contains: "myorg", Message: "must be from myorg"}, false},

		// ── in ──────────────────────────────────────────────────────────────
		// The exact regression this suite guards: operator: in was defined
		// for when:/or: conditions but missing from the validation-rule
		// evaluation switch, so a rule using it always passed silently, in
		// both the reconciler and the webhook, at once.
		{"in: value in list — passes", specData(map[string]interface{}{"workloadType": "app"}), ValidationRule{Field: "spec.workloadType", In: "app,cert,monitoring,infra", Message: "invalid"}, true},
		{"in: value not in list — fails", specData(map[string]interface{}{"workloadType": "bogus"}), ValidationRule{Field: "spec.workloadType", In: "app,cert,monitoring,infra", Message: "invalid"}, false},
		{"in: field missing — fails", specData(map[string]interface{}{}), ValidationRule{Field: "spec.workloadType", In: "app,cert", Message: "invalid"}, false},
		{"in: whitespace around list entries is trimmed", specData(map[string]interface{}{"env": "prod"}), ValidationRule{Field: "spec.env", In: "dev, staging, prod", Message: "invalid"}, true},

		// ── min (ConditionGte) — inclusive ───────────────────────────────────
		{"min: value exactly at minimum — passes", specData(map[string]interface{}{"replicas": float64(1)}), ValidationRule{Field: "spec.replicas", Min: "1", Message: "at least 1 replica"}, true},
		{"min: value above minimum — passes", specData(map[string]interface{}{"replicas": float64(5)}), ValidationRule{Field: "spec.replicas", Min: "1", Message: "at least 1 replica"}, true},
		{"min: value below minimum — fails", specData(map[string]interface{}{"replicas": float64(0)}), ValidationRule{Field: "spec.replicas", Min: "1", Message: "at least 1 replica"}, false},
		{"min: field missing — fails", specData(map[string]interface{}{}), ValidationRule{Field: "spec.replicas", Min: "1", Message: "at least 1 replica"}, false},
		{"min: non-numeric field value — fails", specData(map[string]interface{}{"replicas": "lots"}), ValidationRule{Field: "spec.replicas", Min: "1", Message: "at least 1 replica"}, false},
		{"min: non-numeric config value — rule skipped (passes)", specData(map[string]interface{}{"replicas": float64(5)}), ValidationRule{Field: "spec.replicas", Min: "not-a-number", Message: "invalid config"}, true},

		// ── max (ConditionLte) — inclusive ───────────────────────────────────
		{"max: value at maximum — passes", specData(map[string]interface{}{"replicas": float64(10)}), ValidationRule{Field: "spec.replicas", Max: "10", Message: "no more than 10"}, true},
		{"max: value above maximum — fails", specData(map[string]interface{}{"replicas": float64(15)}), ValidationRule{Field: "spec.replicas", Max: "10", Message: "no more than 10"}, false},
		{"max: value below maximum — passes", specData(map[string]interface{}{"replicas": float64(3)}), ValidationRule{Field: "spec.replicas", Max: "10", Message: "no more than 10"}, true},
		{"max: field missing — fails", specData(map[string]interface{}{}), ValidationRule{Field: "spec.replicas", Max: "10", Message: "no more than 10"}, false},
		{"max: non-numeric field value — fails", specData(map[string]interface{}{"replicas": "many"}), ValidationRule{Field: "spec.replicas", Max: "10", Message: "no more than 10"}, false},
		{"max: non-numeric config value — rule skipped (passes)", specData(map[string]interface{}{"replicas": float64(5)}), ValidationRule{Field: "spec.replicas", Max: "not-a-number", Message: "invalid config"}, true},

		// ── explicit gt/lt — strict, unlike min/max ──────────────────────────
		{"gt: value equal to bound — fails (strict)", specData(map[string]interface{}{"replicas": float64(1)}), ValidationRule{Field: "spec.replicas", GreaterThan: "1", Message: "must exceed 1"}, false},
		{"gt: value above bound — passes", specData(map[string]interface{}{"replicas": float64(2)}), ValidationRule{Field: "spec.replicas", GreaterThan: "1", Message: "must exceed 1"}, true},
		{"lt: value equal to bound — fails (strict)", specData(map[string]interface{}{"replicas": float64(10)}), ValidationRule{Field: "spec.replicas", LessThan: "10", Message: "must be under 10"}, false},
		{"lt: value below bound — passes", specData(map[string]interface{}{"replicas": float64(9)}), ValidationRule{Field: "spec.replicas", LessThan: "10", Message: "must be under 10"}, true},

		// ── explicit gte/lte — inclusive ──────────────────────────────────────
		{"gte: value equal to bound — passes", specData(map[string]interface{}{"replicas": float64(1)}), ValidationRule{Field: "spec.replicas", Min: "1", Message: "at least 1"}, true},
		{"gte: value below bound — fails", specData(map[string]interface{}{"replicas": float64(0)}), ValidationRule{Field: "spec.replicas", Min: "1", Message: "at least 1"}, false},
		{"lte: value equal to bound — passes", specData(map[string]interface{}{"replicas": float64(10)}), ValidationRule{Field: "spec.replicas", Max: "10", Message: "at most 10"}, true},
		{"lte: value above bound — fails", specData(map[string]interface{}{"replicas": float64(11)}), ValidationRule{Field: "spec.replicas", Max: "10", Message: "at most 10"}, false},

		// ── between / notBetween — inclusive bounds ──────────────────────────
		{"between: value inside range — passes", specData(map[string]interface{}{"replicas": float64(5)}), ValidationRule{Field: "spec.replicas", Between: "1,10", Message: "must be 1-10"}, true},
		{"between: value at lower bound — passes", specData(map[string]interface{}{"replicas": float64(1)}), ValidationRule{Field: "spec.replicas", Between: "1,10", Message: "must be 1-10"}, true},
		{"between: value at upper bound — passes", specData(map[string]interface{}{"replicas": float64(10)}), ValidationRule{Field: "spec.replicas", Between: "1,10", Message: "must be 1-10"}, true},
		{"between: value outside range — fails", specData(map[string]interface{}{"replicas": float64(11)}), ValidationRule{Field: "spec.replicas", Between: "1,10", Message: "must be 1-10"}, false},
		{"between: malformed range — rule skipped (passes)", specData(map[string]interface{}{"replicas": float64(5)}), ValidationRule{Field: "spec.replicas", Between: "not,numbers", Message: "invalid"}, true},
		{"notBetween: value outside range — passes", specData(map[string]interface{}{"replicas": float64(11)}), ValidationRule{Field: "spec.replicas", NotBetween: "1,10", Message: "must not be 1-10"}, true},
		{"notBetween: value inside range — fails", specData(map[string]interface{}{"replicas": float64(5)}), ValidationRule{Field: "spec.replicas", NotBetween: "1,10", Message: "must not be 1-10"}, false},

		// ── notIn ─────────────────────────────────────────────────────────────
		{"notIn: value not in list — passes", specData(map[string]interface{}{"workloadType": "custom"}), ValidationRule{Field: "spec.workloadType", NotIn: "app,cert,monitoring", Message: "reserved type"}, true},
		{"notIn: value in list — fails", specData(map[string]interface{}{"workloadType": "app"}), ValidationRule{Field: "spec.workloadType", NotIn: "app,cert,monitoring", Message: "reserved type"}, false},

		// ── contains / notContains ────────────────────────────────────────────
		{"notContains: substring absent — passes", specData(map[string]interface{}{"image": "myorg/app:latest"}), ValidationRule{Field: "spec.image", NotContains: "docker.io", Message: "must not use docker.io"}, true},
		{"notContains: substring present — fails", specData(map[string]interface{}{"image": "docker.io/app:latest"}), ValidationRule{Field: "spec.image", NotContains: "docker.io", Message: "must not use docker.io"}, false},

		// ── regex ─────────────────────────────────────────────────────────────
		{"regex: matches pattern — passes", specData(map[string]interface{}{"name": "app-prod-01"}), ValidationRule{Field: "spec.name", Regex: `^app-\w+-\d+$`, Message: "invalid name format"}, true},
		{"regex: does not match pattern — fails", specData(map[string]interface{}{"name": "APP"}), ValidationRule{Field: "spec.name", Regex: `^app-\w+-\d+$`, Message: "invalid name format"}, false},
		{"regex: invalid pattern — rule skipped (passes)", specData(map[string]interface{}{"name": "app"}), ValidationRule{Field: "spec.name", Regex: `(unclosed`, Message: "invalid"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := EvaluateValidationRule(tt.data, nil, tt.rule)
			if tt.wantPass && v != nil {
				t.Errorf("expected rule to pass — got violation: %+v", v)
			}
			if !tt.wantPass && v == nil {
				t.Error("expected rule to fail — got nil violation")
			}
		})
	}
}

// fakeUniquenessChecker is a test double for UniquenessChecker — the
// concrete live-list-backed implementation lives in
// pkg/runtime/reconciler/uniqueness.go and can't be imported here (this
// package can't import the reconciler, same reverse-cycle reason the
// interface exists in the first place).
type fakeUniquenessChecker struct {
	unique bool
	err    error
}

func (f *fakeUniquenessChecker) IsUnique(field, value, selfNamespace, selfName string) (bool, error) {
	return f.unique, f.err
}

func withUniqueChecker(data map[string]interface{}, checker UniquenessChecker) map[string]interface{} {
	out := make(map[string]interface{}, len(data)+1)
	for k, v := range data {
		out[k] = v
	}
	out[uniquenessCheckerKey] = checker
	return out
}

func TestEvaluateValidationRule_Unique(t *testing.T) {
	rule := ValidationRule{Field: "spec.domain", Unique: boolPtr(true), Message: "must be unique"}
	data := specData(map[string]interface{}{"domain": "a.example.com"})

	t.Run("no checker injected — always passes", func(t *testing.T) {
		v := EvaluateValidationRule(data, nil, rule)
		if v != nil {
			t.Errorf("expected pass with no checker injected — got violation: %+v", v)
		}
	})

	t.Run("no checker injected — passes even when field is missing", func(t *testing.T) {
		v := EvaluateValidationRule(specData(map[string]interface{}{}), nil, rule)
		if v != nil {
			t.Errorf("expected pass with no checker injected — got violation: %+v", v)
		}
	})

	t.Run("checker present, field missing — fails", func(t *testing.T) {
		d := withUniqueChecker(specData(map[string]interface{}{}), &fakeUniquenessChecker{unique: true})
		v := EvaluateValidationRule(d, nil, rule)
		if v == nil {
			t.Error("expected violation — field is missing")
		}
	})

	t.Run("checker reports unique — passes", func(t *testing.T) {
		d := withUniqueChecker(data, &fakeUniquenessChecker{unique: true})
		v := EvaluateValidationRule(d, nil, rule)
		if v != nil {
			t.Errorf("expected pass — got violation: %+v", v)
		}
	})

	t.Run("checker reports duplicate — fails", func(t *testing.T) {
		d := withUniqueChecker(data, &fakeUniquenessChecker{unique: false})
		v := EvaluateValidationRule(d, nil, rule)
		if v == nil {
			t.Error("expected violation — checker reported a duplicate")
		}
	})

	t.Run("checker errors — fails open (passes), rule skipped", func(t *testing.T) {
		d := withUniqueChecker(data, &fakeUniquenessChecker{err: fmt.Errorf("list failed")})
		v := EvaluateValidationRule(d, nil, rule)
		if v != nil {
			t.Errorf("expected pass on checker error — got violation: %+v", v)
		}
	})
}

func TestEvaluateValidationRule_ViolationFieldsArePopulated(t *testing.T) {
	data := specData(map[string]interface{}{"image": "docker.io/nginx:1.25"})
	rule := ValidationRule{
		Field:   "spec.image",
		Prefix:  "myorg/",
		Message: "image must be from the myorg registry",
		Action:  ValidationActionDeny,
	}

	v := EvaluateValidationRule(data, nil, rule)

	if v == nil {
		t.Fatal("expected a violation")
	}
	if v.Field != "spec.image" {
		t.Errorf("Field = %q, want %q", v.Field, "spec.image")
	}
	if v.Message != "image must be from the myorg registry" {
		t.Errorf("Message = %q, want %q", v.Message, "image must be from the myorg registry")
	}
	if v.Value != "docker.io/nginx:1.25" {
		t.Errorf("Value = %q, want %q", v.Value, "docker.io/nginx:1.25")
	}
	if v.Rule != string(ConditionPrefix) {
		t.Errorf("Rule = %q, want %q", v.Rule, string(ConditionPrefix))
	}
}

func TestEvaluateValidationRule_LinkOverridesDisplayField(t *testing.T) {
	data := specData(map[string]interface{}{"team": ""})
	rule := ValidationRule{
		Field:   `{{ getLabel . "team" }}`,
		Link:    "team",
		Exists:  boolPtr(true),
		Message: "team is required",
		Action:  ValidationActionDeny,
	}

	v := EvaluateValidationRule(data, nil, rule)

	if v == nil {
		t.Fatal("expected a violation — field is empty")
	}
	if v.Field != "team" {
		t.Errorf("Field = %q, want %q (Link, not the raw template expression)", v.Field, "team")
	}
}

func TestEvaluateValidationRule_NoLinkUsesFieldAsDisplayField(t *testing.T) {
	data := specData(map[string]interface{}{})
	rule := ValidationRule{
		Field:   `{{ getLabel . "team" }}`,
		Exists:  boolPtr(true),
		Message: "team is required",
		Action:  ValidationActionDeny,
	}

	v := EvaluateValidationRule(data, nil, rule)

	if v == nil {
		t.Fatal("expected a violation — field is empty")
	}
	if v.Field != `{{ getLabel . "team" }}` {
		t.Errorf("Field = %q, want the raw expression unchanged — no Link was set", v.Field)
	}
}
