package structured

import "testing"

func TestReversed_Empty(t *testing.T) {
	out := Reversed([]int{})
	if len(out) != 0 {
		t.Error("reversed empty slice must be empty")
	}
}

func TestReversed_Single(t *testing.T) {
	out := Reversed([]string{"a"})
	if len(out) != 1 || out[0] != "a" {
		t.Errorf("unexpected: %v", out)
	}
}

func TestReversed_Multiple(t *testing.T) {
	in := []int{1, 2, 3, 4, 5}
	out := Reversed(in)
	want := []int{5, 4, 3, 2, 1}
	for i, v := range want {
		if out[i] != v {
			t.Errorf("index %d: expected %d, got %d", i, v, out[i])
		}
	}
	// Original must be unchanged
	if in[0] != 1 {
		t.Error("Reversed must not modify the original slice")
	}
}

func TestReversed_Strings(t *testing.T) {
	out := Reversed([]string{"a", "b", "c"})
	if out[0] != "c" || out[1] != "b" || out[2] != "a" {
		t.Errorf("unexpected: %v", out)
	}
}
