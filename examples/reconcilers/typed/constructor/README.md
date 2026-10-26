# constructor

A `Pipeline` operator whose reconciler is a full `domain.Reconciler`: it runs
Jobs in order (build → test → notify) and owns the whole loop. The same
behaviour can be declared in a Blueprint; this shows the constructor pattern for
logic you want to keep in Go.

`reconciler/pipeline_reconciler.go` holds `NewPipelineReconciler` and `Reconcile`;
types are in `api/v1alpha1/`.

### 1. Generate the registry

Writes the type registry and `cmd/inrun/main.go`.

```bash
make registry
```

### 2. Build your inrun

Installs an `inrun` with the Pipeline type compiled in at `~/.inrun/bin/inrun`.

```bash
make build
```

### 3. Validate

```bash
inrun validate
```

### 4. Run

Applies `manifests/crd.yaml` and `manifests/cr.yaml`, then reconciles.

```bash
inrun
```

### 5. Check the result

In a second terminal.

```bash
kubectl get pipeline,jobs -w
```

### 6. Deploy

Build and push the image:

```bash
make release IMAGE=<registry>/<image>:<tag>
```

Apply the RBAC and Blueprint bundle:

```bash
inrun generate bundle -f blueprint.yaml -o bundle.yaml
```

```bash
kubectl apply -f bundle.yaml
```

Install Inrun with your image:

```bash
helm upgrade --install inrun inrun/inrun \
  --set runtime.image.repository=<registry>/<image> \
  --set runtime.image.tag=<tag> \
  --namespace inrun-system --wait
```

### 7. Clean up

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

### 8. E2E

Runs the example in a kind cluster with your image from the deploy step, since only it has the Go types compiled in.

```bash
inrun e2e --set runtime.image.repository=<registry>/<image> --set runtime.image.tag=<tag>
```
