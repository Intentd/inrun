//go:build gateway

package output

// PrintMigrate is a no-op in gateway builds — migration output comes from the
// structured logger, not a terminal-formatted printer.
func PrintMigrate(_ Opts) {}
