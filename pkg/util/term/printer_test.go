package term

import (
	"strings"
	"testing"
	"time"
)

// ── Bold / Cyan ───────────────────────────────────────────────────────────────

func TestBold_ContainsText(t *testing.T) {
	out := Bold("hello")
	if !strings.Contains(out, "hello") {
		t.Error("Bold must contain the original text")
	}
	if !strings.Contains(out, ColorBold) {
		t.Error("Bold must contain the bold ANSI code")
	}
}

func TestCyan_ContainsText(t *testing.T) {
	out := Cyan("cyan text")
	if !strings.Contains(out, "cyan text") {
		t.Error("Cyan must contain the original text")
	}
	if !strings.Contains(out, ColorCyan) {
		t.Error("Cyan must contain the cyan ANSI code")
	}
}

// ── FormatDuration ────────────────────────────────────────────────────────────

func TestFormatDuration_Seconds(t *testing.T) {
	out := FormatDuration(30 * time.Second)
	if out != "30s" {
		t.Errorf("expected 30s, got %q", out)
	}
}

func TestFormatDuration_Minutes(t *testing.T) {
	out := FormatDuration(5 * time.Minute)
	if out != "5m" {
		t.Errorf("expected 5m, got %q", out)
	}
}

func TestFormatDuration_Hours(t *testing.T) {
	out := FormatDuration(3 * time.Hour)
	if out != "3h" {
		t.Errorf("expected 3h, got %q", out)
	}
}

func TestFormatDuration_Days(t *testing.T) {
	out := FormatDuration(3 * 24 * time.Hour)
	if out != "3d" {
		t.Errorf("expected 3d, got %q", out)
	}
}

func TestFormatDuration_Weeks(t *testing.T) {
	out := FormatDuration(14 * 24 * time.Hour)
	if out != "2w" {
		t.Errorf("expected 2w, got %q", out)
	}
}

// ── PrintTable ────────────────────────────────────────────────────────────────

func TestPrintTable_WritesToWriter(t *testing.T) {
	var buf strings.Builder
	PrintTable(&buf, []string{"Name", "Status"}, [][]string{
		{"web", "Ready"},
		{"api", "Pending"},
	})
	out := buf.String()
	if !strings.Contains(out, "Name") || !strings.Contains(out, "Status") {
		t.Error("output must contain header columns")
	}
	if !strings.Contains(out, "web") || !strings.Contains(out, "Ready") {
		t.Error("output must contain row data")
	}
}

func TestPrintTable_EmptyRows(t *testing.T) {
	var buf strings.Builder
	PrintTable(&buf, []string{"Name"}, nil)
	out := buf.String()
	if !strings.Contains(out, "Name") {
		t.Error("output must still contain header")
	}
}

// ── HealthIcon ────────────────────────────────────────────────────────────────

func TestHealthIcon_Ready(t *testing.T) {
	icon := HealthIcon("Ready")
	if !strings.Contains(icon, ColorGreen) {
		t.Errorf("Ready must produce green icon, got %q", icon)
	}
}

func TestHealthIcon_Failed(t *testing.T) {
	icon := HealthIcon("Failed")
	if !strings.Contains(icon, ColorRed) {
		t.Errorf("Failed must produce red icon, got %q", icon)
	}
}

func TestHealthIcon_Pending(t *testing.T) {
	icon := HealthIcon("Pending")
	if !strings.Contains(icon, ColorYellow) {
		t.Errorf("Pending must produce yellow icon, got %q", icon)
	}
}

func TestHealthIcon_Unknown(t *testing.T) {
	icon := HealthIcon("something-random")
	if !strings.Contains(icon, ColorGray) {
		t.Errorf("Unknown status must produce gray icon, got %q", icon)
	}
}
