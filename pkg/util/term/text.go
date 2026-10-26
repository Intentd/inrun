package term

import (
	"fmt"
	"strings"
	"unicode/utf8"
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

// WordWrap breaks s into lines of at most width characters, between words,
// and starts each line after the first with indent. A word longer than width
// gets a line of its own.
func WordWrap(s string, width int, indent string) string {
	var b strings.Builder
	line := 0
	for _, w := range strings.Fields(s) {
		n := utf8.RuneCountInString(w)
		switch {
		case line == 0:
		case line+1+n > width:
			b.WriteString("\n")
			b.WriteString(indent)
			line = 0
		default:
			b.WriteByte(' ')
			line++
		}
		b.WriteString(w)
		line += n
	}
	return b.String()
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
