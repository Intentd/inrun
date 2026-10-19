// Package generate turns a Blueprint into files a project or cluster needs.
//
// TypeRegistry writes the Go type registry that typed operators compile
// against. RBAC, ConfigMap and RenderBundle produce the cluster install
// manifests (permissions, the embedded Blueprint, or both in one file).
// BlueprintScaffold and the Write* helpers scaffold a new pattern:
// blueprint.yaml, Makefile, Dockerfile, README, simulate.yaml and e2e.yaml.
package generate
