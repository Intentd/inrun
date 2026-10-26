# Testing

Every example ships its tests next to it, in `test/`. Three levels, from no cluster to a real one.

## simulate: no cluster

```bash
inrun simulate
```

Runs the runtime's own reconcile pass (gates, apply, prune, finalizing, status) against an in-memory cluster, for a number of cycles, and records every write. A `simulate.yaml` turns that into assertions:

```yaml
apiVersion: inrun.dev/v1
kind: Simulate
metadata:
  name: hello-website-sim
spec:
  blueprint: ../blueprint.yaml
  cr: ../manifests/cr.yaml
  cycles: 5
  expect:
    steady: true
    noErrors: true
    ops:
      - {cycle: 1, verb: apply, resource: deployments, name: hello-website}
    absent:
      - {cycle: 3, verb: delete, resource: deployments}
```

- `inrun simulate ./...` runs every `simulate.yaml` below the current directory.
- A remote reconciler is called for real on each cycle, so start it first.
- `--envtest` runs the same thing against a local kube-apiserver and etcd, for behaviour the in-memory cluster cannot show (defaulting, validation by the API server).
- `--dev-server` starts a mock HTTP server for testing `reconcile.external` calls.

Because simulate runs the same pass as the cluster, a passing simulation exercises the real code path, not a copy of it.

## e2e: a kind cluster

```bash
inrun e2e
```

Creates a kind cluster, installs Inrun with its Helm chart, applies the CRD and CR, and checks an `e2e.yaml`: steps that run after the CR is applied or deleted and assert on resources (`count`, `ready`), `kubectl` reads (`field` with `equals`, `jq`, …) or commands (`exitCode`, and `contains`, `regex`, `equals`… on the output).

```yaml
apiVersion: inrun.dev/v1
kind: E2E
metadata:
  name: hello-website-e2e
spec:
  blueprint: ../blueprint.yaml
  crd: ../manifests/crd.yaml
  cr: ../manifests/cr.yaml
  expect:
    - name: Deployment ready
      after: cr-applied
      timeout: 60s
      resources:
        - {kind: Deployment, name: hello-website, namespace: default, ready: true}
    - name: Cleaned up
      after: cr-deleted
      timeout: 30s
      resources:
        - {kind: Deployment, name: hello-website, namespace: default, count: 0}
```

- `imports:` runs several suites in one cluster; `freshCluster: true` gives one its own, and `wait:` pauses between suites (for webhook deregistration or namespace termination).
- `--use-current` runs against the current kubeconfig context instead of creating a cluster.

## Checks without running anything

| Command | Checks |
| --- | --- |
| `inrun validate` | a Blueprint, Stack, Module, E2E or Simulate file; `--full` prints the RBAC it needs |
| `inrun serve play` | an intent through the gateway's stages, printing the CR it becomes |
| `inrun gate` | a CR against the CRD's admission rules |
| `inrun template` | the fully expanded Blueprint the runtime would run |
