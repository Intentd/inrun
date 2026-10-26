# controller-runtime

A controller-runtime `WebApp` reconciler running inside Inrun with its
`Reconcile` method unchanged. Three things differ from a controller-runtime
project:

1. No `SetupWithManager`: Inrun provides the informer, queue, workers,
   leader election and metrics.
2. No scheme setup: Inrun registers the types at startup.
3. A constructor wires the reconciler in:

```go
func NewWebAppReconciler(kube kubeclient.Interface) domain.Reconciler {
    return domain.ReconcilerFrom(&WebAppReconciler{
        Client: adapter.ToClient(kube),
    })
}
```

`adapter.ToClient` gives the struct the `client.Client` it already expects.

### 1. Generate the registry

Writes the type registry and `cmd/inrun/main.go`.

```bash
make registry
```

### 2. Build your inrun

Installs an `inrun` with the WebApp type compiled in at `~/.inrun/bin/inrun`.

```bash
make build
```

### 3. Validate

```bash
inrun validate
```

### 4. Simulate

```bash
inrun simulate
```

### 5. Run

Applies `manifests/crd.yaml` and `manifests/cr.yaml`, then reconciles.

```bash
inrun
```

### 6. Check the result

In a second terminal.

```bash
kubectl get webapps,deployments,services
```

### 7. Deploy

Build and push the image:

```bash
IMAGE_REPO=<registry>/<image> IMAGE_TAG=<tag> make release
```

Set the same image under `runtime.image` in `test/values.yaml`, then install
Inrun with it:

```bash
helm upgrade --install inrun inrun/inrun -f test/values.yaml \
  --namespace inrun-system --wait
```

### 8. Clean up

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

### 9. E2E

Runs the example in a kind cluster with your image from the deploy step, since only it has the Go types compiled in.

```bash
inrun e2e --set runtime.image.repository=<registry>/<image> --set runtime.image.tag=<tag>
```
