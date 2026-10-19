# Inrun Helm Chart

Inrun is the intent runner for Kubernetes. A caller sends an intent through the
gateway; Inrun turns it into a custom resource, reconciles it, and returns a
view.

This chart deploys:

- **Runtime:** reconciles the custom resources declared in your Blueprint.
- **Gateway:** takes intents in, serves admission webhooks, and enforces
  deletion and namespace protection.
- **Console:** a web UI over one or more runtimes.

## Prerequisites

- Kubernetes 1.28+
- Helm 3.10+
- The `inrun` CLI:

```bash
curl -sSL https://get.inrun.dev | bash
```

## Install

Inrun does not create RBAC or its Blueprint ConfigMap itself. You generate them
from your Blueprint, review them, and apply them, so every permission is visible
before it reaches the cluster.

### 1. Install the CRD

This uses the Website CRD from the hello-website example:

```bash
kubectl apply -f https://raw.githubusercontent.com/intentd/inrun/main/examples/reconcilers/declarative/01-hello-website/manifests/crd.yaml
```

### 2. Write a Blueprint

Save as `blueprint.yaml`:

```yaml
apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: hello-website
spec:
  crds:
    website:
      apiTypes:
        group: demo.inrun.dev
        version: v1alpha1
        kind: Website
        plural: websites
      operatorBox:
        reconcile:
          onCreate:
            deployments:
              - name: "{{ .metadata.name }}"
                image: "{{ .spec.image }}"
                replicas: "{{ .spec.replicas }}"
                port: "{{ .spec.port }}"
                reconcile: true
            services:
              - name: "{{ .metadata.name }}-svc"
                port: "80"
                targetPort: "{{ .spec.port }}"
                reconcile: true
```

### 3. Review the permissions

```bash
inrun validate --full -f blueprint.yaml
```

This prints every RBAC rule the Blueprint needs, per CRD and per component, with
a note on why each one exists. No cluster required.

### 4. Generate and apply the bundle

```bash
inrun generate bundle -f blueprint.yaml -o bundle.yaml
```

```bash
kubectl apply -f bundle.yaml
```

The bundle holds the `inrun-system` Namespace, the ServiceAccounts, a
ClusterRole with only the permissions the Blueprint needs, its binding, and the
`inrun-blueprint` ConfigMap. Commit it to your GitOps repo if you use one.

### 5. Install the chart

```bash
helm repo add inrun https://intentd.github.io/inrun
```

```bash
helm upgrade --install inrun inrun/inrun \
  --namespace inrun-system \
  --set gateway.enabled=true
```

The chart uses the ServiceAccounts and ConfigMap from the bundle.

### 6. Verify

```bash
kubectl get pods -n inrun-system
```

## Topologies

The runtime and gateway deploy independently.

| Topology | `runtime.enabled` | `gateway.enabled` | Use |
| --- | --- | --- | --- |
| Runtime only | `true` | `false` | Reconcile without intents or admission |
| Gateway only | `false` | `true` | Deletion and namespace protection only |
| Full | `true` | `true` | Intents in, reconcile, views out |

For gateway only, set `gateway.standalone: true` in the Blueprint and generate
with `inrun generate bundle --for gateway`.

## Values

### Runtime

| Parameter | Description | Default |
| --- | --- | --- |
| `runtime.enabled` | Deploy the runtime | `true` |
| `runtime.image.repository` | Image | `ghcr.io/intentd/inrun` |
| `runtime.image.tag` | Tag | Chart `appVersion` |
| `runtime.replicaCount` | Replicas; one leader reconciles | `2` |
| `runtime.serviceAccount` | ServiceAccount from the bundle | `inrun` |
| `runtime.server.httpPort` | Health, metrics and API port | `8080` |
| `runtime.config.logLevel` | debug, info, warn, error | `info` |
| `runtime.config.defaultWorkers` | Workers per CRD | `2` |
| `runtime.config.defaultResync` | Resync interval | `30s` |
| `runtime.config.maxDepth` | Max queue depth per CRD | `500` |
| `runtime.config.failureThreshold` | Failures before degraded | `10` |
| `runtime.leaderElection.enabled` | Leader election | `true` |
| `runtime.blueprint.existingConfigMap` | Blueprint ConfigMap | `<release>-blueprint` |
| `runtime.blueprint.configMapKey` | Key in the ConfigMap | `blueprint.yaml` |
| `runtime.gatewayEndpoint` | Gateway URL shown to the console | `""` |
| `runtime.registry.url` | Pattern registry | `""` |

### Gateway

| Parameter | Description | Default |
| --- | --- | --- |
| `gateway.enabled` | Deploy the gateway | `false` |
| `gateway.image.repository` | Image | `ghcr.io/intentd/inrun-gateway` |
| `gateway.replicaCount` | Replicas; stateless | `2` |
| `gateway.serviceAccount` | ServiceAccount from the bundle | `inrun-gateway` |
| `gateway.server.httpPort` | Gateway API, health and metrics | `8080` |
| `gateway.server.httpsPort` | Webhook port | `8443` |
| `gateway.webhooks.enabled` | Serve webhooks with your own TLS secret | `false` |
| `gateway.webhooks.existingSecret` | TLS secret for webhooks | `""` |
| `gateway.blueprint.existingConfigMap` | Blueprint ConfigMap | the runtime's |
| `gateway.ingress.enabled` | Expose the Gateway API | `false` |

### Console

| Parameter | Description | Default |
| --- | --- | --- |
| `console.enabled` | Deploy the console | `true` |
| `console.image.repository` | Image | `ghcr.io/intentd/inrun-console` |
| `console.config.inrunURLs` | Runtimes to watch | this release's runtime |
| `console.config.refreshInterval` | Refresh interval | `10s` |
| `console.gatewayToken.secretRef.name` | Gateway token Secret for serving intents | `""` |
| `console.ingress.enabled` | Expose the console | `false` |

### Every component

These apply under `runtime:`, `gateway:` and `console:`: `imagePullSecrets`,
`resources`, `pdb`, `networkPolicy` (runtime and console), `hpa` (gateway and
console), `nodeSelector`, `tolerations`, `affinity`,
`topologySpreadConstraints`, `extraEnv`, `extraEnvFrom`, `extraVolumes`,
`extraVolumeMounts`, `podAnnotations` and `podLabels`. See
[values.yaml](values.yaml) for the full list.

## Observe

```bash
inrun proxy
```

This forwards the runtime, gateway and console ports to localhost. The console
is then at http://localhost:8081/console.

## Uninstall

```bash
helm uninstall inrun --namespace inrun-system
```

CRDs and the custom resources Inrun managed stay in the cluster.

## License

Apache 2.0.
