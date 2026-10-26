package expect

import (
	"errors"
	"testing"
	"time"
)

func TestResult_AllPassed_Empty(t *testing.T) {
	r := &Result{}
	if !r.AllPassed() {
		t.Error("AllPassed() on empty result should return true")
	}
}

func TestResult_AllPassed_AllPass(t *testing.T) {
	r := &Result{
		Cases: []CaseResult{
			{Passed: true},
			{Passed: true},
		},
	}
	if !r.AllPassed() {
		t.Error("expected AllPassed() = true")
	}
}

func TestResult_AllPassed_OneFail(t *testing.T) {
	r := &Result{
		Cases: []CaseResult{
			{Passed: true},
			{Passed: false, Err: errors.New("timeout")},
		},
	}
	if r.AllPassed() {
		t.Error("expected AllPassed() = false when one case fails")
	}
}

func TestResult_Passed_Count(t *testing.T) {
	r := &Result{
		Cases: []CaseResult{
			{Passed: true},
			{Passed: false},
			{Passed: true},
		},
	}
	if got := r.Passed(); got != 2 {
		t.Errorf("Passed() = %d, want 2", got)
	}
}

func TestResult_Total(t *testing.T) {
	r := &Result{
		Cases: []CaseResult{{}, {}, {}},
	}
	if got := r.Total(); got != 3 {
		t.Errorf("Total() = %d, want 3", got)
	}
}

func TestResult_Duration(t *testing.T) {
	r := &Result{Elapsed: 45*time.Second + 300*time.Millisecond}
	got := r.Duration()
	// Rounds to nearest second → "45s"
	if got != "45s" {
		t.Errorf("Duration() = %q, want %q", got, "45s")
	}
}

func TestResult_Duration_RoundsUp(t *testing.T) {
	r := &Result{Elapsed: 45*time.Second + 600*time.Millisecond}
	if got := r.Duration(); got != "46s" {
		t.Errorf("Duration() = %q, want \"46s\"", got)
	}
}

func TestResult_Summary(t *testing.T) {
	r := &Result{
		Cases: []CaseResult{
			{Passed: true},
			{Passed: true},
			{Passed: false},
		},
		Elapsed: 12 * time.Second,
	}
	got := r.Summary()
	want := "2 of 3 passed (12s)"
	if got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

func TestResult_Summary_AllPassed(t *testing.T) {
	r := &Result{
		Cases:   []CaseResult{{Passed: true}},
		Elapsed: 3 * time.Second,
	}
	got := r.Summary()
	want := "1 of 1 passed (3s)"
	if got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

func TestResult_Summary_Empty(t *testing.T) {
	r := &Result{Elapsed: 0}
	got := r.Summary()
	want := "0 of 0 passed (0s)"
	if got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

// ── ImportResult ──────────────────────────────────────────────────────────────

func TestImportResult_Name_UsesResultName(t *testing.T) {
	ir := ImportResult{
		Path:   "./foo/e2e.yaml",
		Result: &Result{Name: "my-suite"},
	}
	if got := ir.Name(); got != "my-suite" {
		t.Errorf("Name() = %q, want %q", got, "my-suite")
	}
}

func TestImportResult_Name_FallsBackToPath(t *testing.T) {
	ir := ImportResult{Path: "./foo/e2e.yaml"}
	if got := ir.Name(); got != "./foo/e2e.yaml" {
		t.Errorf("Name() = %q, want %q", got, "./foo/e2e.yaml")
	}
}

func TestImportResult_Name_FallsBackWhenResultNameEmpty(t *testing.T) {
	ir := ImportResult{
		Path:   "./bar/e2e.yaml",
		Result: &Result{Name: ""},
	}
	if got := ir.Name(); got != "./bar/e2e.yaml" {
		t.Errorf("Name() = %q, want %q", got, "./bar/e2e.yaml")
	}
}

func TestImportResult_Passed_NoError(t *testing.T) {
	ir := ImportResult{}
	if !ir.Passed() {
		t.Error("expected Passed() = true when Err is nil")
	}
}

func TestImportResult_Passed_WithError(t *testing.T) {
	ir := ImportResult{Err: errors.New("failed")}
	if ir.Passed() {
		t.Error("expected Passed() = false when Err is non-nil")
	}
}
