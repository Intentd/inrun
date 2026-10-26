//go:build !runtime

package migrate

import (
	"fmt"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/intentd/inrun/pkg/util/term"
	"github.com/spf13/cobra"
)

// progress shows a spinner while a run works through CRs. It is nil, and
// every method a no-op, unless output is text to a terminal.
type progress struct {
	verb string
	spin *term.Spinner
}

func newProgress(cmd *cobra.Command, mode migrate.MigrateMode) *progress {
	if out, _ := cmd.Flags().GetString("output"); out != "text" || !term.IsTerminal() {
		return nil
	}
	verb := "Migrating"
	switch mode {
	case migrate.ModeRollback:
		verb = "Rolling back"
	case migrate.ModeClean:
		verb = "Cleaning"
	}
	return &progress{verb: verb}
}

func (p *progress) start(msg string) {
	if p == nil {
		return
	}
	p.stop()
	p.spin = cmdutil.StartSpinner(msg)
}

// begin starts the spinner for the run itself.
func (p *progress) begin() {
	if p != nil {
		p.start(p.verb + "…")
	}
}

func (p *progress) update(label string, done, total int) {
	if p == nil || p.spin == nil {
		return
	}
	p.spin.Update(fmt.Sprintf("%s %s  %d/%d", p.verb, label, done, total))
}

func (p *progress) stop() {
	if p == nil || p.spin == nil {
		return
	}
	p.spin.Stop()
	p.spin = nil
}

// callback returns the migrator Progress hook, or nil when there is no spinner.
func (p *progress) callback() func(string, int, int) {
	if p == nil {
		return nil
	}
	return p.update
}
