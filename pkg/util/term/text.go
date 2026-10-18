package term

import (
	"fmt"
	"strings"
)

func FormatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func WordWrap(s string, width int, indent string) string {
	if len(s) <= width {
		return s
	}
	return s[:width] + "\n" + indent + WordWrap(s[width:], width, indent)
}

// VisibleLen returns the visible rune count of s, stripping ANSI escape sequences.
func VisibleLen(s string) int {
	inEscape := false
	n := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
}

// PadRight pads s with trailing spaces until its visible width reaches width.
func PadRight(s string, width int) string {
	if w := VisibleLen(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

func OrDefault(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

func ContainsFold(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}
