# Existing service, with resources

The [existing-service](../existing-service/) example, plus Kubernetes
resources the API knows nothing about. This is often why an operator gets
built: a service provisions something, and the cluster needs objects that
follow from it.

The bucket API answers with the bucket's id and endpoint. A ConfigMap
template on the CRD runs after the call, with the answer in scope as
`.response`, and publishes both for apps in the namespace to mount. Inrun
owns the ConfigMap: it is applied with the CR as owner, kept in sync, and
deleted with the CR.

```yaml
onReconcile:
  configMaps:
    - name: "{{ .metadata.name }}-bucket"
      data:
        bucketId: "{{ .response.json.id }}"
        endpoint: "{{ .response.json.endpoint }}"
```

The call takes the place a Go hook has in a declarative operator: it drives
the reconcile's result (status and requeue from `remote.response`), and the
templates follow it. On delete, `request.finalize` replaces the `onDelete`
templates, as a hook's `OnDelete` does.

### 1. Simulate

No cluster needed. `--dev-server` starts the API on :9999.

```bash
inrun simulate --dev-server
```

### 2. Run

```bash
inrun --dev-server
```

### 3. Check the result

```bash
kubectl get bucket assets
```

```bash
kubectl get configmap assets-bucket -o jsonpath='{.data}'; echo
```

### 4. Clean up

Run it while `inrun` is still up, so the delete reaches the API.

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

### 5. E2E

Deploys the dev server into a kind cluster and checks the mapped status, the
ConfigMap and the delete.

```bash
inrun e2e --dev-server
```
