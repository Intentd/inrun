# publish

Turn your own operator into a pattern others can run. You build on a published module, prove the result works, and
push it. The push runs the proof again and records it on the artifact.

## The pattern

[blueprint.yaml](blueprint.yaml) is a team's `WebService`: one container behind a Service, autoscaled. It writes no
resources itself. It imports the published `deployment-stack` module and fills in the team's defaults:

```yaml
imports:
  - module: oci://ghcr.io/intentd/registry/patterns/modules/deployment-stack:v0.1.0
    with:
      name: '{{ .metadata.name }}'
      namespace: '{{ .metadata.namespace }}'
      image: '{{ .spec.image }}'
      port: '{{ .spec.port | default "8080" }}'
      replicas: '{{ .spec.replicas | default "2" }}'
      maxReplicas: "4"
      resourcePreset: small
```

Its users set `image`, and `port` or `replicas` when the defaults don't fit
([manifests/cr.yaml](manifests/cr.yaml)). The autoscaling limit and resource size are the team's call, not theirs.

## 1. Prove it without a cluster

```bash
inrun validate
inrun simulate
```

```text
  ✓ Steady state at cycle 3 in 2.37s

  ✓ steady
  ✓ noErrors
  ✓ apply deployments/orders (cycle 1)
  ✓ apply services/orders-svc (cycle 1)
  ✓ apply horizontalpodautoscalers/orders-hpa (cycle 1)

  PASS
```

[test/simulate.yaml](test/simulate.yaml) asserts what one `WebService` produces.

## 2. Prove it on a cluster

```bash
inrun e2e
```

```text
E2E Results: webservice-e2e

  ✓ Running                                  (1.015s)
  ✓ Cleaned up                               (759ms)

  2 of 2 passed (18s)
```

[test/e2e.yaml](test/e2e.yaml) checks the Deployment comes up ready, and that deleting the `WebService` removes
everything it created. By default it creates its own kind cluster; `--use-current` runs it on yours.

## 3. Push

Log in to your registry and point Inrun at it:

```bash
docker login ghcr.io
export INRUN_REGISTRY=ghcr.io/<you>/patterns
```

Set `author:` in [blueprint.yaml](blueprint.yaml) to your name or org, then:

```bash
inrun push .
```

The name and version come from the blueprint's `metadata`. Before anything is uploaded, the push runs both gates
again; if either fails, nothing is published. A version that is already published is refused, so a change means a
new `metadata.version`.

## 4. See what consumers see

```bash
inrun inspect webservice:v0.1.0
```

The Simulate and E2E lines show the gate results the push recorded. Anyone can now run your pattern the way the
[consume](../consume/) step ran `deployment-stack`:

```bash
inrun webservice:v0.1.0 --apply-cr
```

New packages on ghcr.io start private: make the package public in its settings for others to pull it.

## Clean up

If `inrun` is running, run `./cleanup.sh` first, then stop it with Ctrl+C.

Next: [deprecate](../deprecate/README.md) the pattern.
