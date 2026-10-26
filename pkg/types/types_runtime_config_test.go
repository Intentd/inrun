package types

import "testing"

func TestFinalizerConfig_TimeoutDuration(t *testing.T) {
	if d := (*FinalizerConfig)(nil).TimeoutDuration(); d != 0 {
		t.Errorf("nil config: want 0, got %v", d)
	}
	if d := (&FinalizerConfig{Timeout: "30m"}).TimeoutDuration(); d.Minutes() != 30 {
		t.Errorf("30m: got %v", d)
	}
}
