# Changelog

Inrun follows [Semantic Versioning](https://semver.org/). Until `v1.0`, minor versions may contain breaking changes.

## Unreleased

Inrun is the intent runner that began as Orkestra. This first release narrows it to one loop: an intent arrives through the gateway, becomes a custom resource, is reconciled, and returns to the caller as a view.

### Highlights

- **One reconciler contract.** Hooks, constructors and remote reconcilers now receive the same request and return the same result: status, resources, labels and annotations, a requeue, or a cleanup. Inrun performs every write and checks each resource against the Catalog's `managedResources` first. Declarative reconcilers keep creating resources from their templates.
- **Args in the reconciler's own vocabulary.** `reconcile.args` maps fields of the custom resource onto the names your reconciler uses, rendered on every reconcile. `reconcile.external` calls run beforehand, so args can carry their results.
- **Cleanup from code.** A reconciler can ask for its resource to be deleted after a grace period. Requeues and cleanup are independent; whichever is due first acts, and the reconciler keeps running in between.
- **Typed Catalogs through the stock gateway.** The gateway no longer needs compiled types, so one gateway image serves any Catalog.

### Upgrading from Orkestra

Resources created by Orkestra are not adopted. Rename as follows, then
redeploy.

| Orkestra | Inrun |
| --- | --- |
| Go module, CLI, console | `github.com/inrundev/inrun`, `inrun`, `inrun-console` |
| Images | `ghcr.io/inrundev/inrun`, `inrun-gateway`, `inrun-console` |
| Helm chart | `inrun`, starting at `0.1.0` |
| API group, labels, annotations, finalizers | `inrun.dev` |
| Environment variables | `INRUN_*` |
| `Katalog`, `Komposer`, `Motif` | `Catalog`, `Stack`, `Module` |
| `inrun run` | `inrun` with no subcommand |
| Control Center | `inrun console` |
| `hooks.args`, `constructor.args`, `remote.args` | `reconcile.args` |
| `managedResources` under `hooks`, `constructor` or `remote` | `reconcile.managedResources` |
| `hooks.external` | `reconcile.external` |
| `reconcile.remote.payload` | `reconcile.remote.request` |
| `runtime.cleanup.deleteAfter` | `runtime.cleanup.after` |

**Typed reconcilers (Go):**

- Hooks have a new signature: `OnReconcile(ctx, req, obj) (domain.Result, error)`, and the same for
  `OnDelete`. `OnNotFound` is removed.
- Read args from `req.Args`. `kube.Args()`, `WithArgs` and `ScopedFor` are removed; `kubeclient.Args` is now `domain.Args`, and `BindArgs` is `Bind`.
- `domain.Result.StatusPatch` is now `Status`.

**Remote reconcilers:**

- `result` is `ok` or `error`. Ask for a requeue with the `requeue` field (`{"after": "60s", "reason": "…"}`) or the `requeueAfter` shorthand.
- Return resources instead of applying them. Inrun applies them under the field manager `inrun-reconciler`, which was `inrun-remote`.
- The request body is `key`, `apiVersion`, `kind`, `target`, `object`, `args` and `children`. `gvk` is replaced by `apiVersion` and `kind`, `children` moves up from `prepared`, and `prepared` is gone: `object` is the only copy of the CR, so `request.object.exclude` hides what it names.

**Helm:** point `catalog.existingConfigMap` at the ConfigMap from `inrun generate bundle`. The inline Catalog values are removed.

### Added

- `inrun` also starts the console on :8081, and the gateway on :8082 when the Catalog serves a CRD, so one command runs the whole loop locally; the console can create resources through that gateway. Set `INRUN_CONSOLE_PORT` or `INRUN_GATEWAY_PORT` to move them. Either one failing to start is logged; the runtime runs regardless.
- The console honours `--log-level`; it printed every request and fetch.
- The gateway's `/api/v1/apply` takes YAML as well as JSON, like an intent file for `inrun serve play`. Both decode to the same values, so an intent builds the same CR in either format.
- A Simulate spec can name an intent instead of a CR (`intent`, with the `token` it is sent with): `inrun simulate` builds the CR the way the gateway does, then reconciles it. `inrun serve play` takes `--envtest`, and `--envtest` installs the Catalog's `crdFile`s when a spec lists no CRDs.
- Reconcilers can return labels and annotations for their resource. Inrun applies them with server-side apply: a key the reconciler stops returning is removed, and keys set by anyone else are left alone. The `inrun.dev` domain is reserved.
- Requeues carry a reason, shown in the logs.
- Typed reconcilers can return resources, in full or intent form.
- Intent form covers `namespace` and `pdb`, besides `deployment`, `statefulset`, `service`, `ingress`, `configmap`, `secret`, `serviceaccount`, `job`, `cronjob` and `custom`.
- `inrun simulate` runs remote reconcilers against their endpoint.
- `inrun e2e --chart` installs a local chart, to test unreleased changes end to end.
- `inrun generate bundle`, `configmap` and `rbac` accept Catalogs that use `crdFile`.
- One project layout: `catalog.yaml` at the root, manifests in `manifests/`, and tests in `test/`. `inrun init`, `create pattern` and `migrate` write it, and `inrun simulate` and `inrun e2e` read it by default.
- Examples in four packs: declarative, remote, typed and intent.

### Changed

- `reconcile.args`, `reconcile.managedResources` and `reconcile.external` are rejected on declarative-only CRDs.
- Constructor args are rendered as templates, like hook args.
- A serve target's `args` merge over the CRD's; its `managedResources` and `external` replace them.

### Removed

- `inrun validate --play`: a Simulate spec that gets its CR from an intent says so with `spec.intent`.
- Features that don't serve the intent loop. They remain on the `archive` branch.

### Fixed

- A reconciler that returns resources was called again after every reconcile: each apply stamped the resources' `inrun.dev/managed-since` with the current time, updating them. They now carry their owner's value, so an unchanged resource is left untouched.
- Validation and mutation rules can target label and annotation keys with dots: escape them, as in `metadata.labels.app\.kubernetes\.io/name`. Such keys used to be split into nested fields.
- Admission mutations patch keys containing `/` (such as `example.com/owner`) and objects without a `labels` or `annotations` map.
- A serve target's own `onCreate`, `onReconcile` and `onDelete` templates were ignored when the CRD also had a `reconcile` block.
- `inrun simulate` ignored the Simulate's `target`, so per-target hooks never ran.
- `inrun simulate` dropped the status a reconciler returned.
- `inrun e2e` repeated mutations such as `patch` and `scale` on every retry, and its teardown could hang on a resource whose controller was gone.

## Earlier versions

Inrun's history before the rename is Orkestra's: see the `archive` branch and
the `orkestra-v0.7.18` tag.
