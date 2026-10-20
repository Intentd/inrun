package term

import "testing"

func TestWordWrap(t *testing.T) {
	cases := []struct {
		name, in string
		width    int
		want     string
	}{
		{"short", "fits on one line", 20, "fits on one line"},
		{"breaks between words", "provisions a single instance", 12, "provisions a\n> single\n> instance"},
		{"counts characters, not bytes", "single‑instance pattern", 15, "single‑instance\n> pattern"},
		{"long word alone", "a supercalifragilistic word", 8, "a\n> supercalifragilistic\n> word"},
		{"collapses spacing", "  one   two\nthree ", 40, "one two three"},
		{"empty", "", 10, ""},
	}
	for _, c := range cases {
		if got := WordWrap(c.in, c.width, "> "); got != c.want {
			t.Errorf("%s: WordWrap(%q, %d) = %q, want %q", c.name, c.in, c.width, got, c.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{2765, "2.7 KB"},
		{1 << 20, "1.0 MB"},
		{5 << 30, "5.0 GB"},
		{1<<63 - 1, "8.0 EB"},
	}
	for _, c := range cases {
		if got := FormatSize(c.in); got != c.want {
			t.Errorf("FormatSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVisibleLen(t *testing.T) {
	cases := []struct {
		name, in string
		want     int
	}{
		{"plain", "abc", 3},
		{"empty", "", 0},
		{"runes, not bytes", "héllo ✓", 7},
		{"colour codes are invisible", "\033[32m✓ passed\033[0m", 8},
		{"several codes", "\033[1m\033[31mab\033[0m c", 4},
		{"unterminated code hides the rest", "ok\033[3", 2},
	}
	for _, c := range cases {
		if got := VisibleLen(c.in); got != c.want {
			t.Errorf("%s: VisibleLen(%q) = %d, want %d", c.name, c.in, got, c.want)
		}
	}
}

func TestPadRight(t *testing.T) {
	cases := []struct {
		name, in string
		width    int
		want     string
	}{
		{"pads", "ab", 5, "ab   "},
		{"exact width", "abc", 3, "abc"},
		{"longer is left alone", "abcdef", 3, "abcdef"},
		{"pads to visible width", "\033[31mab\033[0m", 4, "\033[31mab\033[0m  "},
		{"empty", "", 2, "  "},
	}
	for _, c := range cases {
		if got := PadRight(c.in, c.width); got != c.want {
			t.Errorf("%s: PadRight(%q, %d) = %q, want %q", c.name, c.in, c.width, got, c.want)
		}
	}
}

func TestOrDefault(t *testing.T) {
	if got := OrDefault("set", "fallback"); got != "set" {
		t.Errorf("OrDefault(set) = %q", got)
	}
	if got := OrDefault("", "fallback"); got != "fallback" {
		t.Errorf("OrDefault(empty) = %q", got)
	}
	if got := OrDefault(" ", "fallback"); got != " " {
		t.Errorf("OrDefault(space) = %q, want the space kept", got)
	}
}

func TestContainsFold(t *testing.T) {
	tags := []string{"Postgres", "database", "SQL"}
	cases := []struct {
		tag  string
		want bool
	}{
		{"postgres", true},
		{"POSTGRES", true},
		{"sql", true},
		{"data", false},
		{"", false},
	}
	for _, c := range cases {
		if got := ContainsFold(tags, c.tag); got != c.want {
			t.Errorf("ContainsFold(%v, %q) = %v, want %v", tags, c.tag, got, c.want)
		}
	}
	if ContainsFold(nil, "x") {
		t.Error("ContainsFold(nil) = true")
	}
}
