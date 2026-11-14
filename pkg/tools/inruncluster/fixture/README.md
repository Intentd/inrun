# Cluster access fixture

Manual integration test for `inrun clusters access --config`.

## Setup

```bash
inrun create cluster --name inrun-bt --count 3
# inrun-bt-1 → gateway cluster (current context)
# inrun-bt-2 → staging
# inrun-bt-3 → prod
```

## Validate the config (no cluster calls)

```bash
inrun clusters access --validate fixture/cluster-config.yaml
```

Expected:
```text
✓ access config valid (2 clusters)
  prod     →  kind-inrun-bt-3   grant (long token)
  staging  →  kind-inrun-bt-2   grant (long token)
```

## Dry run

```bash
inrun clusters access --config fixture/cluster-config.yaml --dry-run
```

## Run

```bash
inrun clusters access --config fixture/cluster-config.yaml
```

Verify secrets on the gateway cluster:
```bash
kubectl get secret inrun-staging inrun-prod -n default
```

## Revoke

Set `revoke: true` on a cluster and run again. The command asks before it
removes anything; `--yes` skips the question.

```bash
inrun clusters access --config fixture/cluster-config.yaml
```

## Cleanup

```bash
kind delete cluster --name inrun-bt-1
kind delete cluster --name inrun-bt-2
kind delete cluster --name inrun-bt-3
```
