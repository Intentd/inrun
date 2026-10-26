package expect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// resolver returns a no-op template resolver: the tests use plain commands,
// no templates.
func resolver() *template.Resolver {
	return template.NewResolverFromMap(nil)
}

// scope builds an expectation scope that runs only commands, so the tests need
// no client and no cluster.
func scope(exp types.E2EExpectation) types.E2EExpectationScope {
	return types.E2EExpectationScope{Expectation: exp}
}

// scriptAttempts replaces runAttempt with one that returns outcomes in
// order (nil passes), and returns how many attempts ran.
func scriptAttempts(t *testing.T, outcomes ...error) *int {
	t.Helper()
	calls := 0
	prev := runAttempt
	runAttempt = func(context.Context, types.E2EExpectationScope, *template.Resolver) error {
		calls++
		if calls > len(outcomes) {
			t.Fatalf("attempt %d ran, only %d scripted", calls, len(outcomes))
		}
		return outcomes[calls-1]
	}
	t.Cleanup(func() { runAttempt = prev })
	return &calls
}

// pollEvery sets the poll interval for one test.
func pollEvery(t *testing.T, d time.Duration) {
	t.Helper()
	prev := pollInterval
	pollInterval = d
	t.Cleanup(func() { pollInterval = prev })
}

// counter is a command that appends a line to file on every run and passes
// from run `from` on, so a test can read back exactly how many runs happened.
func counter(file string, from int) string {
	return fmt.Sprintf(`echo x >> %[1]s; test "$(wc -l < %[1]s)" -ge %[2]d`, file, from)
}

func runs(t *testing.T, file string) int {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	return strings.Count(string(raw), "\n")
}

// ── The retry loop: attempts are scripted ─────────────────────────────────

func TestVerify_NoRetryPassesFirstAttempt(t *testing.T) {
	calls := scriptAttempts(t, nil)
	res, err := runStep(context.Background(), scope(types.E2EExpectation{Name: "ok"}), resolver())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Attempts != 1 || res.Configured != 1 || *calls != 1 {
		t.Fatalf("attempts = %d of %d (%d ran), want 1 of 1", res.Attempts, res.Configured, *calls)
	}
	if res.Skipped {
		t.Fatal("Skipped = true, want false")
	}
}

func TestVerify_NoRetryFailsFirstAttempt(t *testing.T) {
	boom := errors.New("boom")
	scriptAttempts(t, boom)
	res, err := runStep(context.Background(), scope(types.E2EExpectation{Name: "bad"}), resolver())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the attempt's error", err)
	}
	if res.Attempts != 1 || res.Configured != 1 {
		t.Fatalf("attempts = %d of %d, want 1 of 1", res.Attempts, res.Configured)
	}
}

func TestVerify_RetryPassesOnSecondAttempt(t *testing.T) {
	calls := scriptAttempts(t, errors.New("not yet"), nil)
	res, err := runStep(context.Background(), scope(types.E2EExpectation{Name: "flaky", Retry: 2}), resolver())
	if err != nil {
		t.Fatalf("expected a pass on the retry, got: %v", err)
	}
	if res.Attempts != 2 || res.Configured != 3 || *calls != 2 {
		t.Fatalf("attempts = %d of %d (%d ran), want 2 of 3: no attempt after the pass", res.Attempts, res.Configured, *calls)
	}
}

func TestVerify_RetryExhaustedReturnsLastError(t *testing.T) {
	first, second, last := errors.New("first"), errors.New("second"), errors.New("last")
	scriptAttempts(t, first, second, last)
	res, err := runStep(context.Background(), scope(types.E2EExpectation{Name: "always-fails", Retry: 2}), resolver())
	if !errors.Is(err, last) {
		t.Fatalf("err = %v, want the last attempt's error", err)
	}
	if res.Attempts != 3 || res.Configured != 3 {
		t.Fatalf("attempts = %d of %d, want 3 of 3", res.Attempts, res.Configured)
	}
}

func TestVerify_NegativeRetryMeansNone(t *testing.T) {
	scriptAttempts(t, errors.New("boom"))
	res, _ := runStep(context.Background(), scope(types.E2EExpectation{Name: "neg", Retry: -1}), resolver())
	if res.Attempts != 1 || res.Configured != 1 {
		t.Fatalf("attempts = %d of %d, want 1 of 1", res.Attempts, res.Configured)
	}
}

func TestVerify_SkippedReportsConfiguredWithoutRunning(t *testing.T) {
	calls := scriptAttempts(t)
	res, err := runStep(context.Background(), scope(types.E2EExpectation{
		Name:  "gated",
		Retry: 2,
		When:  types.Conditions{{Field: "x", Equals: "y"}},
	}), resolver())
	if err != errSkipped {
		t.Fatalf("err = %v, want errSkipped", err)
	}
	if !res.Skipped || res.Attempts != 0 || *calls != 0 {
		t.Fatalf("skipped = %v, attempts = %d (%d ran), want skipped with none run", res.Skipped, res.Attempts, *calls)
	}
	if res.Configured != 3 {
		t.Fatalf("configured = %d, want 3: the setting is reported even when skipped", res.Configured)
	}
}

// ── One attempt: polling until the timeout, with real commands ────────────

// With a poll interval longer than the timeout, an attempt polls exactly
// twice: at its start and on its deadline.

func TestVerifyOnce_LastPollLandsOnTheDeadline(t *testing.T) {
	pollEvery(t, time.Hour)
	file := filepath.Join(t.TempDir(), "runs")
	err := attempt(context.Background(), scope(types.E2EExpectation{
		Name:     "late",
		Timeout:  "100ms",
		Commands: []types.E2ECommand{{Run: counter(file, 2)}},
	}), resolver())
	if err != nil {
		t.Fatalf("expected the deadline poll to pass, got: %v", err)
	}
	if n := runs(t, file); n != 2 {
		t.Fatalf("command ran %d times, want 2", n)
	}
}

func TestVerifyOnce_TimeoutDoesNotRunTheChecksAgain(t *testing.T) {
	pollEvery(t, time.Hour)
	file := filepath.Join(t.TempDir(), "runs")
	err := attempt(context.Background(), scope(types.E2EExpectation{
		Name:     "never",
		Timeout:  "100ms",
		Commands: []types.E2ECommand{{Run: counter(file, 99)}},
	}), resolver())
	if err == nil || !strings.Contains(err.Error(), "timeout after 100ms") {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if n := runs(t, file); n != 2 {
		t.Fatalf("command ran %d times, want 2: the timeout error is the last poll's, not a third run", n)
	}
}

func TestVerifyOnce_PollsUntilAPass(t *testing.T) {
	pollEvery(t, 10*time.Millisecond)
	file := filepath.Join(t.TempDir(), "runs")
	err := attempt(context.Background(), scope(types.E2EExpectation{
		Name:     "eventually",
		Timeout:  "5s",
		Commands: []types.E2ECommand{{Run: counter(file, 3)}},
	}), resolver())
	if err != nil {
		t.Fatalf("expected a pass within the timeout, got: %v", err)
	}
	if n := runs(t, file); n != 3 {
		t.Fatalf("command ran %d times, want 3: polling stops at the first pass", n)
	}
}

// ── Retry and polling together ─────────────────────────────────────────────

func TestVerify_RetryRerunsTheAttempt(t *testing.T) {
	pollEvery(t, time.Hour)
	file := filepath.Join(t.TempDir(), "runs")
	res, err := runStep(context.Background(), scope(types.E2EExpectation{
		Name:    "second-attempt",
		Timeout: "100ms",
		Retry:   2,
		// Two polls per attempt: runs 1–2 fail, run 3 (attempt 2) passes.
		Commands: []types.E2ECommand{{Run: counter(file, 3)}},
	}), resolver())
	if err != nil {
		t.Fatalf("expected a pass on the second attempt, got: %v", err)
	}
	if res.Attempts != 2 || res.Configured != 3 {
		t.Fatalf("attempts = %d of %d, want 2 of 3", res.Attempts, res.Configured)
	}
	if n := runs(t, file); n != 3 {
		t.Fatalf("command ran %d times, want 3", n)
	}
}

// ── --dry-run=server ───────────────────────────────────────────────────────

func TestVerify_ServerDryRunSendsWritesOnceAndSkipsTheRest(t *testing.T) {
	kubectl.SetDryRun(true)
	t.Cleanup(func() { kubectl.SetDryRun(false) })
	file := filepath.Join(t.TempDir(), "runs")

	res, err := runStep(context.Background(), scope(types.E2EExpectation{
		Name:    "commands-only",
		Timeout: "100ms",
		Retry:   2,
		// Would fail, and would leave a trace if it ran.
		Commands: []types.E2ECommand{{Run: counter(file, 99)}},
	}), resolver())
	if err != nil {
		t.Fatalf("a dry run with no kubectl writes has nothing to reject, got: %v", err)
	}
	if res.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1: a dry run is never retried", res.Attempts)
	}
	if _, statErr := os.Stat(file); !os.IsNotExist(statErr) {
		t.Fatal("the command ran: a server dry run skips commands")
	}
}
