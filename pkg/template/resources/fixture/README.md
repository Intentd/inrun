# pkg/template/resources/fixture

Living integration fixture for every `pkg/resources/*` package.

## Why this exists

The `Resolve()` + `Update()`/`Create()` functions in `pkg/resources/` only do something meaningful against a real API server. Unit tests can verify field mapping, but drift detection, idempotency, and k8s default-injection behavior can only be observed live. Mocking the API server tests the mock, not the resource package.

This fixture uses a typed Go hook to call every resource package in a single reconcile cycle. Failures surface as a missing resource, a wrong field, or an operator crash — without reading code.

## What this is

This directory is a **standalone Go module** (`go.mod`) — a minimal typed Inrun operator that imports `github.com/intentd/inrun-resource-probe` for its CRD types and hook implementation. The hook exercises all 23 resource packages in one reconcile loop.

The standard `inrun` CLI does not know about `ResourceProbe`. You must build a custom binary first.

## Coverage

| Resource package          | What it exercises                                         |
|---------------------------|-----------------------------------------------------------|
| `namespaces`              | Namespace creation and reconcile                          |
| `configmaps`              | ConfigMap with template-rendered values                   |
| `secrets`                 | Secret with StringData — StringData→Data round-trip       |
| `serviceaccounts`         | ServiceAccount creation (onCreate)                        |
| `clusterroles`            | Cluster-scoped RBAC role, drift guard on Rules            |
| `clusterrolebindings`     | Cluster-scoped RBAC binding, drift guard on Subjects      |
| `roles`                   | Namespaced RBAC role                                      |
| `rolebindings`            | Namespaced RBAC binding                                   |
| `networkpolicies`         | Inline deny-all NetworkPolicy                             |
| `resourcequotas`          | ResourceQuota with hard limits                            |
| `limitranges`             | Container default limits                                  |
| `deployments`             | Deployment with env + serviceAccountName                  |
| `statefulsets`            | StatefulSet creation                                      |
| `replicasets`             | ReplicaSet creation                                       |
| `pods`                    | Direct Pod — declared-intent guards for SA token volume   |
| `jobs`                    | One-shot Job (onCreate)                                   |
| `cronjobs`                | CronJob with schedule, command/args drift guard           |
| `services`                | Service with port mapping                                 |
| `ingresses`               | Ingress with host routing                                 |
| `hpas`                    | HorizontalPodAutoscaler                                   |
| `pdbs`                    | PodDisruptionBudget                                       |
| `pvcs`                    | PersistentVolumeClaim creation                            |
| `pvs`                     | Cluster-scoped PersistentVolume                           |

## Running

### Step 1 — Generate the registry and build the custom binary

```bash
cd pkg/template/resources/fixture
make registry
make build
```

`make registry` runs `inrun generate registry --file blueprint.yaml` to produce the type registry and entrypoint that includes `ResourceProbe`. `make build` compiles a custom `inrun` binary with it. The standard `inrun` cannot run this fixture without this step.

### Step 2 — Verify in-memory

```bash
inrun simulate -f pkg/template/resources/fixture/simulate.yaml
```

Fast — no cluster needed. Exercises the reconciler logic against a fake API server.

### Step 3 — Full cluster verification

`e2e.yaml` references `values.yaml` which sets the runtime image to `ghcr.io/intentd/inrun/pkg/template/resources/fixture:latest`. To use a locally built image instead:

```bash
make docker push IMAGE_REPO=yourregistry/resource-probe IMAGE_TAG=latest
```

Then run e2e:

```bash
inrun e2e -f pkg/template/resources/fixture/e2e.yaml --workers 3 \
  --set runtime.image.repository=yourregistry/resource-probe \
  --set runtime.image.tag=latest
```

Or with the default published image (no `--set` needed):

```bash
inrun e2e -f pkg/template/resources/fixture/e2e.yaml --workers 3
```

Creates a kind cluster, installs Inrun with the resource-probe runtime, applies the CR, asserts all resources, and tears down.

To reuse an existing cluster during iteration:

```bash
inrun e2e -f pkg/template/resources/fixture/e2e.yaml --use-current
```

## Adding a new resource type

1. Add the `Resolve()` + `Update()`/`Create()` call to `hooks/resource_hooks.go`.
2. Add a row to the coverage table above.
3. Add a `create` op to `simulate.yaml` for the new resource type.
4. Add a `resources:` assertion to `e2e/01-resources.yaml`.
5. Run `inrun simulate` locally before opening the PR.
