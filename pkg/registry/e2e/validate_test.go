package e2e

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

// ── ExpandExpectIncludes ──────────────────────────────────────────────────────

func TestExpandExpectIncludes_NoIncludes(t *testing.T) {
	expects := []types.E2EExpectation{
		{Name: "a"},
		{Name: "b"},
	}
	got, err := ExpandExpectIncludes(expects, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestExpandExpectIncludes_ExpandsInPlace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "extra.yaml"), `
expect:
  - name: x
  - name: y
`)
	expects := []types.E2EExpectation{
		{Name: "before"},
		{Include: "./extra.yaml"},
		{Name: "after"},
	}
	got, err := ExpandExpectIncludes(expects, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 entries, got %d: %v", len(got), got)
	}
	names := []string{got[0].Name, got[1].Name, got[2].Name, got[3].Name}
	want := []string{"before", "x", "y", "after"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("index %d: got %q, want %q", i, n, want[i])
		}
	}
}

func TestExpandExpectIncludes_MultipleIncludes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.yaml"), `
expect:
  - name: a1
  - name: a2
`)
	writeFile(t, filepath.Join(dir, "b.yaml"), `
expect:
  - name: b1
`)
	expects := []types.E2EExpectation{
		{Include: "./a.yaml"},
		{Name: "mid"},
		{Include: "./b.yaml"},
	}
	got, err := ExpandExpectIncludes(expects, dir)
	if err != nil {
		t.Fatal(err)
	}
	names := names(got)
	want := []string{"a1", "a2", "mid", "b1"}
	if !strSliceEq(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestExpandExpectIncludes_MissingExpectKeyYieldsEmpty(t *testing.T) {
	dir := t.TempDir()
	// File exists but has no 'expect:' key — should expand to nothing, no error.
	writeFile(t, filepath.Join(dir, "empty.yaml"), `
metadata:
  name: something
`)
	expects := []types.E2EExpectation{{Include: "./empty.yaml"}}
	got, err := ExpandExpectIncludes(expects, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty expansion, got %v", got)
	}
}

func TestExpandExpectIncludes_MissingFile(t *testing.T) {
	dir := t.TempDir()
	expects := []types.E2EExpectation{{Include: "./nonexistent.yaml"}}
	_, err := ExpandExpectIncludes(expects, dir)
	if err == nil {
		t.Fatal("expected error for missing include file, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent.yaml") {
		t.Errorf("error should mention the file name, got: %v", err)
	}
}

func TestExpandExpectIncludes_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `{{{not yaml`)
	expects := []types.E2EExpectation{{Include: "./bad.yaml"}}
	_, err := ExpandExpectIncludes(expects, dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestExpandExpectIncludes_AbsoluteIncludePath(t *testing.T) {
	dir := t.TempDir()
	absFile := filepath.Join(dir, "abs.yaml")
	writeFile(t, absFile, `
expect:
  - name: abs-entry
`)
	expects := []types.E2EExpectation{{Include: absFile}}
	got, err := ExpandExpectIncludes(expects, "/unrelated/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "abs-entry" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestExpandExpectIncludes_WorkDirIsIncludeFileDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "steps.yaml"), `
expect:
  - name: step-a
  - name: step-b
`)
	expects := []types.E2EExpectation{{Include: filepath.Join(sub, "steps.yaml")}}
	got, err := ExpandExpectIncludes(expects, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 expectations, got %d", len(got))
	}
	for _, exp := range got {
		if exp.WorkDir != sub {
			t.Errorf("expected WorkDir %q, got %q", sub, exp.WorkDir)
		}
	}
}

// ── ValidateKubectl ───────────────────────────────────────────────────────────

func TestValidateKubectl_NoKubectlBlock(t *testing.T) {
	expects := []types.E2EExpectation{{Name: "plain"}}
	errs := ValidateKubectl(expects)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// get ─────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_GetMissingKind(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Name:          "foo",
		Field:         ".status",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "kind")
}

func TestValidateKubectl_GetMissingName(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Field:         ".status",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "name")
}

func TestValidateKubectl_GetMissingFieldAndFormat(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "field or format")
}

func TestValidateKubectl_GetNoAssertion(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:  "Deployment",
		Name:  "app",
		Field: ".status",
	})
	requireErr(t, errs, "assertion")
}

func TestValidateKubectl_GetInvalidFormat(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		Format:        "xml",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "json or yaml")
}

func TestValidateKubectl_GetJQWithoutJSON(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		Format:        "yaml",
		JQ:            ".metadata",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "jq requires format: json")
}

func TestValidateKubectl_GetYQWithoutYAML(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		Format:        "json",
		YQ:            ".metadata",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "yq requires format: yaml")
}

func TestValidateKubectl_GetValid(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		Field:         ".status.phase",
		E2EAssertions: types.E2EAssertions{Equals: "Running"},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// logs ────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_LogsMissingSelector(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		E2EAssertions: types.E2EAssertions{Contains: "started"},
	})
	requireErr(t, errs, "name, labelSelector, or leaderElection")
}

func TestValidateKubectl_LogsNoAssertion(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		Name: "my-pod",
	})
	requireErr(t, errs, "assertion")
}

func TestValidateKubectl_LogsLeaderElectionMissingLease(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		LeaderElection: &types.E2EKubectlLeaderElection{},
		E2EAssertions:  types.E2EAssertions{Contains: "started"},
	})
	requireErr(t, errs, "leaderElection.lease")
}

func TestValidateKubectl_LogsLeaderElectionMutuallyExclusiveWithName(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		Name: "my-pod",
		LeaderElection: &types.E2EKubectlLeaderElection{
			Lease: "my-lease",
		},
		E2EAssertions: types.E2EAssertions{Contains: "started"},
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_LogsLeaderElectionMutuallyExclusiveWithSelector(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		LabelSelector: "app=foo",
		LeaderElection: &types.E2EKubectlLeaderElection{
			Lease: "my-lease",
		},
		E2EAssertions: types.E2EAssertions{Contains: "started"},
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_LogsLeaderElectionValid(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		LeaderElection: &types.E2EKubectlLeaderElection{
			Lease: "my-lease",
		},
		E2EAssertions: types.E2EAssertions{Contains: "started"},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// delete ──────────────────────────────────────────────────────────────────────

func TestValidateKubectl_DeleteNeitherFileNorKind(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{})
	requireErr(t, errs, "file or (kind + name)")
}

func TestValidateKubectl_DeleteKindWithoutName(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{Kind: "Deployment"})
	requireErr(t, errs, "file or (kind + name)")
}

func TestValidateKubectl_DeleteMutuallyExclusive(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{
		File: "./foo.yaml",
		Kind: "Deployment",
		Name: "app",
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_DeleteByFile(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{File: "./foo.yaml"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateKubectl_DeleteByKindAndName(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{Kind: "Deployment", Name: "app"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// port-forward ────────────────────────────────────────────────────────────────

func TestValidateKubectl_PortForwardAssertionWithoutPath(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service:       "my-svc",
		Port:          8080,
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "path is required")
}

func TestValidateKubectl_PortForwardNoServiceOrPod(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Port: 8080,
	})
	requireErr(t, errs, "service, pod, or leaderElection")
}

func TestValidateKubectl_PortForwardZeroPort(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Port:    0,
	})
	requireErr(t, errs, "port must be > 0")
}

func TestValidateKubectl_PortForwardValidNoAssertion(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Port:    8080,
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateKubectl_PortForwardLeaderElectionMissingLease(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Port:           8080,
		LeaderElection: &types.E2EKubectlLeaderElection{},
	})
	requireErr(t, errs, "leaderElection.lease")
}

// apply ───────────────────────────────────────────────────────────────────────

func TestValidateKubectl_ApplyNeitherFileNorInline(t *testing.T) {
	errs := validateKubectlApply("loc", types.E2EKubectlApply{})
	requireErr(t, errs, "file or inline")
}

func TestValidateKubectl_ApplyMutuallyExclusive(t *testing.T) {
	errs := validateKubectlApply("loc", types.E2EKubectlApply{
		File:   "./foo.yaml",
		Inline: "apiVersion: v1",
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_ApplyByFile(t *testing.T) {
	errs := validateKubectlApply("loc", types.E2EKubectlApply{File: "./foo.yaml"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// patch ───────────────────────────────────────────────────────────────────────

func TestValidateKubectl_PatchInvalidType(t *testing.T) {
	errs := validateKubectlPatch("loc", types.E2EKubectlPatch{
		Kind:  "Deployment",
		Name:  "app",
		Patch: `{"spec":{}}`,
		Type:  "invalid",
	})
	requireErr(t, errs, "merge, strategic, or json")
}

func TestValidateKubectl_PatchMissingPatch(t *testing.T) {
	errs := validateKubectlPatch("loc", types.E2EKubectlPatch{
		Kind: "Deployment",
		Name: "app",
	})
	requireErr(t, errs, "patch is required")
}

func TestValidateKubectl_PatchValid(t *testing.T) {
	errs := validateKubectlPatch("loc", types.E2EKubectlPatch{
		Kind:  "Deployment",
		Name:  "app",
		Patch: `{"spec":{}}`,
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// top ─────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_TopInvalidKind(t *testing.T) {
	errs := validateKubectlTop("loc", types.E2EKubectlTop{
		Kind:          "deployment",
		E2EAssertions: types.E2EAssertions{Contains: "cpu"},
	})
	requireErr(t, errs, "pod or node")
}

func TestValidateKubectl_TopMissingKind(t *testing.T) {
	errs := validateKubectlTop("loc", types.E2EKubectlTop{E2EAssertions: types.E2EAssertions{Contains: "cpu"}})
	requireErr(t, errs, "kind is required")
}

func TestValidateKubectl_TopValidPod(t *testing.T) {
	errs := validateKubectlTop("loc", types.E2EKubectlTop{
		Kind:          "pod",
		E2EAssertions: types.E2EAssertions{Contains: "cpu"},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func requireErr(t *testing.T, errs []error, substr string) {
	t.Helper()
	for _, e := range errs {
		if strings.Contains(e.Error(), substr) {
			return
		}
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	t.Errorf("expected an error containing %q, got: %v", substr, msgs)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(expects []types.E2EExpectation) []string {
	out := make([]string, len(expects))
	for i, e := range expects {
		out[i] = e.Name
	}
	return out
}

func strSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var _ = errors.New // keep import used if no error-wrapping needed
