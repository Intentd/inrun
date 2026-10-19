# pkg/template/funcs/fixture

Living integration fixture for the Inrun note functions.

## Why this exists

Notes take `map[string]interface{}` from the dynamic client as input. Unit tests
construct these maps by hand — but real API responses differ in subtle ways:
extra metadata fields, different numeric types after JSON round-tripping, absent
optional fields that resolve to zero in the real API.

This fixture closes that gap. Apply a CR, watch status populate, and every note's
output is visible directly on the object — no log-diving required.

---

## Blueprints

All blueprints use the same `NoteProbe` CRD. A single CRD with a flexible spec
is enough to probe every note family without registering separate CRD types per
resource family.

| File | Notes covered | Enrichment required |
|---|---|---|
| `blueprint.yaml` | kubernetes, replica, container, service | none |
| `blueprint-pods.yaml` | pod enrichment on Deployment | `enrich: [pods]` |
| `blueprint-statefulset.yaml` | StatefulSet pod enrichment, `podByOrdinal` | `enrich: [pods]` |
| `blueprint-service.yaml` | endpoint enrichment on Service | `enrich: [endpoints]` |
| `blueprint-job.yaml` | job lifecycle + pod enrichment on Job | `enrich: [pods]` |
| `blueprint-warnings.yaml` | warning event enrichment on any resource | `enrich: [events]` |
| `blueprint-pvc.yaml` | PVC lifecycle notes + enriched PV notes | `enrich: [pvc]` |
| `blueprint-ingress.yaml` | Ingress notes — host, IP, rules, TLS | none |
| `blueprint-hpa.yaml` | HPA replica scaling notes | none |

### `blueprint.yaml`

Covers the general kubernetes-family notes that work on any child resource:
`resourceExists`, `allReplicasReady`, `containerImage`, `serviceClusterIP`,
`endpointsReady`, and the full replica + kubernetes note set.

### `blueprint-pods.yaml`

Covers the pod note family: `podNames`, `podIPs`, `podPhases`, `podNodes`,
`podCount`, `readyPodCount`, `podMaxRestarts`, `hasCrashingPod`.

### `blueprint-statefulset.yaml`

Covers StatefulSet-specific patterns: ordered membership (`podNames`, `podIPs`),
and `podByOrdinal` for surfacing the primary member's name and IP.

### `blueprint-service.yaml`

Covers enriched endpoint notes: `hasEndpoints`, `serviceEndpoints`,
`serviceEndpointCount`, `serviceFirstEndpoint`.

### `blueprint-job.yaml`

Covers job lifecycle notes (`jobSucceeded`, `jobFailed`, `jobActive`) and
enriched pod notes on jobs: `jobFirstExitCode`, `jobActivePodNames`,
`jobSucceededPodNames`, `jobFailedPodNames`.

### `blueprint-warnings.yaml`

Covers warning event notes: `hasWarnings`, `warningCount`, `firstWarningReason`,
`firstWarning`. Events recorded on pods owned by a workload are also
aggregated — container failures (ImagePullBackOff, OOMKilled) show up here.

---

## Running a probe

```bash
cd pkg/note/fixture

# Run the blueprint for the probe family you want to test.
# crdFile and crFiles are embedded — Inrun applies the CRD and CR automatically:
inrun -f blueprint-<resource_type>.yaml

# Watch status populate:
kubectl get noteprobe my-probe -o yaml -w

# Clean up:
bash cleanup.sh
```

---

## Adding a note

When you add a note to any `kube_*.go` or `kubernetes.go` file, pick the blueprint
that matches the note's resource family and add a `status.fields` entry:

```yaml
- path: myNewNote
  value: "{{ myNewNote .children.deployment }}"
```

Routing rule by resource family:

- Pod notes → `blueprint-pods.yaml`
- StatefulSet ordinal notes → `blueprint-statefulset.yaml`
- Endpoint notes → `blueprint-service.yaml`
- Job lifecycle / pod notes → `blueprint-job.yaml`
- Warning event notes → `blueprint-warnings.yaml`
- PVC/PV notes → `blueprint-pvc.yaml`
- Ingress notes → `blueprint-ingress.yaml`
- HPA notes → `blueprint-hpa.yaml`
- Everything else → `blueprint.yaml`

---

## CI

The `fixture-note` job in `.github/workflows/validate-pr.yml` runs the fixture on
every PR touching `pkg/note/`. It spins up a kind cluster, runs `inrun -f blueprint.yaml`,
and asserts `status.phase` is set.
