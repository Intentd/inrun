// Package cli builds the command-line binary. Each command group lives in
// its own subpackage and registers itself on the root command defined in
// cmdutil; this package only selects which groups are compiled in.
//
// Build tags choose the binary:
//   - no tag: the developer CLI, with every command (commands_dev.go)
//   - runtime: only the run command, for the runtime image (commands_runtime.go)
//   - gateway: only the gate command, for the gateway image (commands_gateway.go)
package cli
