# pkg/template/resources/fixture

A typed operator whose hook applies every kind in `pkg/template/resources` in
one reconcile. It catches what only a real API server shows: drift, idempotency
and the defaults the server injects. A failure shows up as a missing resource,
a wrong field or a crashed operator.

It is its own module (`github.com/intentd/inrun-resource-probe`), so the
standard `inrun` doesn't know `ResourceProbe`: build the fixture's binary first.

## Coverage

Namespaces, ConfigMaps, Secrets, ServiceAccounts, ClusterRoles,
ClusterRoleBindings, Roles, RoleBindings, NetworkPolicies, ResourceQuotas,
LimitRanges, Deployments, StatefulSets, ReplicaSets, Pods, Jobs, CronJobs,
Services, Ingresses, HPAs, PDBs, PVCs and PVs, each server-side applied.

`simulate.yaml` runs three cycles and expects a steady state: after the first
apply nothing changes. A field the server defaults, or a Secret's StringData
coming back as Data, would show up there as a change on every cycle.

## Run it

```bash
make registry build   # generates the registry; installs the binary as ~/.inrun/bin/inrun
inrun simulate -f simulate.yaml
inrun e2e -f e2e.yaml --workers 3
```

`e2e` creates a kind cluster and uses the published runtime image from
`values.yaml`. For a local image, `make docker push IMAGE_REPO=<repo>` and add
`--set runtime.image.repository=<repo>`; add `--use-current` to reuse a cluster.

## Adding a kind

1. Apply it in `hooks/resource_hooks.go`.
2. Add a `create` op to the matching `ops/*.yaml`.
3. Add a `resources:` check to `e2e/01-resources.yaml`.
4. List it under Coverage.
