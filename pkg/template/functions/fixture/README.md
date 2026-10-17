# pkg/note/fixture

Living integration fixture for the Inrun note functions.

## Why this exists

Notes take `map[string]interface{}` from the dynamic client as input. Unit tests
construct these maps by hand — but real API responses differ in subtle ways:
extra metadata fields, different numeric types after JSON round-tripping, absent
optional fields that resolve to zero in the real API.

This fixture closes that gap. Apply a CR, watch status populate, and every note's
output is visible directly on the object — no log-diving required.

---

## Catalogs

All catalogs use the same `NoteProbe` CRD. A single CRD with a flexible spec
is enough to probe every note family without registering separate CRD types per
resource family.

| File | Notes covered | Enrichment required |
|---|---|---|
| `catalog.yaml` | kubernetes, replica, container, service | none |
| `catalog-pods.yaml` | pod enrichment on Deployment | `enrich: [pods]` |
| `catalog-statefulset.yaml` | StatefulSet pod enrichment, `podByOrdinal` | `enrich: [pods]` |
| `catalog-service.yaml` | endpoint enrichment on Service | `enrich: [endpoints]` |
| `catalog-job.yaml` | job lifecycle + pod enrichment on Job | `enrich: [pods]` |
| `catalog-warnings.yaml` | warning event enrichment on any resource | `enrich: [events]` |
| `catalog-pvc.yaml` | PVC lifecycle notes + enriched PV notes | `enrich: [pvc]` |
| `catalog-ingress.yaml` | Ingress notes — host, IP, rules, TLS | none |
| `catalog-hpa.yaml` | HPA replica scaling notes | none |

### `catalog.yaml`

Covers the general kubernetes-family notes that work on any child resource:
`resourceExists`, `allReplicasReady`, `containerImage`, `serviceClusterIP`,
`endpointsReady`, and the full replica + kubernetes note set.

### `catalog-pods.yaml`

Covers the pod note family: `podNames`, `podIPs`, `podPhases`, `podNodes`,
`podCount`, `readyPodCount`, `podMaxRestarts`, `hasCrashingPod`.

### `catalog-statefulset.yaml`

Covers StatefulSet-specific patterns: ordered membership (`podNames`, `podIPs`),
and `podByOrdinal` for surfacing the primary member's name and IP.

### `catalog-service.yaml`

Covers enriched endpoint notes: `hasEndpoints`, `serviceEndpoints`,
`serviceEndpointCount`, `serviceFirstEndpoint`.

### `catalog-job.yaml`

Covers job lifecycle notes (`jobSucceeded`, `jobFailed`, `jobActive`) and
enriched pod notes on jobs: `jobFirstExitCode`, `jobActivePodNames`,
`jobSucceededPodNames`, `jobFailedPodNames`.

### `catalog-warnings.yaml`

Covers warning event notes: `hasWarnings`, `warningCount`, `firstWarningReason`,
`firstWarning`. Events recorded on pods owned by a workload are also
aggregated — container failures (ImagePullBackOff, OOMKilled) show up here.

---

## Running a probe

```bash
cd pkg/note/fixture

# Run the catalog for the probe family you want to test.
# crdFile and crFiles are embedded — Inrun applies the CRD and CR automatically:
inrun -f catalog-<resource_type>.yaml

# Watch status populate:
kubectl get noteprobe my-probe -o yaml -w

# Clean up:
bash cleanup.sh
```

---

## Adding a note

When you add a note to any `kube_*.go` or `kubernetes.go` file, pick the catalog
that matches the note's resource family and add a `status.fields` entry:

```yaml
- path: myNewNote
  value: "{{ myNewNote .children.deployment }}"
```

Routing rule by resource family:

- Pod notes → `catalog-pods.yaml`
- StatefulSet ordinal notes → `catalog-statefulset.yaml`
- Endpoint notes → `catalog-service.yaml`
- Job lifecycle / pod notes → `catalog-job.yaml`
- Warning event notes → `catalog-warnings.yaml`
- PVC/PV notes → `catalog-pvc.yaml`
- Ingress notes → `catalog-ingress.yaml`
- HPA notes → `catalog-hpa.yaml`
- Everything else → `catalog.yaml`

---

## CI

The `fixture-note` job in `.github/workflows/validate-pr.yml` runs the fixture on
every PR touching `pkg/note/`. It spins up a kind cluster, runs `inrun -f catalog.yaml`,
and asserts `status.phase` is set.
