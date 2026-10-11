# Remote

A `WebApp` operator whose reconciler is a bash script behind HTTP. Inrun
owns the informer, queue, backoff, owner references, server-side apply and
health; the script only returns what should exist.

### 1. Start the reconciler

Needs `socat` and `jq`. Listens on :8025.

```bash
RECONCILER_TOKEN=inrun-demo-token ./reconciler/reconciler.sh
```

### 2. Simulate

In a second terminal. No cluster needed: each cycle calls the running
reconciler and applies what it returns to an in-memory cluster. Add
`--envtest` to run against a real API server instead.

```bash
inrun simulate
```

### 3. Run

Applies `manifests/secret.yaml`, which holds the same token.

```bash
inrun
```

### 4. Check the result

```bash
kubectl get webapp my-webapp -o jsonpath='{.status}'; echo
```

```bash
kubectl get deployment my-webapp
```

```bash
kubectl get service my-webapp-svc
```

### 4a. Prune

The Catalog sets `prune.enabled`, so a child the reconciler stops returning is
deleted. Turn off `expose` and the reconcilers leave the Service out:

```bash
kubectl patch webapp my-webapp --type merge -p '{"spec":{"expose":false}}'
```

```bash
kubectl get service my-webapp-svc   # NotFound after the next reconcile
```

### 4b. Another language

Stop the bash reconciler and start one of these instead. The Catalog does not
change.

```bash
RECONCILER_TOKEN=inrun-demo-token python3 reconciler/main.py
```

```bash
RECONCILER_TOKEN=inrun-demo-token go run reconciler/main.go
```

## The contract

Inrun POSTs the CR's identity, the CR itself (with `managedFields` stripped) and the evaluated `args`:

```json
{ "key": "default/my-webapp",
  "apiVersion": "migration.demo.inrun.dev/v1alpha1", "kind": "WebApp",
  "object": {...},
  "args": { "logLevel": "info", "environment": "development", "appName": "my-webapp" } }
```

`target` is added when the CR came through a serve target, and `children` when `request.children` is set. `request.object.exclude` removes paths from `object`; nothing else in the body carries the CR.

The reconciler returns status, labels and annotations for the WebApp, and
resources in intent form: a built-in `type` and flat `fields`. Inrun builds the
objects, sets owner references and applies them. With `reconcile.prune.enabled`,
a child it applied before and no longer finds in `resources` is deleted.

```json
{ "result": "ok",
  "status": { "phase": "Running" },
  "labels": { "environment": "development" },
  "annotations": { "webapp.example.com/image": "nginx:latest" },
  "resources": [
    { "type": "deployment", "fields": { "name": "my-webapp", "image": "nginx:latest", "replicas": 1, "port": 80 } },
    { "type": "service",    "fields": { "name": "my-webapp-svc", "port": 80, "targetPort": 80 } } ] }
```

The `reconcilerEndpoint` note uses `.inrun.inPod` to pick localhost or the
in-cluster Service, so one Catalog serves both (see [`reconciler/deploy.yaml`](./reconciler/deploy.yaml)).

### 5. Clean up

Run it while `inrun` is still up, so the runtime can remove the CR's finalizer.

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

### 6. E2E

Deploys the reconciler image ([`reconciler/deploy.yaml`](./reconciler/deploy.yaml)) into a kind cluster and checks status, children, drift correction and cleanup. No local reconciler needed.

```bash
inrun e2e
```