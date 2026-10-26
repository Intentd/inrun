package kubectl

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyDryRun(t *testing.T) {
	cases := []struct {
		args []string
		dry  bool
	}{
		{[]string{"apply", "-f", "-"}, true},
		{[]string{"patch", "deploy/web", "-p", "{}"}, true},
		{[]string{"delete", "pod", "web"}, true},
		{[]string{"scale", "deploy/web", "--replicas=2"}, true},
		{[]string{"rollout", "restart", "deploy/web"}, true},
		{[]string{"rollout", "undo", "deploy/web"}, true},
		{[]string{"rollout", "status", "deploy/web"}, false}, // a read: kubectl rejects --dry-run on it
		{[]string{"rollout", "history", "deploy/web"}, false},
		{[]string{"get", "pods"}, false},
		{[]string{"logs", "web"}, false},
		{nil, false},
	}
	for _, c := range cases {
		got := applyDryRun(append([]string(nil), c.args...))
		has := len(got) > 0 && got[len(got)-1] == "--dry-run=server"
		if has != c.dry {
			t.Errorf("applyDryRun(%v) = %v, want dry-run %v", c.args, got, c.dry)
		}
	}
}

// The rewrite must reach the command: kubectl, run through a stub that
// prints its arguments, receives --dry-run=server.
func TestRunCommand_DryRunReachesKubectl(t *testing.T) {
	dir := t.TempDir()
	stub := "#!/bin/sh\necho \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	SetDryRun(true)
	t.Cleanup(func() { SetDryRun(false) })

	out, err := runCommand(context.Background(), runOptions{args: []string{"apply", "-f", "web.yaml"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "apply -f web.yaml --dry-run=server" {
		t.Fatalf("kubectl got %q, want the dry-run flag", out)
	}
}
