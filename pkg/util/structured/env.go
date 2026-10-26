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

// ExpandEnv expands environment variable references using the resolved E2E environment.
// Unresolved variables are left unchanged.
func ExpandEnv(value string, env map[string]string) string {
	return os.Expand(value, func(key string) string {
		v, ok := env[key]
		if !ok {
			return "${" + key + "}"
		}
		return v
	})
}
