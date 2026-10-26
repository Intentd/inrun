# 03-secret-copy

A cluster-scoped `SecretDistribution` copies a Secret from a source namespace
into each target namespace and keeps the copies in sync. It shows that
Blueprints manage built-in kinds, not only new ones.

> **Trust:** whoever can create a `SecretDistribution` can copy any Secret the
> operator can read into any namespace. Treat it as a platform-internal CRD:
> grant create on it only to platform admins, and scope the operator's Secret
> access to the source namespaces it should read.

### 1. Simulate

No cluster needed.

```bash
inrun simulate
```

### 2. Run

Applies `manifests/setup.yaml` first: the `platform` namespace and the source Secret.

```bash
inrun
```

### 3. Check a copy

In a second terminal.

```bash
kubectl get secret database-credentials -n team-alpha
```

### 4. Change the source

```bash
kubectl patch secret database-credentials -n platform --type=merge -p '{"stringData":{"password":"newpassword"}}'
```

The copy follows:

```bash
kubectl get secret database-credentials -n team-alpha -o jsonpath='{.data.password}' | base64 -d; echo
```

### 5. Clean up

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

### 6. E2E

Runs the same checks in a kind cluster.

```bash
inrun e2e
```
