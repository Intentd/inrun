# Inrun documentation

Kubernetes reconciliation is battle-tested and free. Using it is expensive: a purpose-built operator, Kubernetes expertise, and maintenance for as long as it runs. So developers leave it to platform engineers.

Inrun opens it to anyone who has written a service that takes a request and returns a response. You bring the decision. Inrun brings the machinery.

A caller sends an **intent**: a small payload in their own words, with no `apiVersion`, `kind` or `spec`. Inrun turns it into a custom resource, reconciles it, and gives the caller back a **view** of the result.

```text
intent ──▶ gateway ──▶ custom resource ──▶ runtime ──▶ reconcile ──▶ status ──▶ view
```

## Why Inrun

- **One contract, four ways to fill it.** Templates in a **[Blueprint](./concepts/01-blueprint.md)**, an HTTP service in any language, an existing API that knows nothing about Inrun, or Go. Each takes the same request and returns the same Result, so you can change how a resource is reconciled without changing it.
- **Inrun does every write.** Informers, queues, retries, leader election, server-side apply, finalizers, status, events and cleanup are Inrun's; only for kinds the Blueprint declares, with RBAC generated from it.
- **No CRDs of its own.** The only CRDs in your cluster are the ones you defined; the Blueprint is a ConfigMap.
- **Callers never touch Kubernetes.** The gateway takes intents and returns views, in the caller's own words.
- **The schema can change under stored data.** `migrate.from` moves fields to new paths and `migrate.to` moves resources to a new CRD, with rollback, while callers keep sending the same intents.
- **Kubernetes as a backend.** A reconciler doesn't have to create anything: the custom resource can be a durable state machine, cleaned up when it's done.
- **Tested before a cluster.** `inrun simulate` runs the runtime's own reconcile pass in memory; `inrun e2e` runs the same example on kind.
- **Shared as data.** Blueprints are published to a registry with their tests and the proof that they passed.

## Start

- [Install](getting-started/01-install.md)
- [Learning Inrun](getting-started/02-learning-inrun.md): your first reconciled resource, then six example packs

## Concepts

- [The Blueprint](concepts/01-blueprint.md)
- [Reconcilers](concepts/02-reconcilers.md)
- [The runtime](concepts/03-runtime.md)
- [Gateway and intents](concepts/04-gateway.md)
- [The console](concepts/05-console.md)
- [Testing](concepts/06-testing.md)
- [Security](concepts/07-security.md)
- [The registry](concepts/08-registry.md)
- [Migration](concepts/09-migration.md)

## Reference

- [Reference index](reference/index.md): schema, functions and the CLI
- [Resources](reference/schema/resources/index.md): every kind a template can declare
- [Functions](reference/inrun-functions/index.md): what templates can call

## Project

- [Roadmap](roadmap.md)
- [Changelog](https://github.com/intentd/inrun/blob/main/CHANGELOG.md)
- [Contributing](contributing/index.md)
