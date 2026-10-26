# platform-blueprint

A Helm chart that renders an Inrun Blueprint and nothing else: no CRDs, no
operator. A Stack pulls it in through `sources.helm`.

## Values

| Field | Default | Description |
| --- | --- | --- |
| `apiGroup` | `demo.inrun.dev` | API group for every CRD |
| `apiVersion` | `v1alpha1` | API version for every CRD |
| `crds` | `database`, `cache` | Map of CRD name to `kind`, `plural`, `namespaced`, `enabled`, `reconcile.workers`, `reconcile.resync`, `description` |

## Use it from a Stack

```yaml
sources:
  helm:
    - repo: https://github.com/intentd/inrun.git
      path: charts/examples/platform-blueprint
      chart: platform-blueprint
      version: main
```
