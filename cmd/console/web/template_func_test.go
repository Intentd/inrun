package web

import "testing"

func templateFunc[T any](t *testing.T, name string) T {
	t.Helper()
	fn, ok := templateFuncs[name]
	if !ok {
		t.Fatalf("templateFuncs[%q] not registered", name)
	}
	f, ok := fn.(T)
	if !ok {
		t.Fatalf("templateFuncs[%q] has an unexpected type", name)
	}
	return f
}

func TestTemplateFunc_Arithmetic(t *testing.T) {
	if got := templateFunc[func(int, int) int](t, "mul")(3, 4); got != 12 {
		t.Errorf("mul(3,4) = %d, want 12", got)
	}
	if got := templateFunc[func(int, int) int](t, "add")(3, 4); got != 7 {
		t.Errorf("add(3,4) = %d, want 7", got)
	}
	if got := templateFunc[func(int, int) int](t, "sub")(7, 4); got != 3 {
		t.Errorf("sub(7,4) = %d, want 3", got)
	}
	if got := templateFunc[func(int, int) int](t, "min")(7, 4); got != 4 {
		t.Errorf("min(7,4) = %d, want 4", got)
	}
	div := templateFunc[func(int, int) int](t, "div")
	if got := div(10, 2); got != 5 {
		t.Errorf("div(10,2) = %d, want 5", got)
	}
	if got := div(10, 0); got != 0 {
		t.Errorf("div(10,0) = %d, want 0 (guarded, not a panic)", got)
	}
}

func TestTemplateFunc_FormatNumber(t *testing.T) {
	formatNumber := templateFunc[func(int) string](t, "formatNumber")
	cases := []struct {
		in   int
		want string
	}{
		{5, "5"},
		{9_999, "9999"},
		{10_000, "10.0K"},
		{999_999, "1000.0K"},
		{1_000_000, "1.0M"},
		{999_999_999, "1000.0M"},
		{1_000_000_000, "1.0B"},
	}
	for _, c := range cases {
		if got := formatNumber(c.in); got != c.want {
			t.Errorf("formatNumber(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTemplateFunc_FormatTime(t *testing.T) {
	formatTime := templateFunc[func(string) string](t, "formatTime")
	if got := formatTime(""); got != "never" {
		t.Errorf("formatTime(\"\") = %q, want %q", got, "never")
	}
	if got := formatTime("not-a-time"); got != "not-a-time" {
		t.Errorf("formatTime(garbage) = %q, want the input echoed back", got)
	}
	if got := formatTime("2026-07-27T12:00:00Z"); got != "2026-07-27 12:00:00" {
		t.Errorf("formatTime(rfc3339) = %q, want %q", got, "2026-07-27 12:00:00")
	}
}

func TestTemplateFunc_Join(t *testing.T) {
	join := templateFunc[func(string, []string) string](t, "join")
	if got := join(", ", []string{"a", "b", "c"}); got != "a, b, c" {
		t.Errorf("join = %q, want %q", got, "a, b, c")
	}
	if got := join(", ", nil); got != "" {
		t.Errorf("join(nil) = %q, want empty string", got)
	}
}

func TestTemplateFunc_Contains(t *testing.T) {
	contains := templateFunc[func([]string, string) bool](t, "contains")
	if !contains([]string{"a", "b"}, "b") {
		t.Error("contains([a,b], b) = false, want true")
	}
	if contains([]string{"a", "b"}, "c") {
		t.Error("contains([a,b], c) = true, want false")
	}
	if contains(nil, "a") {
		t.Error("contains(nil, a) = true, want false")
	}
}

func TestTemplateFunc_Default(t *testing.T) {
	def := templateFunc[func(interface{}, interface{}) interface{}](t, "default")

	cases := []struct {
		name string
		val  interface{}
		want interface{}
	}{
		{"nil falls back", nil, "fallback"},
		{"empty string falls back", "", "fallback"},
		{"zero int falls back", 0, "fallback"},
		{"empty slice falls back", []interface{}{}, "fallback"},
		{"empty map falls back", map[string]interface{}{}, "fallback"},
		{"non-empty string passes through", "value", "value"},
		{"non-zero int passes through", 5, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := def("fallback", c.val); got != c.want {
				t.Errorf("default(fallback, %v) = %v, want %v", c.val, got, c.want)
			}
		})
	}
}

func TestTemplateFunc_MapGet(t *testing.T) {
	mapGet := templateFunc[func(interface{}, string) interface{}](t, "mapGet")
	m := map[string]interface{}{"foo": "bar"}
	if got := mapGet(m, "foo"); got != "bar" {
		t.Errorf("mapGet(m, foo) = %v, want bar", got)
	}
	if got := mapGet(m, "missing"); got != nil {
		t.Errorf("mapGet(m, missing) = %v, want nil", got)
	}
	if got := mapGet(nil, "foo"); got != nil {
		t.Errorf("mapGet(nil, foo) = %v, want nil", got)
	}
	if got := mapGet("not a map", "foo"); got != nil {
		t.Errorf("mapGet(non-map, foo) = %v, want nil", got)
	}
}

func TestTemplateFunc_AsCoercions(t *testing.T) {
	asBool := templateFunc[func(interface{}) bool](t, "asBool")
	if !asBool(true) {
		t.Error("asBool(true) = false")
	}
	if asBool("true") {
		t.Error("asBool(\"true\") = true, want false (not a bool value)")
	}

	asString := templateFunc[func(interface{}) string](t, "asString")
	if got := asString("hi"); got != "hi" {
		t.Errorf("asString(hi) = %q", got)
	}
	if got := asString(5); got != "" {
		t.Errorf("asString(5) = %q, want empty", got)
	}

	asSlice := templateFunc[func(interface{}) []interface{}](t, "asSlice")
	if got := asSlice([]interface{}{1, 2}); len(got) != 2 {
		t.Errorf("asSlice = %v, want len 2", got)
	}
	if got := asSlice("nope"); got != nil {
		t.Errorf("asSlice(non-slice) = %v, want nil", got)
	}

	asMap := templateFunc[func(interface{}) map[string]interface{}](t, "asMap")
	if got := asMap(map[string]interface{}{"a": 1}); len(got) != 1 {
		t.Errorf("asMap = %v, want len 1", got)
	}
	if got := asMap("nope"); got != nil {
		t.Errorf("asMap(non-map) = %v, want nil", got)
	}
}

func TestTemplateFunc_HasPrefix(t *testing.T) {
	hasPrefix := templateFunc[func(string, string) bool](t, "hasPrefix")
	if !hasPrefix("Running:pending", "Running") {
		t.Error("hasPrefix(Running:pending, Running) = false")
	}
	if hasPrefix("Pending", "Running") {
		t.Error("hasPrefix(Pending, Running) = true")
	}
}

func TestTemplateFunc_Truncate(t *testing.T) {
	truncate := templateFunc[func(string, int) string](t, "truncate")
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short, 10) = %q, want unchanged", got)
	}
	if got := truncate("this is a long string", 10); got != "this is..." {
		t.Errorf("truncate(long, 10) = %q, want %q", got, "this is...")
	}
}
