<div align="center">
  <img src="./docs/assets/inrun-wordmark.svg" alt="Inrun" height="64" />

  <p><strong>Run intents on Kubernetes.</strong></p>

  <p>
    <a href="https://github.com/intentd/inrun/actions/workflows/ci.yml"><img src="https://github.com/intentd/inrun/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
    <a href="https://github.com/intentd/inrun/releases"><img src="https://img.shields.io/github/v/release/intentd/inrun" alt="Release" /></a>
    <a href="https://artifacthub.io/packages/search?repo=inrun"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/inrun" alt="Artifact Hub" /></a>
    <img src="https://img.shields.io/badge/Go-1.27+-00ADD8.svg" alt="Go 1.27+" />
    <img src="https://img.shields.io/badge/Kubernetes-1.37-326CE5.svg" alt="Kubernetes 1.37" />
    <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License" />
  </p>
</div>

Kubernetes reconciliation is battle-tested and free. Using it is expensive: a purpose-built operator, Kubernetes expertise, and maintenance for as long as it runs. So developers leave it to platform engineers.

Inrun opens it to anyone who has written a service that takes a request and returns a response.

You bring the decision. Inrun brings the machinery.

## How it works

A caller sends an **intent**: a small payload in their own words, with no `apiVersion`, `kind` or `spec`. Inrun turns it into a custom resource, reconciles it, and gives the caller back a **view** of the result.

```console
intent ──▶ gateway ──▶ custom resource ──▶ runtime ──▶ reconcile ──▶ status ──▶ view
```

**The reconciler decides. Inrun writes.** Inrun runs the informers, queues, retries, leader election, server-side apply, owner references, finalizers, status, events, cleanup and metrics. How a resource is reconciled is up to you:

- **Declarative:** describe the resources in a Blueprint. No code.
- **Remote:** any HTTP service, in any language, returns what should exist.
- **Existing service:** an API that knows nothing about Inrun reconciles as it is. Inrun calls it once per reconcile and maps its answer into status.
- **Typed:** Go hooks, a full reconciler, or an unchanged controller-runtime `Reconcile`.

Declarative, remote and typed reconcilers take the same request and return the same result: status, resources, labels and annotations, a requeue, or a cleanup.

## What comes with it

- **Operators as data.** A Blueprint is the operator. It's versioned and published to an OCI registry, carrying its own simulate and e2e results and the tests that produced them. `inrun inspect` shows them before you install anything; `inrun e2e` runs them again on a kind cluster it brings and tears down, so checking the claim needs nothing but Docker.
- **Callers apart from the schema.** Callers send intents; the CRD stays an internal detail. When a field moves, `migrate.from` brings stored resources to the new shape with the same mapping the gateway applies, and callers never notice.
- **Kubernetes as a backend.** A reconciler doesn't have to create anything: the custom resource can be a durable state machine, cleaned up when it's done.

## Install

```bash
brew install intentd/tap/inrun
# or
curl -sSL https://get.inrun.dev | bash
```

## Try it

Two commands to your first operator:

```bash
inrun init     # a hello-website operator in the current directory
inrun          # run it on your current cluster; the console opens on :8081
```

No cluster? Add `--dev` to create a temporary kind cluster (needs Docker). Or check it without one, in memory:

```bash
inrun simulate
```

Or start from a published pattern:

```bash
inrun inspect postgres:v0.1.0                   # version, description, test results
inrun postgres:v0.1.0  --apply-cr --dev         # run it - all batteries included
```

`postgres:v0.1.0` is short for `ghcr.io/intentd/registry/patterns/blueprints/postgres:v0.1.0`. Patterns can live in any OCI registry: set `INRUN_REGISTRY` to use your own. The published ones are built from [inrun-registry](https://github.com/intentd/inrun-registry).

## Examples

Examples come in packs. Copy one into a new project with `inrun init --pack`:

```bash
inrun init --pack declarative   # Start here: operators declared in a Blueprint, no code
inrun init --pack remote        # A reconciler in any language, called over HTTP, including an existing service
inrun init --pack typed         # Go: hooks, a constructor, and a controller-runtime drop-in
inrun init --pack intent        # Intent in through the gateway, view out
```

`inrun init --list` shows them all. Each pack is a folder of examples; for any of them:

```bash
cd declarative/01-hello-website
inrun                      # run it
inrun simulate             # check it without a cluster
inrun simulate --envtest   # the same against a real API server and etcd, still no cluster
inrun e2e                  # test it end to end on a kind cluster it creates and removes
```

Each example's README is its guide.

## Built on

Two libraries that came out of Inrun and stand on their own:

- [KEL](https://github.com/intentd/kel): functions that know Kubernetes, for reading state in templates and Go.
- [kubewright](https://github.com/intentd/kubewright): building and writing an owner's child resources.

## Status

`v0.1.0` is the first release. The API may still change before `v1.0`; breaking changes are listed in [CHANGELOG.md](CHANGELOG.md).

Inrun began as Orkestra. Its history is on the `archive` branch and the `orkestra-v0.7.18` tag.

## Development

```bash
make test
```

See [tests/README.md](tests/README.md) for the test tiers and [docs/contributing](docs/contributing/) for how the code is laid out.

## License

Apache 2.0. See [LICENSE](LICENSE).
