package structured

import (
	"fmt"
	"os"
	"strings"
)

// ResolveEnvVar replaces $VAR_NAME or ${VAR_NAME} with its environment variable value.
func ResolveEnvVar(s string) (string, error) {
	if !strings.HasPrefix(s, "$") {
		return s, nil
	}

	val := os.ExpandEnv(s)
	if val == "" {
		return "", fmt.Errorf("env var %q is not set or empty", os.Getenv(s))
	}
	return val, nil
}
