// Package migrate runs the migrations a blueprint declares on live CRs
// through KOME (github.com/intentd/kome): field level (serve.fields[*].
// migrate.from) and kind level (migrate.to), forward, rollback from the
// snapshot annotations, and clean. It keeps Inrun's annotation prefix
// (inrun.dev), field manager and template functions, and Inrun's result
// types for the CLI.
package migrate
