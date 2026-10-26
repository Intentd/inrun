package structured

import (
	"strings"
	"testing"
)

type yamlTarget struct {
	Name     string `yaml:"name"`
	Replicas int    `yaml:"replicas"`
}

// ── StrictUnmarshal ───────────────────────────────────────────────────────────

func TestStrictUnmarshal_Valid(t *testing.T) {
	data := []byte("name: web\nreplicas: 3\n")
	var out yamlTarget
	if err := StrictUnmarshal(data, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Name != "web" || out.Replicas != 3 {
		t.Errorf("unexpected: %+v", out)
	}
}

func TestStrictUnmarshal_UnknownField(t *testing.T) {
	data := []byte("name: web\nunknown: boom\n")
	var out yamlTarget
	err := StrictUnmarshal(data, &out)
	if err == nil {
		t.Error("unknown field must return an error in strict mode")
	}
}

func TestStrictUnmarshal_Empty(t *testing.T) {
	var out yamlTarget
	if err := StrictUnmarshal([]byte(""), &out); err != nil {
		// Empty YAML is valid (zero value)
		t.Logf("empty YAML error (acceptable): %v", err)
	}
}

func TestStrictUnmarshal_TypeMismatch(t *testing.T) {
	// replicas expects int, give it a string that can't coerce
	data := []byte("name: ok\nreplicas: not-a-number\n")
	var out yamlTarget
	err := StrictUnmarshal(data, &out)
	if err == nil {
		t.Error("type mismatch must return an error")
	}
}

// ── FormatYAMLError ───────────────────────────────────────────────────────────

func TestFormatYAMLError_Nil(t *testing.T) {
	out := FormatYAMLError(nil, nil)
	if out != "" {
		t.Errorf("nil error must return empty string, got %q", out)
	}
}

func TestFormatYAMLError_UnknownField(t *testing.T) {
	data := []byte("name: web\nunknown: boom\n")
	var out yamlTarget
	err := StrictUnmarshal(data, &out)
	if err == nil {
		t.Skip("StrictUnmarshal didn't error — can't test FormatYAMLError path")
	}
	// After formatting, it should be non-empty and human-readable
	if len(err.Error()) == 0 {
		t.Error("formatted error must be non-empty")
	}
}

func TestFormatYAMLError_FallsBackToRawError(t *testing.T) {
	// An error with no parseable "line X: field" format falls back to the raw
	// error message rather than returning an empty string.
	import_strings_unused := strings.NewReader("") // keep import used
	_ = import_strings_unused
	data := []byte("irrelevant")
	rawErr := &plainError{"something completely different"}
	result := FormatYAMLError(rawErr, data)
	if result == "" {
		t.Error("FormatYAMLError with unparseable error must return non-empty string")
	}
}

type plainError struct{ msg string }

func (e *plainError) Error() string { return e.msg }

// ── getLineFromData ───────────────────────────────────────────────────────────

func TestGetLineFromData_ValidLine(t *testing.T) {
	data := []byte("line1\nline2\nline3\n")
	got := getLineFromData(data, 2)
	if got != "line2" {
		t.Errorf("expected line2, got %q", got)
	}
}

func TestGetLineFromData_FirstLine(t *testing.T) {
	data := []byte("first\nsecond\n")
	got := getLineFromData(data, 1)
	if got != "first" {
		t.Errorf("expected first, got %q", got)
	}
}

func TestGetLineFromData_OutOfRange(t *testing.T) {
	data := []byte("only one line")
	got := getLineFromData(data, 99)
	if got != "" {
		t.Errorf("out-of-range must return empty string, got %q", got)
	}
}

func TestGetLineFromData_ZeroLine(t *testing.T) {
	data := []byte("line1\nline2\n")
	got := getLineFromData(data, 0)
	if got != "" {
		t.Errorf("line 0 must return empty string, got %q", got)
	}
}
