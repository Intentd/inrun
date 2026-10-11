# Contributing to pkg/resources

The registry is the library of built-in Kubernetes resource handlers Inrun knows how to create, apply (SSA), and delete on behalf of an operator. Each resource type lives in its own subdirectory.

---

## What exists

| Directory | Resource | Notes |
|-----------|----------|-------|
| `deployments/` | `apps/v1 Deployment` | SSA |
| `services/` | `v1 Service` | SSA |
| `configmaps/` | `v1 ConfigMap` | SSA |
| `secrets/` | `v1 Secret` | SSA |
| `statefulsets/` | `apps/v1 StatefulSet` | SSA |
| `jobs/` | `batch/v1 Job` | Create-only |
| `cronjobs/` | `batch/v1 CronJob` | SSA |
| `replicasets/` | `apps/v1 ReplicaSet` | SSA |
| `ingresses/` | `networking.k8s.io/v1 Ingress` | SSA — needs tests |
| `hpas/` | `autoscaling/v2 HPA` | SSA — needs tests |
| `pdbs/` | `policy/v1 PodDisruptionBudget` | SSA with immutable-selector fallback — needs tests |
| `pvcs/` | `v1 PersistentVolumeClaim` | Create-only — needs tests |
| `pvs/` | `v1 PersistentVolume` | SSA (cluster-scoped) — needs tests |
| `namespaces/` | `v1 Namespace` | SSA — needs tests |
| `roles/` | `rbac/v1 Role` | SSA — needs tests |
| `rolebindings/` | `rbac/v1 RoleBinding` | SSA with immutable-roleRef fallback — needs tests |
| `clusterroles/` | `rbac/v1 ClusterRole` | SSA (cluster-scoped) — needs tests |
| `clusterrolebindings/` | `rbac/v1 ClusterRoleBinding` | SSA with immutable-roleRef fallback (cluster-scoped) — needs tests |
| `serviceaccounts/` | `v1 ServiceAccount` | Create-only — needs tests |
| `networkpolicies/` | `networking.k8s.io/v1 NetworkPolicy` | SSA — needs tests |
| `resourcequotas/` | `v1 ResourceQuota` | SSA — needs tests |
| `limitranges/` | `v1 LimitRange` | SSA — needs tests |
| `pods/` | `v1 Pod` | SSA with immutable-spec fallback — needs tests |
| `customresources/` | Dynamic CR via `dynamic` client | Create-only |

---

## What needs implementing

The following resource types are declared in `pkg/types/types_hook_templates.go` as `PlaceholderSource` — they are accepted in YAML but not yet executed. Implementing one means building the full handler package.

**Workloads:**
- `DaemonSets`
- `PodTemplates`

**Scheduling:**
- `PriorityClasses`
- `PriorityLevelConfigurations`
- `RuntimeClasses`

**Storage:**
- `StorageClasses`
- `StorageLocations`
- `StoragePools`
- `StorageBackups`
- `StorageSnapshots`
- `StorageVolumes`

**Monitoring:**
- `ServiceMonitors` (Prometheus Operator CRD)

**Other:**
- `Volumes` (injected into pod specs)
- `VolumeMounts` (injected into pod specs)
- `PodSecurityPolicies`

When you implement one, remove it from the `PlaceholderSource` block in `pkg/types/types.go` and replace it with a proper typed slice.

---

## How to add a resource type

Follow the pattern of any complete resource (e.g., `deployments/`):

### 1. Create the directory

```text
pkg/resources/<resourcename>/
  <resourcename>.go   — Create, Apply, Update, Delete, Resolve functions
  types.go            — ResolvedSpec struct
```

### 2. Implement five functions

```go
func Create(ctx context.Context, kube kubeclient.KubeClient, owner domain.Object, spec ResolvedSpec) error
func Apply(ctx context.Context, kube kubeclient.KubeClient, owner domain.Object, spec ResolvedSpec) error
func Update(ctx context.Context, kube kubeclient.KubeClient, owner domain.Object, spec ResolvedSpec) error
func Delete(ctx context.Context, kube kubeclient.KubeClient, owner domain.Object, spec ResolvedSpec) error
func Resolve(src types.MyResourceSource, ownerName string) ResolvedSpec
```

**`Create`** — checks existence, calls the k8s Create API if absent. No-op if the resource already exists. Used for `onCreate:` hooks.

**`Apply`** — Server-Side Apply via `ApplyPatchType`. Build the object, set `TypeMeta` (`APIVersion` + `Kind`), marshal to JSON, then patch:

```go
obj.TypeMeta = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"}
body, _ := json.Marshal(obj)
kube.Clientset().AppsV1().Deployments(ns).Patch(
    ctx, spec.Name, k8stypes.ApplyPatchType, body,
    metav1.PatchOptions{FieldManager: config.FieldManagerRuntime, Force: utils.BoolPtr(true)},
)
```

If the resource has immutable fields (pods, pdbs), catch `errors.IsInvalid` and fall back to `Delete` + `Create`.

**`Update`** — delegates to `Apply`. Kept so existing callers do not need to change.

**`Delete`** — no-op if the resource does not exist.

### 3. Add a resolver in `template/resolver.go`

```go
func (r *Resolver) ResolveMyResourceTemplate(src *types.MyResourceSource) (*myresource.ResolvedSpec, error)
```

Use `r.Resolve(expr)` for any field that may contain a `{{ .spec.something }}` expression.

### 4. Write the runner in `pkg/runtime/runners/`

Create `pkg/runtime/runners/myresources.go` — see [pkg/runners/docs/01-runner-contract.md](https://github.com/intentd/inrun/blob/main/pkg/runtime/runners/docs/01-runner-contract.md) for the canonical shape. Then wire it into `pkg/runtime/reconciler/run_template_reconcile.go` via `runners.RunMyResources(...)` and add `expandForEachMyResources` to `run_foreach.go`.

The full end-to-end walkthrough is in [pkg/reconciler/docs/07-adding-a-resource.md](https://github.com/intentd/inrun/blob/main/pkg/runtime/reconciler/docs/07-adding-a-resource.md).

### 5. Write tests

Every resource needs tests covering:
- `Create` — resource created; already exists (no-op)
- `Apply` — SSA patch issued; immutable-field fallback (delete + recreate) if applicable
- `Delete` — resource exists (deleted); does not exist (no-op)

Use the `pkg/registry/simulate` harness and a fake clientset (`kubeclient.NewFakeClientset()`).

---

## Code conventions

- Use `kubeclient.KubeClient` for all API calls — never raw client-go.
- Return wrapped errors: `fmt.Errorf("creating daemonset: %w", err)`.
- Owner references are set via `common.SetOwnerReference(owner, resource)`.
- Labels are merged with `labels.ManagedLabels(owner)`.
- Namespace resolution uses `common.ResolveNamespace(owner, spec.Namespace)`.
