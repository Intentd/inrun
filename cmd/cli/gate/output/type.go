//go:build !runtime

package output

import "github.com/intentd/inrun/pkg/migrate"

// Opts carries the data needed by every gate output printer.
type Opts struct {
	Results    []*migrate.KindMigrateResult
	DryRun     bool
	Clean      bool
	Verbose    bool
	Limit      int
	Paths      []string
	Namespaces []string
	Name       string
	Kind       string
	Selector   string
}
