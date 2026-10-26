// Tests for EvaluateConditions, EvaluateOneCond, NavigateDotPath, NavigateRawPath,
// ResolveConditionOp (when.go).
package types_test

import (
	"errors"
	"testing"
	"time"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

// ── NavigateDotPath ───────────────────────────────────────────────────────────

func TestNavigateDotPath_EmptyPath(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	assert.Equal(t, "", types.NavigateDotPath(data, ""))
}

func TestNavigateDotPath_TopLevel(t *testing.T) {
	data := map[string]interface{}{"phase": "Running"}
	assert.Equal(t, "Running", types.NavigateDotPath(data, "phase"))
}

func TestNavigateDotPath_Nested(t *testing.T) {
	data := map[string]interface{}{
		"status": map[string]interface{}{"phase": "Ready"},
	}
	assert.Equal(t, "Ready", types.NavigateDotPath(data, "status.phase"))
}

func TestNavigateDotPath_DeepNested(t *testing.T) {
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "deep",
			},
		},
	}
	assert.Equal(t, "deep", types.NavigateDotPath(data, "a.b.c"))
}

func TestNavigateDotPath_MissingKey(t *testing.T) {
	data := map[string]interface{}{"phase": "Running"}
	assert.Equal(t, "", types.NavigateDotPath(data, "status.phase"))
}

func TestNavigateDotPath_NonMapIntermediate(t *testing.T) {
	data := map[string]interface{}{"status": "running"}
	assert.Equal(t, "", types.NavigateDotPath(data, "status.phase"))
}

func TestNavigateDotPath_IntValue(t *testing.T) {
	data := map[string]interface{}{"replicas": 3}
	assert.Equal(t, "3", types.NavigateDotPath(data, "replicas"))
}

func TestNavigateDotPath_NilValue(t *testing.T) {
	data := map[string]interface{}{"key": nil}
	assert.Equal(t, "", types.NavigateDotPath(data, "key"))
}

// ── NavigateRawPath ───────────────────────────────────────────────────────────

func TestNavigateRawPath_EmptyPath(t *testing.T) {
	data := map[string]interface{}{"key": "val"}
	assert.Nil(t, types.NavigateRawPath(data, ""))
}

func TestNavigateRawPath_TopLevelString(t *testing.T) {
	data := map[string]interface{}{"phase": "Running"}
	assert.Equal(t, "Running", types.NavigateRawPath(data, "phase"))
}

func TestNavigateRawPath_TopLevelMap(t *testing.T) {
	inner := map[string]interface{}{"x": 1}
	data := map[string]interface{}{"spec": inner}
	assert.Equal(t, inner, types.NavigateRawPath(data, "spec"))
}

func TestNavigateRawPath_TopLevelSlice(t *testing.T) {
	slice := []interface{}{"a", "b"}
	data := map[string]interface{}{"items": slice}
	assert.Equal(t, slice, types.NavigateRawPath(data, "items"))
}

func TestNavigateRawPath_MissingKey(t *testing.T) {
	data := map[string]interface{}{}
	assert.Nil(t, types.NavigateRawPath(data, "missing"))
}

func TestNavigateRawPath_BoolValue(t *testing.T) {
	data := map[string]interface{}{"enabled": true}
	assert.Equal(t, true, types.NavigateRawPath(data, "enabled"))
}

// ── ResolveConditionOp ────────────────────────────────────────────────────────

func TestResolveConditionOp_Equals(t *testing.T) {
	c := types.Condition{Equals: "Ready"}
	op, val := types.ResolveConditionOp(c)
	assert.Equal(t, types.ConditionEquals, op)
	assert.Equal(t, "Ready", val)
}

func TestResolveConditionOp_NotEquals(t *testing.T) {
	c := types.Condition{NotEquals: "Failed"}
	op, val := types.ResolveConditionOp(c)
	assert.Equal(t, types.ConditionNotEquals, op)
	assert.Equal(t, "Failed", val)
}

func TestResolveConditionOp_Prefix(t *testing.T) {
	c := types.Condition{Prefix: "prod-"}
	op, val := types.ResolveConditionOp(c)
	assert.Equal(t, types.ConditionPrefix, op)
	assert.Equal(t, "prod-", val)
}

func TestResolveConditionOp_Suffix(t *testing.T) {
	c := types.Condition{Suffix: "-v2"}
	op, val := types.ResolveConditionOp(c)
	assert.Equal(t, types.ConditionSuffix, op)
	assert.Equal(t, "-v2", val)
}

func TestResolveConditionOp_Contains(t *testing.T) {
	c := types.Condition{Contains: "error"}
	op, val := types.ResolveConditionOp(c)
	assert.Equal(t, types.ConditionContains, op)
	assert.Equal(t, "error", val)
}

func TestResolveConditionOp_GreaterThan(t *testing.T) {
	c := types.Condition{GreaterThan: "10"}
	op, val := types.ResolveConditionOp(c)
	assert.Equal(t, types.ConditionGt, op)
	assert.Equal(t, "10", val)
}

func TestResolveConditionOp_LessThan(t *testing.T) {
	c := types.Condition{LessThan: "5"}
	op, val := types.ResolveConditionOp(c)
	assert.Equal(t, types.ConditionLt, op)
	assert.Equal(t, "5", val)
}

func TestResolveConditionOp_ExplicitOperator(t *testing.T) {
	c := types.Condition{In: "a,b,c"}
	op, val := types.ResolveConditionOp(c)
	assert.Equal(t, types.ConditionIn, op)
	assert.Equal(t, "a,b,c", val)
}

func TestResolveConditionOp_Default(t *testing.T) {
	c := types.Condition{Field: "status.phase"}
	op, val := types.ResolveConditionOp(c)
	assert.Equal(t, types.ConditionExists, op)
	assert.Equal(t, "", val)
}

// ── EvaluateOneCond — field operators ────────────────────────────────────────

func data(kv ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(kv)/2)
	for i := 0; i < len(kv)-1; i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

func TestEvaluateOneCond_Equals_Match(t *testing.T) {
	d := data("phase", "Running")
	c := types.Condition{Field: "phase", Equals: "Running"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Equals_NoMatch(t *testing.T) {
	d := data("phase", "Pending")
	c := types.Condition{Field: "phase", Equals: "Running"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_NotEquals(t *testing.T) {
	d := data("phase", "Pending")
	c := types.Condition{Field: "phase", NotEquals: "Running"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Contains(t *testing.T) {
	d := data("message", "connection refused")
	c := types.Condition{Field: "message", Contains: "refused"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Contains_NoMatch(t *testing.T) {
	d := data("message", "all good")
	c := types.Condition{Field: "message", Contains: "error"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Prefix(t *testing.T) {
	d := data("name", "prod-app")
	c := types.Condition{Field: "name", Prefix: "prod-"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Prefix_NoMatch(t *testing.T) {
	d := data("name", "dev-app")
	c := types.Condition{Field: "name", Prefix: "prod-"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Suffix(t *testing.T) {
	d := data("name", "app-v2")
	c := types.Condition{Field: "name", Suffix: "-v2"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Exists_Present(t *testing.T) {
	d := data("phase", "Running")
	c := types.Condition{Field: "phase", Exists: ptr.To(true)}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Exists_Missing(t *testing.T) {
	d := map[string]interface{}{}
	c := types.Condition{Field: "phase", Exists: ptr.To(true)}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_NotExists_Missing(t *testing.T) {
	d := map[string]interface{}{}
	c := types.Condition{Field: "phase", NotExists: ptr.To(true)}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_NotExists_Present(t *testing.T) {
	d := data("phase", "Running")
	c := types.Condition{Field: "phase", NotExists: ptr.To(true)}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Gt_Pass(t *testing.T) {
	d := data("replicas", "5")
	c := types.Condition{Field: "replicas", GreaterThan: "3"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Gt_Fail(t *testing.T) {
	d := data("replicas", "2")
	c := types.Condition{Field: "replicas", GreaterThan: "3"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Gt_AbsentFieldTreatedAsZero(t *testing.T) {
	d := map[string]interface{}{}
	c := types.Condition{Field: "count", GreaterThan: "0"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Lt_Pass(t *testing.T) {
	d := data("cpu", "50")
	c := types.Condition{Field: "cpu", LessThan: "80"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_In_Match(t *testing.T) {
	d := data("env", "prod")
	c := types.Condition{Field: "env", In: "dev,staging,prod"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_In_NoMatch(t *testing.T) {
	d := data("env", "test")
	c := types.Condition{Field: "env", In: "dev,staging,prod"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_NotIn_Match(t *testing.T) {
	d := data("env", "canary")
	c := types.Condition{Field: "env", NotIn: "dev,staging,prod"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_NotIn_NoMatch(t *testing.T) {
	d := data("env", "prod")
	c := types.Condition{Field: "env", NotIn: "dev,staging,prod"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Gte_EqualBound(t *testing.T) {
	d := data("replicas", "3")
	c := types.Condition{Field: "replicas", Min: "3"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Gte_BelowBound(t *testing.T) {
	d := data("replicas", "2")
	c := types.Condition{Field: "replicas", Min: "3"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Lte_EqualBound(t *testing.T) {
	d := data("cpu", "80")
	c := types.Condition{Field: "cpu", Max: "80"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Lte_AboveBound(t *testing.T) {
	d := data("cpu", "81")
	c := types.Condition{Field: "cpu", Max: "80"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestResolveConditionOp_Min(t *testing.T) {
	op, val := types.ResolveConditionOp(types.Condition{Min: "1"})
	assert.Equal(t, types.ConditionGte, op)
	assert.Equal(t, "1", val)
}

func TestResolveConditionOp_Max(t *testing.T) {
	op, val := types.ResolveConditionOp(types.Condition{Max: "10"})
	assert.Equal(t, types.ConditionLte, op)
	assert.Equal(t, "10", val)
}

func TestEvaluateOneCond_Min_EqualBound(t *testing.T) {
	d := data("replicas", "1")
	c := types.Condition{Field: "replicas", Min: "1"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Min_BelowBound(t *testing.T) {
	d := data("replicas", "0")
	c := types.Condition{Field: "replicas", Min: "1"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Max_EqualBound(t *testing.T) {
	d := data("cpu", "80")
	c := types.Condition{Field: "cpu", Max: "80"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Max_AboveBound(t *testing.T) {
	d := data("cpu", "81")
	c := types.Condition{Field: "cpu", Max: "80"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Between_Inside(t *testing.T) {
	d := data("replicas", "5")
	c := types.Condition{Field: "replicas", Between: "1,10"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Between_AtBounds(t *testing.T) {
	c := types.Condition{Field: "replicas", Between: "1,10"}
	assert.True(t, types.EvaluateOneCond(data("replicas", "1"), c, nil))
	assert.True(t, types.EvaluateOneCond(data("replicas", "10"), c, nil))
}

func TestEvaluateOneCond_Between_Outside(t *testing.T) {
	d := data("replicas", "11")
	c := types.Condition{Field: "replicas", Between: "1,10"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Between_MalformedRange(t *testing.T) {
	d := data("replicas", "5")
	c := types.Condition{Field: "replicas", Between: "not,numbers"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_NotBetween_Outside(t *testing.T) {
	d := data("replicas", "11")
	c := types.Condition{Field: "replicas", NotBetween: "1,10"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_NotBetween_Inside(t *testing.T) {
	d := data("replicas", "5")
	c := types.Condition{Field: "replicas", NotBetween: "1,10"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_NotContains_Match(t *testing.T) {
	d := data("image", "myorg/app:latest")
	c := types.Condition{Field: "image", NotContains: "docker.io"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_NotContains_NoMatch(t *testing.T) {
	d := data("image", "docker.io/app:latest")
	c := types.Condition{Field: "image", NotContains: "docker.io"}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Regex_Match(t *testing.T) {
	d := data("name", "app-prod-01")
	c := types.Condition{Field: "name", Regex: `^app-\w+-\d+$`}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Regex_NoMatch(t *testing.T) {
	d := data("name", "APP")
	c := types.Condition{Field: "name", Regex: `^app-\w+-\d+$`}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Regex_InvalidPattern(t *testing.T) {
	d := data("name", "app")
	c := types.Condition{Field: "name", Regex: `(unclosed`}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_TypeOf_Map(t *testing.T) {
	d := map[string]interface{}{
		"spec": map[string]interface{}{"key": "val"},
	}
	c := types.Condition{Field: "spec", TypeOf: "map"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_TypeOf_List(t *testing.T) {
	d := map[string]interface{}{
		"items": []interface{}{"a", "b"},
	}
	c := types.Condition{Field: "items", TypeOf: "slice"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_TypeOf_String(t *testing.T) {
	d := data("phase", "Running")
	c := types.Condition{Field: "phase", TypeOf: "string"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_TypeOf_Bool(t *testing.T) {
	d := map[string]interface{}{"enabled": true}
	c := types.Condition{Field: "enabled", TypeOf: "bool"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_TypeOf_Number(t *testing.T) {
	d := map[string]interface{}{"replicas": 3}
	c := types.Condition{Field: "replicas", TypeOf: "number"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_TypeOf_Null(t *testing.T) {
	d := map[string]interface{}{"field": nil}
	c := types.Condition{Field: "field", TypeOf: "null"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_TypeOf_Explicit(t *testing.T) {
	d := data("phase", "Running")
	c := types.Condition{Field: "phase", TypeOf: "string"}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Unique_AlwaysTrue(t *testing.T) {
	d := data("name", "foo")
	c := types.Condition{Field: "name", Unique: ptr.To(true)}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

// ── EvaluateConditions — allOf / or ──────────────────────────────────────────────

func TestEvaluateConditions_EmptyBothPasses(t *testing.T) {
	assert.True(t, types.EvaluateConditions(nil, nil, nil, nil))
}

func TestEvaluateConditions_AllOfAllPass(t *testing.T) {
	d := data("phase", "Running", "env", "prod")
	allOf := []types.Condition{
		{Field: "phase", Equals: "Running"},
		{Field: "env", Equals: "prod"},
	}
	assert.True(t, types.EvaluateConditions(d, allOf, nil, nil))
}

func TestEvaluateConditions_AllOfOneFails(t *testing.T) {
	d := data("phase", "Pending", "env", "prod")
	allOf := []types.Condition{
		{Field: "phase", Equals: "Running"},
		{Field: "env", Equals: "prod"},
	}
	assert.False(t, types.EvaluateConditions(d, allOf, nil, nil))
}

func TestEvaluateConditions_OrOneMatches(t *testing.T) {
	d := data("phase", "Failed")
	or := []types.Condition{
		{Field: "phase", Equals: "Failed"},
		{Field: "phase", Equals: "Succeeded"},
	}
	assert.True(t, types.EvaluateConditions(d, nil, or, nil))
}

func TestEvaluateConditions_OrNoneMatch(t *testing.T) {
	d := data("phase", "Running")
	or := []types.Condition{
		{Field: "phase", Equals: "Failed"},
		{Field: "phase", Equals: "Succeeded"},
	}
	assert.False(t, types.EvaluateConditions(d, nil, or, nil))
}

func TestEvaluateConditions_BothMustPass(t *testing.T) {
	d := data("env", "prod", "phase", "Failed")
	allOf := []types.Condition{{Field: "env", Equals: "prod"}}
	or := []types.Condition{
		{Field: "phase", Equals: "Failed"},
		{Field: "phase", Equals: "Succeeded"},
	}
	assert.True(t, types.EvaluateConditions(d, allOf, or, nil))
}

func TestEvaluateConditions_AllOfPassOrFails(t *testing.T) {
	d := data("env", "prod", "phase", "Running")
	allOf := []types.Condition{{Field: "env", Equals: "prod"}}
	or := []types.Condition{
		{Field: "phase", Equals: "Failed"},
		{Field: "phase", Equals: "Succeeded"},
	}
	assert.False(t, types.EvaluateConditions(d, allOf, or, nil))
}

// ── EvaluateOneCond — cron window injection via _cronWindows ─────────────────

func TestEvaluateOneCond_CronWindowInjected_True(t *testing.T) {
	d := map[string]interface{}{
		"_cronWindows": map[string]interface{}{
			"0 9 * * 1-5": "true",
		},
	}
	c := types.Condition{Cron: "0 9 * * 1-5", Duration: types.Duration{Duration: time.Hour}}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_CronWindowInjected_False(t *testing.T) {
	d := map[string]interface{}{
		"_cronWindows": map[string]interface{}{
			"0 9 * * 1-5": "false",
		},
	}
	c := types.Condition{Cron: "0 9 * * 1-5", Duration: types.Duration{Duration: time.Hour}}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

// ── EvaluateOneCond — unique operator via _uniquenessChecker injection ───────
//
// The concrete live-list-backed checker lives in
// pkg/runtime/reconciler/uniqueness.go; here we inject a fake under the
// same "_uniquenessChecker" key template.Resolver.WithUniquenessChecker
// uses, matching the _cronWindows injection convention above.

type fakeUniqueChecker struct {
	unique bool
	err    error
}

func (f *fakeUniqueChecker) IsUnique(field, value, selfNamespace, selfName string) (bool, error) {
	return f.unique, f.err
}

func TestEvaluateOneCond_Unique_NoCheckerInjected_AlwaysPasses(t *testing.T) {
	d := data("domain", "a.example.com")
	c := types.Condition{Field: "domain", Unique: ptr.To(true)}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Unique_CheckerReportsUnique(t *testing.T) {
	d := data("domain", "a.example.com")
	d["_uniquenessChecker"] = &fakeUniqueChecker{unique: true}
	c := types.Condition{Field: "domain", Unique: ptr.To(true)}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Unique_CheckerReportsDuplicate(t *testing.T) {
	d := data("domain", "shared.example.com")
	d["_uniquenessChecker"] = &fakeUniqueChecker{unique: false}
	c := types.Condition{Field: "domain", Unique: ptr.To(true)}
	assert.False(t, types.EvaluateOneCond(d, c, nil))
}

func TestEvaluateOneCond_Unique_CheckerErrors_FailsOpen(t *testing.T) {
	d := data("domain", "a.example.com")
	d["_uniquenessChecker"] = &fakeUniqueChecker{err: errors.New("list failed")}
	c := types.Condition{Field: "domain", Unique: ptr.To(true)}
	assert.True(t, types.EvaluateOneCond(d, c, nil))
}

// ── EvaluateOneCond — time window (via Condition.Time) ───────────────────────

func TestEvaluateOneCond_TimeWindow_WithinRange(t *testing.T) {
	// Use a window that is definitely open right now: 00:00–23:59
	c := types.Condition{
		Time: &types.TimeWindow{After: "00:00", Before: "23:59"},
	}
	assert.True(t, types.EvaluateOneCond(nil, c, nil))
}

func TestEvaluateOneCond_TimeWindow_OnlyAfter_Pass(t *testing.T) {
	// After 00:00 — always true
	c := types.Condition{
		Time: &types.TimeWindow{After: "00:00"},
	}
	assert.True(t, types.EvaluateOneCond(nil, c, nil))
}

func TestEvaluateOneCond_TimeWindow_OnlyBefore_Pass(t *testing.T) {
	// Before 23:59 — always true
	c := types.Condition{
		Time: &types.TimeWindow{Before: "23:59"},
	}
	assert.True(t, types.EvaluateOneCond(nil, c, nil))
}

func TestEvaluateOneCond_TimeWindow_InvalidAfter(t *testing.T) {
	c := types.Condition{
		Time: &types.TimeWindow{After: "not-a-time"},
	}
	assert.False(t, types.EvaluateOneCond(nil, c, nil))
}

func TestEvaluateOneCond_TimeWindow_InvalidBefore(t *testing.T) {
	c := types.Condition{
		Time: &types.TimeWindow{Before: "not-a-time"},
	}
	assert.False(t, types.EvaluateOneCond(nil, c, nil))
}

// ── EvaluateOneCond — day of week (via Condition.DayOfWeek) ──────────────────

func TestEvaluateOneCond_DayOfWeek_InMatchesAllDays(t *testing.T) {
	// In: all 7 days — always true regardless of current weekday
	c := types.Condition{
		DayOfWeek: &types.DayOfWeekCondition{
			In: []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
		},
	}
	assert.True(t, types.EvaluateOneCond(nil, c, nil))
}

func TestEvaluateOneCond_DayOfWeek_NotInEmpty_ReturnsFalse(t *testing.T) {
	// Neither In nor NotIn set — evalDayOfWeek returns false
	c := types.Condition{
		DayOfWeek: &types.DayOfWeekCondition{},
	}
	assert.False(t, types.EvaluateOneCond(nil, c, nil))
}

func TestEvaluateOneCond_DayOfWeek_NotInAllDays_ReturnsFalse(t *testing.T) {
	// Excluding all days — always false
	c := types.Condition{
		DayOfWeek: &types.DayOfWeekCondition{
			NotIn: []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
		},
	}
	assert.False(t, types.EvaluateOneCond(nil, c, nil))
}

func TestEvaluateOneCond_DayOfWeek_NotInNoMatch_ReturnsTrue(t *testing.T) {
	// NotIn: ["Funday"] — "Funday" never matches any real weekday, so always true
	c := types.Condition{
		DayOfWeek: &types.DayOfWeekCondition{
			NotIn: []string{"Funday"},
		},
	}
	assert.True(t, types.EvaluateOneCond(nil, c, nil))
}

func TestEvaluateOneCond_DayOfWeek_WeekdayTrue_OnWeekday(t *testing.T) {
	b := true
	c := types.Condition{DayOfWeek: &types.DayOfWeekCondition{Weekday: &b}}
	// Monday is a weekday — must pass
	got := types.EvalDayOfWeekAt(c.DayOfWeek, mustParseTime("2026-07-13T12:00:00Z")) // Monday
	assert.True(t, got)
}

func TestEvaluateOneCond_DayOfWeek_WeekdayTrue_OnWeekend(t *testing.T) {
	b := true
	c := types.Condition{DayOfWeek: &types.DayOfWeekCondition{Weekday: &b}}
	// Saturday is not a weekday — must fail
	got := types.EvalDayOfWeekAt(c.DayOfWeek, mustParseTime("2026-07-12T12:00:00Z")) // Saturday
	assert.False(t, got)
}

func TestEvaluateOneCond_DayOfWeek_WeekendTrue_OnWeekend(t *testing.T) {
	b := true
	c := types.Condition{DayOfWeek: &types.DayOfWeekCondition{Weekend: &b}}
	got := types.EvalDayOfWeekAt(c.DayOfWeek, mustParseTime("2026-07-12T12:00:00Z")) // Saturday
	assert.True(t, got)
}

func TestEvaluateOneCond_DayOfWeek_WeekendTrue_OnWeekday(t *testing.T) {
	b := true
	c := types.Condition{DayOfWeek: &types.DayOfWeekCondition{Weekend: &b}}
	got := types.EvalDayOfWeekAt(c.DayOfWeek, mustParseTime("2026-07-14T12:00:00Z")) // Monday
	assert.False(t, got)
}

func TestEvaluateOneCond_DayOfWeek_WeekdayAndWeekend_MutuallyExclusive(t *testing.T) {
	b := true
	// weekday: true on a weekday, weekend: true on a weekend — never both true simultaneously
	mon := mustParseTime("2026-07-13T12:00:00Z")
	sat := mustParseTime("2026-07-12T12:00:00Z")
	weekdayCond := &types.DayOfWeekCondition{Weekday: &b}
	weekendCond := &types.DayOfWeekCondition{Weekend: &b}
	assert.True(t, types.EvalDayOfWeekAt(weekdayCond, mon))
	assert.False(t, types.EvalDayOfWeekAt(weekendCond, mon))
	assert.False(t, types.EvalDayOfWeekAt(weekdayCond, sat))
	assert.True(t, types.EvalDayOfWeekAt(weekendCond, sat))
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// ── EvaluateOneCond — cron (stateless fallback) ───────────────────────────────

func TestEvaluateOneCond_CronStateless_InvalidExpr(t *testing.T) {
	// Invalid cron expression → false
	c := types.Condition{Cron: "not-a-cron", Duration: types.Duration{Duration: time.Hour}}
	assert.False(t, types.EvaluateOneCond(nil, c, nil))
}
