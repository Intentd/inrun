# Bootstrap fixture

Manual integration test for `inrun clusters bootstrap --config`.

## Setup

```bash
inrun create cluster --name inrun-bt --count 3
# inrun-bt-1 → gateway cluster (current context)
# inrun-bt-2 → staging
# inrun-bt-3 → prod
```

## Validate the config (no cluster calls)

```bash
inrun clusters bootstrap --validate fixture/cluster-config.yaml
```

Expected:
```text
✓ bootstrap config valid (2 clusters)
  staging  →  kind-inrun-bt-2
  prod     →  kind-inrun-bt-3
```

## Dry run

```bash
inrun clusters bootstrap --config fixture/cluster-config.yaml --dry-run
```

## Run

```bash
inrun clusters bootstrap --config fixture/cluster-config.yaml
```

Verify secrets on the gateway cluster:
```bash
kubectl get secret inrun-staging inrun-prod -n default
```

## Cleanup

```bash
kind delete cluster --name inrun-bt-1
kind delete cluster --name inrun-bt-2
kind delete cluster --name inrun-bt-3
```
