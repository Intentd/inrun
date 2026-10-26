package expect

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

// cmd is a post-block command that runs s.
func cmd(s string) types.E2ECommand { return types.E2ECommand{Run: s} }

func TestRunPost_CarriesOnAndWarns(t *testing.T) {
	dir := t.TempDir()
	missed := cmd("echo hello")
	missed.Assert = &types.Assertions{Equals: "goodbye"}
	block := &types.E2ESideEffects{Commands: []types.E2ECommand{
		cmd("exit 3"),
		cmd("touch ran"),
		missed,
	}}

	warnings := runPost(context.Background(), "finally", block,
		types.E2EExpectationScope{WorkDir: dir}, resolver())

	if _, err := os.Stat(filepath.Join(dir, "ran")); err != nil {
		t.Fatal("the command after a failed one did not run")
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %q, want 2: the failed command and the missed assertion", warnings)
	}
	if !strings.HasPrefix(warnings[0], "finally commands[0]: ") || !strings.HasPrefix(warnings[1], "finally commands[2]: ") {
		t.Errorf("warnings = %q, want each named by its block and position", warnings)
	}
}

func TestRunPost_CommandsSeeTheExpectationEnv(t *testing.T) {
	dir := t.TempDir()
	spec := &types.E2ESpec{Env: map[string]string{"SPEC_TOKEN": "from-spec"}}
	env := spec.ResolveEnv(types.E2EExpectation{Env: map[string]string{"WHO": "from-expect"}})
	block := &types.E2ESideEffects{Commands: []types.E2ECommand{
		cmd(`test "$SPEC_TOKEN $WHO" = "from-spec from-expect"`),
	}}

	if w := runPost(context.Background(), "onFailure", block,
		types.E2EExpectationScope{WorkDir: dir, Env: env}, resolver()); len(w) != 0 {
		t.Fatalf("warnings = %q, want none: the command should see both env values", w)
	}
}

func TestRunPost_SurvivesACancelledRun(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	block := &types.E2ESideEffects{Commands: []types.E2ECommand{cmd("touch cleaned")}}

	runPost(ctx, "finally", block, types.E2EExpectationScope{WorkDir: dir}, resolver())

	if _, err := os.Stat(filepath.Join(dir, "cleaned")); err != nil {
		t.Fatal("finally did not run after the run was cancelled")
	}
}

func TestSuitePost_UnknownEnvIsAWarning(t *testing.T) {
	dir := t.TempDir()
	spec := &types.E2ESpec{OnFailure: &types.E2EOnFailure{E2ESideEffects: types.E2ESideEffects{
		Commands: []types.E2ECommand{cmd("touch ran")},
		External: []types.E2EExternalCheck{Call("probe", "http://${INRUN_TEST_UNDECLARED}/x")},
	}}}

	w := SuitePost(context.Background(), "onFailure", &spec.OnFailure.E2ESideEffects, SuitePostOptions{Spec: spec, WorkDir: dir})

	if len(w) != 1 || !strings.Contains(w[0], "${INRUN_TEST_UNDECLARED}") {
		t.Fatalf("warnings = %q, want one naming the unknown env", w)
	}
	if _, err := os.Stat(filepath.Join(dir, "ran")); err == nil {
		t.Error("the block ran with an unresolved env reference")
	}
}

func TestSuitePost_NoBlock(t *testing.T) {
	if w := SuitePost(context.Background(), "onFailure", nil, SuitePostOptions{Spec: &types.E2ESpec{}}); w != nil {
		t.Fatalf("warnings = %q, want none without a block", w)
	}
}
