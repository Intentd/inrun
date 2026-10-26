# Concepts

What the example packs rely on, one idea per page.

| Page | The idea |
| --- | --- |
| [The Blueprint](01-blueprint.md) | One file describes an operator: its CRDs and what each becomes |
| [Reconcilers](02-reconcilers.md) | One contract, four ways to fill it: declarative, hooks, constructor, remote |
| [The runtime](03-runtime.md) | The process that reconciles, and what it does around every pass: finalizers, apply, prune, events, deletion |
| [Gateway and intents](04-gateway.md) | Callers send intents in their own words and read back a view |
| [The console](05-console.md) | A web view of running runtimes, and a form that sends intents through the gateway |
| [Testing](06-testing.md) | `simulate` without a cluster, `envtest` against an API server, `e2e` on kind |
| [Security](07-security.md) | RBAC from the Blueprint, TLS, admission and conversion webhooks, protection |
| [The registry](08-registry.md) | What is distributed: patterns that carry their own proof, and their lifecycle |
| [Migration](09-migration.md) | Changing the CRD under stored CRs: fields to new paths, CRs to a new CRD, with rollback |
