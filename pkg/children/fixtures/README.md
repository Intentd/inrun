# pkg/children fixtures

Living integration fixture for the `forEach` expansion, `custom:` child creation, and child tracking paths in `pkg/children`.

## What each module covers

| Module | Resource types exercised |
|---|---|
| `modules/01-namespaced/` | `Namespace`, `NetworkPolicy`, `ResourceQuota`, `LimitRange` |
| `modules/02-rbac/` | `ClusterRole`, `ClusterRoleBinding`, `Role`, `RoleBinding`, `ServiceAccount` |
| `modules/03-workloads/` | `Deployment`, `StatefulSet`, `ReplicaSet`, `Pod`, `Job`, `CronJob` |
| `modules/04-network/` | `Service`, `Ingress`, `HPA`, `PDB` |
| `modules/05-config/` | `ConfigMap`, `Secret` |
| `modules/06-storage/` | `PersistentVolume`, `PersistentVolumeClaim` |
| `modules/07-custom/` | `TenantPlan` via `custom:` — custom child CR creation |
| `modules/08-plan/` | `ConfigMap` — TenantPlan's own operator output |

Modules 01–06 use `forEach:` to exercise the full expand → name → track path. Module 07 exercises `custom:` — a single child CR (`TenantPlan`) that is itself an Inrun-managed CRD, with its own operator declared in module 08. This covers the full chain: parent creates custom child → child's operator runs → child's resources are asserted and cleaned up.

## Running

Commands must be run from inside this directory.

### Verify expansion in-memory — fast, no cluster needed

```bash
inrun simulate -f pkg/children/fixtures/simulate.yaml
```

### Full cluster test 
```bash
inrun e2e -f pkg/children/fixtures/e2e.yaml --workers 3
```

Creates a kind cluster, applies the CR, asserts all resources

## Adding a new child resource type

When you add a new resource type to `pkg/children/` (new `ExpandForEach*`, GVR, name helper, and tracking block):

1. **Add it to the appropriate module** in `modules/` — or create a new module if it belongs to a new group.
2. **Run `inrun validate`** — confirms the schema is correct.
3. **Add an op to `simulate.yaml`** — one `verb: create` entry for the new resource type.
4. **Run `inrun simulate`** — confirms expansion fires in-memory.
5. **Add assertions to `e2e/01-resources.yaml`** — one entry per expanded name.
6. **Add a row to the table above** so the coverage map stays accurate.
