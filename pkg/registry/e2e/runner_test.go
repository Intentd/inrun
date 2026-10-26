package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
)

func TestRunnerAbs(t *testing.T) {
	wd := t.TempDir()

	r := &Runner{
		e2eDir: wd,
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "absolute path",
			path: filepath.Join(wd, "manifest.yaml"),
			want: filepath.Join(wd, "manifest.yaml"),
		},
		{
			name: "relative path",
			path: "manifest.yaml",
			want: filepath.Join(wd, "manifest.yaml"),
		},
		{
			name: "nested relative path",
			path: filepath.Join("manifests", "manifest.yaml"),
			want: filepath.Join(wd, "manifests", "manifest.yaml"),
		},
		{
			name: "relative path with parent directory",
			path: filepath.Join("..", "manifest.yaml"),
			want: filepath.Join(filepath.Dir(wd), "manifest.yaml"),
		},
		{
			name: "empty path",
			path: "",
			want: wd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.abs(tt.path)
			if got != tt.want {
				t.Errorf("abs(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// commandsSpec writes a cluster-less E2E spec into dir: one expectation that
// passes or fails, and a spec-level onFailure that touches marker.
func commandsSpec(t *testing.T, dir string, fail bool, marker string) string {
	t.Helper()
	run := "true"
	if fail {
		run = "exit 1"
	}
	spec := `apiVersion: inrun.dev/v1
kind: E2E
metadata:
  name: runner-test
spec:
  cluster:
    provider: none
  onFailure:
    commands:
      - run: touch ` + marker + `
  expect:
    - name: the step
      timeout: 1s
      commands:
        - run: '` + run + `'
`
	path := filepath.Join(dir, "e2e.yaml")
	if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestRun_ServerDryRunNeedsACluster(t *testing.T) {
	dir := t.TempDir()
	r, err := New(commandsSpec(t, dir, false, filepath.Join(dir, "marker")), Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "provider: none") {
		t.Fatalf("err = %v, want the no-cluster dry-run error", err)
	}
}

func TestRun_SpecOnFailureRunsOnlyWhenSomethingFailed(t *testing.T) {
	for _, fail := range []bool{false, true} {
		dir := t.TempDir()
		marker := filepath.Join(dir, "on-failure-ran")
		r, err := New(commandsSpec(t, dir, fail, marker), Options{})
		if err != nil {
			t.Fatal(err)
		}
		_, runErr := r.Run(context.Background())
		if (runErr != nil) != fail {
			t.Fatalf("fail=%v: Run err = %v", fail, runErr)
		}
		if exists(marker) != fail {
			t.Fatalf("fail=%v: spec onFailure ran = %v, want %v", fail, exists(marker), fail)
		}
	}
}

func TestSuiteOnFailure_SkipsAggregatorsAndServerDryRuns(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "on-failure-ran")
	r, err := New(commandsSpec(t, dir, true, marker), Options{})
	if err != nil {
		t.Fatal(err)
	}

	r.suiteOnFailure(context.Background(), true)
	if exists(marker) {
		t.Fatal("ran for a pure aggregator: its imports run their own")
	}

	kubectl.SetDryRun(true)
	r.suiteOnFailure(context.Background(), false)
	kubectl.SetDryRun(false)
	if exists(marker) {
		t.Fatal("ran under a server dry run: nothing changed to diagnose")
	}

	r.suiteOnFailure(context.Background(), false)
	if !exists(marker) {
		t.Fatal("did not run for a failed suite")
	}
}

func TestRun_WritesTheReportWhenAStepFails(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.json")
	r, err := New(commandsSpec(t, dir, true, filepath.Join(dir, "marker")), Options{ReportFile: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(context.Background()); err == nil {
		t.Fatal("expected the failing step to fail the run")
	}
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("no report: %v", err)
	}
	var rep struct {
		Cases []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
			Error  string `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatalf("report is not JSON: %v\n%s", err, raw)
	}
	if len(rep.Cases) != 1 || rep.Cases[0].Name != "the step" || rep.Cases[0].Passed || rep.Cases[0].Error == "" {
		t.Fatalf("report cases = %+v, want the failed step with its error", rep.Cases)
	}
}

// The spec's finally runs once at the end of the suite, whether the suite
// passed or failed: here it appends a line to a file, which must have one.
func TestRun_SpecFinallyRunsOnceWhetherOrNotSomethingFailed(t *testing.T) {
	for _, fail := range []bool{false, true} {
		dir := t.TempDir()
		marks := filepath.Join(dir, "finally-ran")
		run := "true"
		if fail {
			run = "exit 1"
		}
		spec := `apiVersion: inrun.dev/v1
kind: E2E
metadata:
  name: runner-finally-test
spec:
  cluster:
    provider: none
  finally:
    commands:
      - run: echo ran >> ` + marks + `
  expect:
    - name: the step
      timeout: 1s
      commands:
        - run: '` + run + `'
`
		path := filepath.Join(dir, "e2e.yaml")
		if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
			t.Fatal(err)
		}
		r, err := New(path, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if _, runErr := r.Run(context.Background()); (runErr != nil) != fail {
			t.Fatalf("fail=%v: Run err = %v", fail, runErr)
		}
		data, _ := os.ReadFile(marks)
		if got := strings.Count(string(data), "ran\n"); got != 1 {
			t.Fatalf("fail=%v: spec finally ran %d times, want 1", fail, got)
		}
	}
}
