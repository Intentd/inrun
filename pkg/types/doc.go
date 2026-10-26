// Package types holds the shared structs, interfaces and registries of the
// runtime, gateway and CLI: the Blueprint spec, CRD entries, resource
// templates, admission and autoscale settings. It is kept in one package to
// avoid import cycles; callers import it as types.
package types
