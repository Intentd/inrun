# deployments/

Live deployments. `public/` runs six runtimes in one cluster, each in its own namespace, aggregated by one Console at [console.inrun.dev](https://console.inrun.dev).

| Runtime | Operator | CRDs | Shows |
|---------|----------|------|-------|
| `inrun-system-01` | hello-website | 1 | Deployment + Service from a `Website` CR (also hosts the Console) |
| `inrun-system-02` | website-with-serviceaccount | 1 | The same, plus a ServiceAccount per instance |
| `inrun-system-03` | secret-distribution | 1 | Copies a Secret across namespaces from a cluster-scoped CR |
| `inrun-system-04` | app-platform | 5 | ReplicaSets, generated API keys with 90-day rotation, ConfigMap distribution |
| `inrun-system-05` | data-platform | 10 | Ingestion to delivery, 30/60-day credential rotation |
| `inrun-system-06` | network-suite | 7 | Traffic routing, self-signed TLS, 180-day key rotation |

Each `cluster-NN/` holds `blueprint.yaml`, `crd.yaml` and `cr.yaml`. Runtimes 01 and 02 manage the same CRD and are split by `allowedNamespaces` (`demo-01`, `demo-02`); 04 to 06 own their own API groups.

```bash
cd deployments/public
make all          # or make cluster-01 … cluster-06
make clean
```

Each target creates the namespace, applies the CRD, applies the RBAC from `inrun generate bundle`, installs via Helm and applies the CR.
