// Package migrate runs the migrations a blueprint declares on live CRs:
// field-level (serve.fields[*].migrate.from, see package field) and
// kind-level (migrate.to, see package kind). Each supports forward,
// rollback from its snapshot annotation, and clean.
package migrate
