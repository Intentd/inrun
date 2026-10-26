# Reconcilers

A reconciler decides what a CR should become. Everything else (watching, queueing, retries, applying, pruning, status, events, deletion) belongs to the runtime. Every reconciler takes the same **Request** and returns the same **Result**, so a CRD can change how it is reconciled without changing its API.

## The contract

**In:** the CR (`req.Object`), its rendered `reconcile.args` (`req.Args()`), and whether it is being deleted (`req.Finalizing()`).

**Out:** a Result. The reconciler only declares; the runtime does every write.

| Field | What the runtime does with it |
| --- | --- |
| `status` | Merged into the CR's status, after `emit.status` fields (the Result wins) |
| `resources` | Applied with server-side apply, field manager `inrun-reconciler`; full objects or intent form (`{"type": "deployment", "fields": {...}}`). Each kind must be listed in `reconcile.managedResources`. Objects in the CR's namespace get the CR as owner |
| `labels`, `annotations` | Set on the CR itself; a key dropped from a later Result is removed. `inrun.dev/` keys are reserved |
| `requeue` / `requeueAfter` | Reconcile again after a delay (only after a successful reconcile; failures use retry backoff) |
| `cleanup` | Delete the CR after a grace period; returning no `cleanup` withdraws a pending one |

A failed reconcile returns an error: the runtime records a `ReconcileError`
event and retries with backoff.

## Four kinds

| Kind | Blueprint | You write |
| --- | --- | --- |
| **declarative** | templates under `reconcile.onCreate` / `onReconcile` | nothing: the templates are the reconciler |
| **hooks** | `reconcile.hooks` (location, function) | Go hooks next to the templates, for what YAML does not cover |
| **constructor** | `reconcile.default: false`, `reconcile.constructor` | a full Go `Reconciler` |
| **remote** | `reconcile.default: false`, `reconcile.remote.endpoint` | an HTTP endpoint, in any language |

Hooks and constructors are compiled into your own `inrun` build
(`inrun generate registry`, then build); declarative and remote run on the
stock binary. `/blueprint` and the console show each CRD's kind.

### Remote

Inrun POSTs, once per reconcile:

```json
{ "key": "default/my-webapp",
  "apiVersion": "demo.inrun.dev/v1alpha1", "kind": "WebApp",
  "object": { ... },
  "args": { "image": "nginx:latest" } }
```

plus `target` when the CR came through a serve target, `children` when
`request.children` is set, and `finalizing: true` while the CR is being
deleted. `request.object.exclude` strips paths from `object`. The reply is the
Result as JSON with `"result": "ok"`, or `"result": "error"` and an
`"error"` message. `auth` sends a token from a Secret; `timeout` caps the call.

## Shaping the input

- **`reconcile.args`** map CR fields into the reconciler's own names, with
  templates (`image: "{{ .spec.image }}"`). A reconciler can read only `args`
  and never parse the CR.
- **`reconcile.external`** calls run before the reconciler (HTTP, Prometheus,
  Postgres, Redis and more); their results are `.external.<name>.*` in
  `args` and templates.

## Writes are fenced

The runtime applies only kinds listed in `reconcile.managedResources`, and
the RBAC it runs with is generated from the Blueprint (`inrun generate rbac`).
A reconciler cannot write anything the Blueprint did not declare.

## Per serve target

A CRD served through the gateway can have serve targets, and each target can
carry its own `reconcile` block: templates on one, a remote reconciler or a
constructor on another, each with its own `managedResources`. The caller picks
the target with the intent; the CRD stays one API.
