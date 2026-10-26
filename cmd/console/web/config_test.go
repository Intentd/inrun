package web

import (
	"testing"
	"time"
)

func TestGetStrEnv(t *testing.T) {
	t.Run("unset falls back to default", func(t *testing.T) {
		if got := getStrEnv("CC_TEST_UNSET_STR", "fallback"); got != "fallback" {
			t.Errorf("got %q, want fallback", got)
		}
	})
	t.Run("set overrides default", func(t *testing.T) {
		t.Setenv("CC_TEST_STR", "value")
		if got := getStrEnv("CC_TEST_STR", "fallback"); got != "value" {
			t.Errorf("got %q, want value", got)
		}
	})
}

func TestSplitEnv(t *testing.T) {
	t.Run("unset falls back to default", func(t *testing.T) {
		def := []string{"a"}
		got := splitEnv("CC_TEST_UNSET_LIST", def)
		if len(got) != 1 || got[0] != "a" {
			t.Errorf("got %v, want default %v", got, def)
		}
	})

	t.Run("splits on commas and trims whitespace", func(t *testing.T) {
		t.Setenv("CC_TEST_LIST", "a, b ,c")
		got := splitEnv("CC_TEST_LIST", nil)
		want := []string{"a", "b", "c"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got %v, want %v", got, want)
			}
		}
	})

	t.Run("drops empty segments from trailing/consecutive commas", func(t *testing.T) {
		t.Setenv("CC_TEST_LIST_EMPTY", "a,,b,")
		got := splitEnv("CC_TEST_LIST_EMPTY", nil)
		want := []string{"a", "b"}
		if len(got) != len(want) || got[0] != "a" || got[1] != "b" {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestSplitEnvUpper(t *testing.T) {
	t.Setenv("CC_TEST_LIST_UPPER", "foo, Bar")
	got := splitEnvUpper("CC_TEST_LIST_UPPER", nil)
	want := []string{"FOO", "BAR"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGetBoolEnv(t *testing.T) {
	t.Run("unset falls back to default", func(t *testing.T) {
		if got := getBoolEnv("CC_TEST_UNSET_BOOL", true); !got {
			t.Error("got false, want default true")
		}
	})
	t.Run("set true", func(t *testing.T) {
		t.Setenv("CC_TEST_BOOL", "true")
		if got := getBoolEnv("CC_TEST_BOOL", false); !got {
			t.Error("got false, want true")
		}
	})
	t.Run("unparseable value falls back to default", func(t *testing.T) {
		t.Setenv("CC_TEST_BOOL_GARBAGE", "not-a-bool")
		if got := getBoolEnv("CC_TEST_BOOL_GARBAGE", true); !got {
			t.Error("got false, want default true")
		}
	})
}

func TestGetIntEnv(t *testing.T) {
	t.Run("unset falls back to default", func(t *testing.T) {
		if got := getIntEnv("CC_TEST_UNSET_INT", 42); got != 42 {
			t.Errorf("got %d, want 42", got)
		}
	})
	t.Run("set overrides default", func(t *testing.T) {
		t.Setenv("CC_TEST_INT", "7")
		if got := getIntEnv("CC_TEST_INT", 42); got != 7 {
			t.Errorf("got %d, want 7", got)
		}
	})
	t.Run("unparseable value falls back to default", func(t *testing.T) {
		t.Setenv("CC_TEST_INT_GARBAGE", "not-a-number")
		if got := getIntEnv("CC_TEST_INT_GARBAGE", 42); got != 42 {
			t.Errorf("got %d, want default 42", got)
		}
	})
}

func TestGetDurEnv(t *testing.T) {
	t.Run("unset falls back to default seconds", func(t *testing.T) {
		if got := getDurEnv("CC_TEST_UNSET_DUR", 15); got != 15*time.Second {
			t.Errorf("got %v, want 15s", got)
		}
	})
	t.Run("set overrides default, interpreted as seconds", func(t *testing.T) {
		t.Setenv("CC_TEST_DUR", "30")
		if got := getDurEnv("CC_TEST_DUR", 15); got != 30*time.Second {
			t.Errorf("got %v, want 30s", got)
		}
	})
	t.Run("unparseable value falls back to default", func(t *testing.T) {
		t.Setenv("CC_TEST_DUR_GARBAGE", "not-a-number")
		if got := getDurEnv("CC_TEST_DUR_GARBAGE", 15); got != 15*time.Second {
			t.Errorf("got %v, want default 15s", got)
		}
	})
}

func TestHandleEnvVars_PublicDeploymentShorthand(t *testing.T) {
	t.Run("PUBLIC_DEPLOYMENT=true forces NoLogin and disables the runtime manager", func(t *testing.T) {
		t.Setenv("PUBLIC_DEPLOYMENT", "true")
		cfg := handleEnvVars()
		if !cfg.NoLogin {
			t.Error("NoLogin = false, want true under PUBLIC_DEPLOYMENT")
		}
		if cfg.EnableRuntimeManager {
			t.Error("EnableRuntimeManager = true, want false under PUBLIC_DEPLOYMENT")
		}
	})

	t.Run("PUBLIC_DEPLOYMENT shorthand takes precedence over NO_LOGIN=false", func(t *testing.T) {
		t.Setenv("PUBLIC_DEPLOYMENT", "true")
		t.Setenv("NO_LOGIN", "false")
		cfg := handleEnvVars()
		if !cfg.NoLogin {
			t.Error("NoLogin = false, want true — PUBLIC_DEPLOYMENT wins")
		}
	})

	t.Run("without PUBLIC_DEPLOYMENT, defaults are unaffected", func(t *testing.T) {
		cfg := handleEnvVars()
		if cfg.NoLogin {
			t.Error("NoLogin = true, want false by default")
		}
		if !cfg.EnableRuntimeManager {
			t.Error("EnableRuntimeManager = false, want true by default")
		}
	})
}
