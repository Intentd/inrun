# hooks

A `Database` operator split between YAML and Go. The Blueprint declares the
ServiceAccount; the `OnReconcile` and `OnDelete` hooks in
`hooks/database_hooks.go` build the StatefulSet, Service and optional backup
CronJob, and refer to the ServiceAccount by name (`<cr>-sa`).

Types are in `api/v1alpha1/`; `blueprint.yaml` points at both.

### 1. Generate the registry

Writes the type registry and `cmd/inrun/main.go`.

```bash
make registry
```

### 2. Build your inrun

Installs an `inrun` with the Database type compiled in at `~/.inrun/bin/inrun`.

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
kubectl get statefulset,service,cronjob
```

```bash
kubectl get events --field-selector involvedObject.name=my-db
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
