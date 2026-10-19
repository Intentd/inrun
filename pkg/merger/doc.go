// Package merger resolves Blueprint and Stack files, local or remote, into
// one set of CRD definitions for the runtime. A Blueprint declares CRDs
// directly; a Stack imports Blueprints, OCI patterns, modules and Helm
// sources and merges them. Use New to build a Merger.
package merger
