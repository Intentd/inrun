package structured

import (
	"os"
	"strings"
	"testing"
)

// RequireErr fails unless at least one error in errs contains substr.
func RequireErr(t *testing.T, errs []error, substr string) {
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

// WriteFile writes content to path, failing the test on error.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
