package structured

import (
	"os"
	"testing"
)

func TestResolveEnvVar_NoPrefix_ReturnAsIs(t *testing.T) {
	val, err := ResolveEnvVar("plain-value")
	if err != nil || val != "plain-value" {
		t.Errorf("no $ prefix must return value unchanged: %q %v", val, err)
	}
}

func TestResolveEnvVar_SetVar_ReturnsValue(t *testing.T) {
	os.Setenv("MERGER_TEST_VAR", "hello-merger")
	defer os.Unsetenv("MERGER_TEST_VAR")

	os.Setenv("MERGER_SECOND_TEST_VAR", "hello-second-merger")
	defer os.Unsetenv("MERGER_SECOND_TEST_VAR")

	val, err := ResolveEnvVar("$MERGER_TEST_VAR")
	if err != nil || val != "hello-merger" {
		t.Errorf("expected hello-merger, got %q err=%v", val, err)
	}

	secondVal, err := ResolveEnvVar("${MERGER_SECOND_TEST_VAR}")
	if err != nil || secondVal != "hello-second-merger" {
		t.Errorf("expected hello-secondmerger, got %q err=%v", val, err)
	}
}

func TestResolveEnvVar_UnsetVar_Error(t *testing.T) {
	os.Unsetenv("MERGER_DEFINITELY_NOT_SET")
	_, err := ResolveEnvVar("$MERGER_DEFINITELY_NOT_SET")
	if err == nil {
		t.Error("unset env var must return error")
	}
}

func TestResolveEnvVar_EmptyVar_Error(t *testing.T) {
	os.Setenv("MERGER_EMPTY_VAR", "")
	defer os.Unsetenv("MERGER_EMPTY_VAR")

	_, err := ResolveEnvVar("$MERGER_EMPTY_VAR")
	if err == nil {
		t.Error("empty env var must return error")
	}
}
