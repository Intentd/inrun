# pkg/runtime/reconciler/fixture

Living integration fixture for the reconciler's resource types.

## Why this exists

The runner functions in `pkg/runner/` apply Kubernetes resources from blueprint
templates. There is no meaningful way to unit-test them: the logic that matters
— template rendering, server-side apply, status propagation, condition evaluation
— only works against a real API server. Mocking it tests the mock, not Inrun.

This fixture is the right vehicle. It declares a `ReconcilerSuite` CRD (namespaced,
in `default`) and a `ReconcilerProbe` CRD (cluster-scoped). A single `ReconcilerSuite`
CR is the test entry point: its operator creates a `ReconcilerProbe` via `custom:`, and
the probe's operator then triggers every supported resource block in one reconcile cycle.

The scope split is intentional: Kubernetes GC cannot cascade owner references from a
namespaced owner to a cluster-scoped dependent, so deleting the suite exercises the
explicit `runners.DeleteOwnedClusterScopedResources` path. The probe is cluster-scoped
so it can own resources in `probe-ns` without cross-namespace owner reference issues.

Failures surface as a missing resource, a wrong status field, or an operator crash, all
observable without reading code.

## What each block covers

| Block in `blueprint.yaml`        | What it exercises                          |
|--------------------------------|--------------------------------------------|
| `custom:` (ReconcilerSuite)    | Namespaced CR creating a cluster-scoped child; exercises explicit GC on deletion |
| `validation.rules`             | Admission validation                       |
| `mutation.rules`               | Admission mutation (default injection)     |
| `namespaces`                   | Namespace creation                         |
| `secrets` (`once: true`)       | Secret generation, idempotency             |
| `configMaps`                   | ConfigMap with template values             |
| `networkPolicies`              | Inline deny-all NetworkPolicy              |
| `resourceQuotas`               | Inline ResourceQuota with hard limits      |
| `limitRanges`                  | Container default limits                   |
| `clusterRoles`                 | Cluster-scoped RBAC role                   |
| `clusterRoleBindings`          | Cluster-scoped RBAC binding                |
| `serviceAccounts`              | ServiceAccount creation                    |
| `roles`                        | Namespaced RBAC role                       |
| `roleBindings`                 | Namespaced RBAC binding                    |
| `replicaSets`                  | ReplicaSet creation                        |
| `deployments` (main)           | Deployment with env + serviceAccountName   |
| `deployments` with `when:`     | Conditional resource (`tier=premium` only) |
| `services`                     | Service with port mapping                  |
| `jobs`                         | One-shot Job                               |
| `cronJobs`                     | CronJob with schedule                      |
| `statefulSets`                 | StatefulSet creation                       |
| `persistentVolumes`            | Cluster-scoped PV with hostPath            |
| `persistentVolumeClaims`       | PVC creation                               |
| `ingresses`                    | Ingress with host routing                  |
| `hpa`                          | HorizontalPodAutoscaler targeting probe-app |
| `pdb`                          | PodDisruptionBudget targeting probe-app    |
| `pods`                         | Direct Pod creation                        |
| `status.fields`                | Status propagation and template evaluation |

## Running

Run simulate first — it exercises the real reconciler in-memory and is fast:

```bash
inrun simulate -f pkg/runtime/reconciler/fixture/simulate.yaml
```

For full cluster verification:

```bash
inrun e2e -f pkg/runtime/reconciler/fixture/e2e.yaml --workers 3
```

`inrun e2e` creates the kind cluster, installs Inrun, applies the CRD and CR,
runs all assertions, and tears down the cluster.

To reuse an existing cluster during iteration:

```bash
inrun e2e -f pkg/runtime/reconciler/fixture/e2e.yaml --use-current
```

## Adding a new resource type

1. Add a block to the appropriate fixture module in
   [pkg/runtime/reconciler/fixture/modules/](./modules/). Name resources
   `{{ .metadata.name }}-<type>` to avoid collisions.
2. Add a row to the table above.
3. Add a `create` op to [simulate.yaml](./simulate.yaml) for the new resource type.
4. Add a `resources:` assertion to the `All resources created` checkpoint in
   [e2e/01-resources.yaml](./e2e/01-resources.yaml).
5. Run `inrun e2e -f pkg/runtime/reconciler/fixture/simulate.yaml` locally before opening the PR.
