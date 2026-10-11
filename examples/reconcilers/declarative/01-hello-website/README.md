# 01-hello-website

One `Website` CR becomes a Deployment and a Service, owned by the CR, with
`phase` and `endpoint` written to its status.

### 1. Validate

```bash
inrun validate
```

### 2. Run

```bash
inrun
```

### 3. Open the console

`inrun` started it next to the runtime. Open
[localhost:8081](http://localhost:8081) and log in with `inrun` / `inrun` to
see the live operator: CRD health, worker state, reconcile metrics, and the
`hello-website` CR with its Deployment and Service.

### 4. Check the status

In a second terminal.

```bash
kubectl get website hello-website -o jsonpath='{.status}'; echo
```

### 5. Delete the Deployment

It comes back on the next reconcile; the console shows the reconcile.

```bash
kubectl delete deployment hello-website
```

### 6. Scale through the CR

```bash
kubectl patch website hello-website --type=merge -p '{"spec":{"replicas":2}}'
```

### 7. Clean up

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

### 8. E2E

Runs the same checks in a kind cluster.

```bash
inrun e2e
```
