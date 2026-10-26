# registry

A pattern is an operator packaged for others: a Blueprint, its CRD, a sample CR, and the proof that it works. The
registry is where patterns are shared, as OCI artifacts in any container registry.

What makes a pattern worth trusting is that the proof travels with it. `inrun push` runs the pattern's simulate and
e2e gates before anything is published and records the results on the artifact, so a consumer sees what was tested,
and when, before pulling anything.

## Steps

Three folders, one lifecycle. Work through them in order:

- **[consume](consume/README.md)**: run a published pattern. No CRD, reconciler or tests to write.
- **[publish](publish/README.md)**: build your own pattern on a published module, prove it, push it.
- **[deprecate](deprecate/README.md)**: retire your pattern. Consumers keep running it only by accepting that on
  purpose.

## Before you begin

**A cluster** for running patterns. Any cluster in your kubeconfig works; without one, add `--dev` to the first
`inrun` command and it creates a local kind cluster.

**A registry you can push to**, from the publish step on. Consuming needs no account: public patterns come from
`ghcr.io/intentd/registry` by default.
