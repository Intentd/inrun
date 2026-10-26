# consume

Run an operator someone else published. You write no CRD, no reconciler and no tests: the author proved the
pattern with simulate and e2e before publishing it, and the proof travels with it.

## 1. Find a pattern

```bash
inrun patterns
```

Public patterns need no login. If this fails with `denied` or `unauthorized`, an old login to ghcr.io is getting
in the way: `docker logout ghcr.io`, or log in again.

```text
   NAME              LATEST  KIND       E2E  TAGS                    DESCRIPTION
   deployment-stack  v0.1.0  Blueprint  ✓    deployment, service...  A declarative application d...
   postgres          v0.1.0  Blueprint  ✓    postgres, database,...  A declarative PostgreSQL de...
   redis             v0.1.0  Blueprint  ✓    redis, cache, key-v...  A declarative Redis deploym...
   ...
```

## 2. Look before you pull

```bash
inrun inspect deployment-stack:v0.1.0
```

```text
  Simulate:    ✓ Verified · 9 assertions · 730ms · tested 5d ago
  E2E:         ✓ Verified · 6 assertions · 22s · tested 5d ago

  Files:
    blueprint.yaml                 1.4 KB
    stack.yaml                     392 B
    manifests/crd.yaml             1.7 KB
    manifests/cr.yaml              188 B
```

Read any file without pulling, for example the sample CR:

```bash
inrun inspect deployment-stack:v0.1.0 --view cr.yaml
```

```yaml
apiVersion: apps.inrun.dev/v1
kind: Application
metadata:
  name: my-app
  namespace: default
spec:
  image: nginx:latest
  port: 80
  replicas: 2
  maxReplicas: 5
  resourcePreset: burst
```

## 3. Run it

```bash
inrun deployment-stack:v0.1.0 --apply-cr
```

This pulls the pattern, applies its CRD and that sample CR, and starts the runtime. No cluster yet? Add `--dev`
and Inrun creates a local kind cluster first.

In another terminal:

```bash
kubectl get deploy,svc,hpa,pdb -n default
```

```text
deployment.apps/my-app   2/2     2            2           48s
service/my-app-svc   ClusterIP   10.96.41.112   <none>        80/TCP    48s
horizontalpodautoscaler.autoscaling/my-app-hpa   Deployment/my-app   cpu: <unknown>/70%   2         5         2          48s
poddisruptionbudget.policy/my-app-pdb   1               N/A               1                     48s
```

One CR, and the pattern made a Deployment, Service, autoscaler and disruption budget (plus a ServiceAccount and
its Role).

## 4. Run it your way

To change how the pattern runs here without touching it, import it in a [Stack](stack.yaml):

```yaml
imports:
  registry:
    - url: oci://ghcr.io/intentd/registry/patterns/blueprints/deployment-stack:v0.1.0

spec:
  crds:
    application:
      operatorBox:
        reconcile:
          workers: 4
          resync: 30s
```

Stop the first run (Ctrl+C), then:

```bash
inrun -f stack.yaml
```

The same operator, now with 4 workers, picks up `my-app` where the first run left it. Everything else still comes
from the pattern.

## Clean up

While `inrun` is still running (it removes what the CR created):

```bash
./cleanup.sh
```

Then stop `inrun` with Ctrl+C.

Next: [publish](../publish/README.md) your own pattern.
