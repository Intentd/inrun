# 02-with-serviceaccount

One `Website` CR becomes a ServiceAccount, a Deployment that runs as it, and a
Service. Status reports `phase`, `endpoint` and `allReplicasReady`, the last
read from the live Deployment with a note.

### 1. Simulate

No cluster needed.

```bash
inrun simulate
```

### 2. Run

```bash
inrun
```

### 3. Check the ServiceAccount

In a second terminal.

```bash
kubectl get deployment my-site -o jsonpath='{.spec.template.spec.serviceAccountName}'; echo
```

### 4. Scale and watch the status

```bash
kubectl patch website my-site --type=merge -p '{"spec":{"replicas":4}}'
```

```bash
kubectl get website my-site -o jsonpath='{.status}'; echo
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
