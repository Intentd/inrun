<div align="center">
  <img src="./docs/assets/inrun-wordmark.svg" alt="Inrun" height="64" />

  <p><strong>Run intents on Kubernetes.</strong></p>

  <p>
    <a href="https://github.com/intentd/inrun/releases"><img src="https://img.shields.io/github/v/release/intentd/inrun" alt="Release" /></a>
    <a href="https://artifacthub.io/packages/search?repo=inrun"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/inrun" alt="Artifact Hub" /></a>
    <img src="https://img.shields.io/badge/Go-1.26+-00ADD8.svg" alt="Go" />
    <img src="https://img.shields.io/badge/Kubernetes-1.28+-326CE5.svg" alt="Kubernetes" />
    <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License" />
  </p>
</div>

A caller sends an **intent**: a small JSON payload in their own words, with no
`apiVersion`, `kind` or `spec`. Inrun turns it into a custom resource,
reconciles it, and gives the caller back a **view** of the result.

```console
intent ──▶ gateway ──▶ custom resource ──▶ runtime ──▶ reconcile ──▶ status ──▶ view
```

How a resource is reconciled is up to you:

- **Declarative:** describe the resources in a Blueprint; no code.
- **Remote:** any HTTP endpoint, in any language, returns what should exist.
- **Typed:** Go hooks, a full reconciler, or an unchanged controller-runtime `Reconcile`.

Inrun runs the informers, queues, workers, retries, leader election, status,
cleanup and metrics.

> **Status:** pre-release. The first release, `v0.1.0`, is in preparation, and
> the API may still change before `v1.0`.

## Try it

Build the CLI from source (Go 1.26+):

```bash
go build -o inrun ./cmd/inrun
```

Then run the first example against a cluster in your current kubeconfig:

```bash
cd examples/reconcilers/declarative/01-hello-website
```

```bash
inrun
```

## Examples

| Pack | Shows |
| --- | --- |
| [declarative](examples/reconcilers/declarative/) | Operators declared in a Blueprint. Start here. |
| [remote](examples/reconcilers/remote/) | A reconciler in any language, called over HTTP |
| [typed](examples/reconcilers/typed/) | Go hooks, a constructor, and a controller-runtime drop-in |
| [intent](examples/intent/) | Intent in through the gateway, view out |

Each example can be checked without a cluster (`inrun simulate`) and end to
end on kind (`inrun e2e`).

## Development

```bash
make test
```

See [tests/README.md](tests/README.md) for the test tiers and
[docs/contributing](docs/contributing/) for how the code is laid out.

## History

Inrun began as Orkestra. Its history is on the `archive` branch and the
`orkestra-v0.7.18` tag.

## License

Apache 2.0. See [LICENSE](LICENSE).
