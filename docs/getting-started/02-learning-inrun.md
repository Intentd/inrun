# Learning Inrun

Your first reconciled resource in a minute, then six example packs, each building on the last. Every pack is embedded in the CLI, so `inrun init` copies one into a new directory, and every example can be checked without a cluster (`inrun simulate`) before it runs end to end on [kind](https://kind.sigs.k8s.io/) (`inrun e2e`).

Each stage below says what you learn and where to start. The pack's README
walks you through it; the examples are the source of truth.

```bash
inrun init --list
```

## 0. Your first reconciled resource

`inrun init` with no pack scaffolds the canonical one: a `Website` CRD that
becomes a Deployment and a Service.

```bash
inrun init my-operator && cd my-operator
```

```text
blueprint.yaml        the operator
manifests/crd.yaml    the Website CRD
manifests/cr.yaml     one Website
test/simulate.yaml    checks without a cluster
test/e2e.yaml         checks on kind
```

Check it without a cluster:

```bash
inrun simulate
```

Then run it. `--dev` creates a kind cluster when none is reachable _(requires Docker)_:

```bash
inrun --dev
```

In a second terminal:

```bash
kubectl get website hello-website -o jsonpath='{.status}'; echo
```

```bash
kubectl get deployment,service
```

Edit `blueprint.yaml` (another replica count, a new status field), restart `inrun`, and watch the cluster follow. That loop (edit the Blueprint, simulate, run) is the same in every pack below.

## 1. Declarative: an operator with no code

```bash
inrun init my-website --pack declarative
```

A Blueprint describes what a CR becomes: `01-hello-website` turns a `Website` into a Deployment and a Service and writes its status. The next three add a ServiceAccount, status read from live children, and copying a Secret or ConfigMap across namespaces.

You learn:

- the Blueprint: the CRD it reconciles, `onCreate` / `onReconcile` templates, `emit.status`
- `inrun` runs the runtime against your current cluster
- `inrun simulate` replays reconciles in memory; `inrun e2e` checks the same example on kind

[Declarative pack](https://github.com/intentd/inrun/tree/main/examples/reconcilers/declarative)

## 2. Remote: a reconciler in any language

```bash
inrun init my-webapp --pack remote
```

The reconciler is a small HTTP service (bash, Python or Go in the pack). Inrun calls it once per reconcile with the CR and the Blueprint's `args`, and applies what it returns: status, labels, annotations and resources. Turn `expose` off and watch `prune` delete the Service it stopped returning.

You learn:

- the reconciler contract: one request in, one Result out
- `reconcile.args` shapes what the reconciler sees
- prune, and the finalizing request on delete

[Remote pack](https://github.com/intentd/inrun/tree/main/examples/reconcilers/remote)

## 3. Typed: Go, when you want it

```bash
inrun init my-operator --pack typed
```

Three ways to bring Go: hooks next to a Blueprint, a full reconciler (a
constructor), and an unchanged controller-runtime `Reconcile`. Each is its own
module; `make registry` and `make build` produce an `inrun` with your types
compiled in.

You learn:

- the same Result from Go as from HTTP
- the type registry, and why typed Blueprints need their own build

[Typed pack](https://github.com/intentd/inrun/tree/main/examples/reconcilers/typed)

## 4. Intent: the caller's side

```bash
inrun init my-intent --pack intent
```

A caller sends a flat intent to the gateway, which turns it into a CR; a
Python reconciler reconciles it; the caller reads a view back. The caller
never writes `apiVersion`, `kind` or `spec`, and the reconciler never reads
the CR: the Blueprint maps between them.

You learn:

- `serve`: the gateway's fields, tokens and the view it returns
- `inrun serve play` runs an intent through the gateway's stages locally
- the console

[Intent pack](https://github.com/intentd/inrun/tree/main/examples/intent)

## 5. Migration: change the CRD under live data

```bash
inrun init my-migration --pack migration
```

The CRD behind your callers changes, and the CRs already stored must follow.
`field/` moves values to new paths inside a CRD; `kind/` moves CRs to a new
kind or scope. Every move is snapshotted, so it rolls back, and a value edited
after the migration is never overwritten.

You learn:

- `migrate.from` on a served field: one declaration for the gateway's writes and the backfill
- `migrate.to` and `migrate.selection`; `inrun migrate`, `rollback` and `clean`
- the transitional CRD, and drift on rollback

[Migration pack](https://github.com/intentd/inrun/tree/main/examples/migration)

## 6. Registry: share operators as patterns

```bash
inrun init my-patterns --pack registry
```

Run a published pattern without writing anything, publish your own through
the simulate and e2e gates, then deprecate it so the teams using it move on
deliberately.

You learn:

- `inrun patterns`, `inspect` and running a pattern by name
- `inrun push` and the proof it records on the artifact
- `lifecycle.deprecation`, and accepting it in a Stack

[Registry pack](https://github.com/intentd/inrun/tree/main/examples/registry)

## After the packs

- The [concepts](../concepts/index.md) explain what each pack relied on.
- The [schema reference](../reference/schema/index.md) lists every field.
