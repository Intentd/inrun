# The Blueprint

A Blueprint is one YAML file that describes an operator: the CRDs it reconciles and what each of their CRs becomes. Inrun installs no CRDs of its own; in a cluster the Blueprint is a ConfigMap (`inrun generate configmap`), so the only CRDs in the API server are the ones you defined.

```yaml
apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: hello-website
  version: 0.1.0
spec:
  crds:
    website:
      crdFile: ./manifests/crd.yaml
      operatorBox:
        reconcile:
          onCreate:
            deployments:
              - name: "{{ .metadata.name }}"
                image: "{{ .spec.image }}"
        emit:
          status:
            fields:
              - path: phase
                value: Running
```

## Top level

| Key | Holds |
| --- | --- |
| `metadata` | name, version, author, description, tags |
| `spec.crds` | one entry per CRD, keyed by a short name |
| `spec.imports` | Modules whose resources this Blueprint takes in |
| `spec.finalizers` | finalizers added to every CR, besides Inrun's own |
| `imports` | other Blueprints and Stacks to merge in: from the registry, files or Helm |
| `functions` | your own template functions, beside the built-in ones |
| `presets` | named settings (resources, probes, autoscaling, PDBs…) templates can refer to |
| `security` | deletion and namespace protection, admission and conversion webhooks |
| `gateway` | the gateway's endpoint, API, tokens and clusters |
| `lifecycle` | maturity, compatibility and deprecation, for registry consumers |
| `policy` | what a Stack accepts from its imports (`policy.lifecycle.minMaturity`) |
| `publish` | what `inrun push` publishes to a registry |

## A CRD entry

| Key | Holds |
| --- | --- |
| `crdFile` / `apiTypes` | the CRD: a manifest, or group, version, kind and plural (`apiTypes.location` makes it typed) |
| `operatorBox` | how CRs are reconciled, in five blocks (below) |
| `serve` | how callers reach it through the gateway |
| `admission` | validation, mutation and conversion rules |
| `migrate` | moving stored CRs to another CRD (`migrate.to`), scoped by `migrate.selection`; a field's move to a new path is `serve.fields[*].migrate.from` ([Migration](09-migration.md)) |
| `dependsOn` | CRDs that must be ready first |
| `labels`, `annotations` | set on the CRD's CRs |
| `enabled` | turn an entry off without deleting it |

The `operatorBox` blocks, in the order a reconcile meets them:

| Block | Decides |
| --- | --- |
| `observe` | what else triggers a reconcile: watched kinds, Kubernetes events, cross-CRD reads |
| `preReconcile` | whether to reconcile now (`when` / `or` gates) |
| `runtime` | allowed and restricted namespaces, finalizers, deletion protection, worker autoscale, cleanup |
| `reconcile` | the reconciler (templates, hooks, a constructor or a remote endpoint) with `args`, `external`, `managedResources`, `prune`, `requeue`, and its workers, queue and resync |
| `emit` | status fields and events written back to the CR |

## Templates and functions

Any string value can be a Go template over the CR (`{{ .spec.image }}`), the results of `reconcile.external` calls (`.external.<name>`), cross-CRD reads, and runtime context (`.inrun.namespace`). Functions are what those templates can call: Kubernetes-aware helpers (pod readiness, job completion, HPA, quantities, cron, semver…) from [kel](https://github.com/intentd/kel), and your own from `functions:`. `inrun functions` lists and searches them.

## Validation

`inrun validate` checks a Blueprint block by block, in the order above, and prints the RBAC it needs with `--full`. The runtime and the gateway run the same validation at startup; a Blueprint that fails it never starts.

## Blueprints, Stacks and Modules

- A **Module** is a reusable set of resources with named inputs, which a Blueprint imports into a CRD's reconciler.
- A **Stack** composes several Blueprints and Modules into one operator.
- `inrun push` and `inrun pull` move them through a [registry](08-registry.md).
