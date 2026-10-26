package term

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with stdout redirected to a pipe, which is not a
// terminal, and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestSpinnerNonTTY(t *testing.T) {
	cases := []struct {
		name string
		run  func()
		want []string
	}{
		{
			name: "success prints only the start line",
			run:  func() { StartSpinner("Deleting cr.yaml...").Success() },
			want: []string{"Deleting cr.yaml..."},
		},
		{
			name: "failure prints a final line",
			run:  func() { StartSpinner("Deleting cr.yaml...").Failure() },
			want: []string{"Deleting cr.yaml...", FailureMark() + " Deleting cr.yaml..."},
		},
		{
			name: "success after update prints the new message",
			run: func() {
				sp := StartSpinner("Deleting cr.yaml...")
				sp.Update("Deleting cr.yaml (finalizers cleared)")
				sp.Success()
			},
			want: []string{"Deleting cr.yaml...", SuccessMark() + " Deleting cr.yaml (finalizers cleared)"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, tc.run)
			var got []string
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				got = append(got, strings.TrimSpace(line))
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}
