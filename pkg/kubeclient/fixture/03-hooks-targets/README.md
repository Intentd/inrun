# Per-Target OperatorBox — BlockchainAppWithTargets

Three surfaces, one CRD, three different reconcile strategies — all dispatched
by the runtime based on the `inrun.dev/serve-target` annotation the gateway
stamps on the CR.

| Target | Strategy | What it does |
|--------|----------|--------------|
| `v2-enabled` | Per-target hooks | Same binary as `v2-disabled`; args force `featureEnabled=true` and gate on business hours |
| `v2-disabled` | Per-target hooks | Same binary; args force `featureEnabled=false`, no gate |
| `v2-ctor` | Per-target constructor | Distinct `domain.Reconciler` implementation; reads `featureEnabled` from args, owns the full reconcile loop |

**Hooks targets** share one binary. The Blueprint's `args` determine what each
surface means — the hook reads `req.Args` and never knows which surface it came from:

```yaml
serve:
  target:
    v2-enabled:
      primary: true
      operatorBox:
        preReconcile:
          enqueueGate:           # blocks outside business hours on this surface
            when:
              - field: '{{ inBlockchainBusinessHours }}'
                equals: "true"
        reconcile:
          args:
            featureEnabled: "true"
            inBusinessHours: '{{ inBlockchainBusinessHours }}'

    v2-disabled:
      operatorBox:
        reconcile:
          args:
            featureEnabled: "false"
            inBusinessHours: '{{ inBlockchainBusinessHours }}'
```

**Constructor target** brings its own `domain.Reconciler`. The runtime wraps
all three in a `MuxReconciler` that routes each CR to the right reconciler at
dispatch time:

```yaml
    v2-ctor:
      operatorBox:
        reconcile:
          default: false
          constructor:
            location: github.com/intentd/inrun-args-hooks-targets/constructor
            function: NewBlockchainAppWithTargetsReconciler
            alias: bcctor
            args:
              featureEnabled: "true"
```

The caller just picks a target:

**Requirement:** `inrun` CLI — install from [inrun-install](https://github.com/intentd/inrun#getting-started)

---

## Step 1 — Generate the registry

```bash
make registry
```

## Step 2 — Build

```bash
make clean && make build
inrun validate blueprint.yaml
```

### Simulate and Play without a cluster


#### Simulate

```bash
inrun simulate -f simulate-v2-enabled.yaml
inrun simulate -f simulate-v2-disabled.yaml
inrun simulate
```

#### Play

- First check permissions:

```bash
inrun serve can-i --token dev --operation create --target v2-enabled
```

- Then Play:

```bash
inrun serve play -i intent/intent-v2-enabled.yaml --token dev
```

## Step 3 — Run

```bash
inrun --dev-server
```

In another terminal, start the gateway and get the token:

```bash
export INRUN_PORT=8888
inrun gate run
```

Apply via the `v2-enabled` surface (feature on, business-hours gate active):

```bash
export TOKEN=$(kubectl get secret inrun-dev-token -n default -o jsonpath='{.data.token}' | base64 -d)
```

```bash
inrun serve apply -f intent/intent-v2-enabled.yaml --token $TOKEN --api http://localhost:8888
```

Check the result:

```bash
kubectl get deployment 03-hooks-targets-my-chain \
  -o jsonpath='{.metadata.annotations.feature\.demo/v2-enabled}' && echo

kubectl get blockchainappwithtargets 03-hooks-targets-my-chain \
  -o jsonpath='{.status.featureEnabled}' && echo

kubectl get blockchainappwithtargets 03-hooks-targets-my-chain \
  -o jsonpath='{.status.inBusinessHours}' && echo
```

Switch to `v2-disabled` (feature off, no gate):

```bash
inrun serve apply -f intent/intent-v2-disabled.json --token $TOKEN --api http://localhost:8888
```

Switch to `v2-ctor` (constructor reconciler, feature on):

```bash
inrun serve apply -f intent/intent-v2-ctor.json --token $TOKEN --api http://localhost:8888
```

The runtime routes this CR to `BlockchainAppWithTargetsReconciler` via `MuxReconciler`
instead of the CRD-level `Generic Reconciler`.

> Switching targets cleans up the previous surface's resources automatically.
> `keepPreviousSurface: true` on the target entry skips the cleanup when you
> want both surfaces running simultaneously.

## E2E

```bash
make docker push IMAGE_REPO=yourregistry/blockchainappwithtargets-operator IMAGE_TAG=latest

inrun e2e --dev-server \
  --set runtime.image.repository=yourregistry/blockchainappwithtargets-operator \
  --set runtime.image.tag=latest
```

## Cleanup

```bash
chmod +x ./cleanup.sh && ./cleanup.sh
```
