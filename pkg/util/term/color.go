package term

const (
	ColorReset     = "\033[0m"
	ColorBlack     = "\033[30m"
	ColorWhite     = "\033[37m"
	ColorRed       = "\033[31m"
	ColorGreen     = "\033[32m"
	ColorYellow    = "\033[33m"
	ColorBlue      = "\033[34m"
	ColorMagenta   = "\033[35m"
	ColorCyan      = "\033[36m"
	ColorBold      = "\033[1m"
	ColorDim       = "\033[2m"
	ColorItalic    = "\033[3m"
	ColorUnderline = "\033[4m"
	ColorBlink     = "\033[5m"
	ColorReverse   = "\033[7m"
	ColorHidden    = "\033[8m"
	ColorGray      = "\033[90m"
)

func Colorize(color, text string) string {
	return color + text + ColorReset
}

func White(text string) string   { return Colorize(ColorWhite, text) }
func Red(text string) string     { return Colorize(ColorRed, text) }
func Green(text string) string   { return Colorize(ColorGreen, text) }
func Yellow(text string) string  { return Colorize(ColorYellow, text) }
func Blue(text string) string    { return Colorize(ColorBlue, text) }
func Magenta(text string) string { return Colorize(ColorMagenta, text) }
func Cyan(text string) string    { return Colorize(ColorCyan, text) }
func Gray(text string) string    { return Colorize(ColorGray, text) }
func Bold(text string) string    { return Colorize(ColorBold, text) }
func Dim(text string) string     { return Colorize(ColorDim, text) }

// Reset removes all styling from the given text.
func Reset(text string) string { return Colorize(ColorReset, text) }

// SuccessMark returns a green checkmark symbol for successful operations.
func SuccessMark() string      { return Green("✓") }
func SuccessMarkPlain() string { return "✓" }

// FailureMark returns a red cross symbol for failed operations.
func FailureMark() string { return Red("✗") }

// WarningMark returns a yellow warning symbol for non-fatal issues.
func WarningMark() string { return Yellow("⚠") }

// SecureMark returns a shield emoji indicating full security protection.
// Used in CLI output to show that deletion protection is enabled and active.
func SecureMark() string { return "🛡️" }

// SomeSecureMark returns an unlocked padlock emoji indicating partial or
// no security protection. Used in CLI output to show that deletion protection
// is disabled or only partially active (e.g., protectCRD=false but protectCRs=true).
func SomeSecureMark() string { return "🔓" }

// NoSecurityMark returns a cross mark indicating no security protection.
// Used when deletion protection is completely disabled for a resource
// (e.g., security.deletionProtection.enabled = false or per‑CRD protectCRs=false).
func NoSecurityMark() string { return "⛔" }

// InfoMark returns a cyan arrow symbol for informational messages.
func InfoMark() string { return Cyan("→") }
