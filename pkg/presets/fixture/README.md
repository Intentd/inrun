# pkg/presets/fixture

Living integration fixture for Inrun presets.

## Why this exists

Unit tests verify expansion logic in isolation. This fixture verifies that the expanded values land correctly on real Kubernetes resources — right CPU/memory on the Deployment, right security context on the Pod spec, right probe timings on the container.

Apply a CR, run the operator, and inspect the child resources directly on the cluster.

---

## Blueprints

All blueprints use the same `PresetProbe` CRD. One CRD is enough to probe every preset family.

| File | Presets covered |
|---|---|
| `blueprint-resource.yaml` | tiny, small, medium, large, burst, steady, compute-heavy, memory-heavy |
| `blueprint-security.yaml` | baseline, restricted, hardened (container + pod security) |
| `blueprint-probes.yaml` | fast, standard, patient, slow-start |
| `blueprint-autoscale.yaml` | burst, steady, batch, latency-sensitive, cost-optimized |
| `blueprint-hpa.yaml` | web, api, latency-sensitive, batch, cost-optimized (HPA behavior) |
| `blueprint-pdb.yaml` | zero-downtime, rolling, relaxed (PDB disruption budget) |
| `blueprint-rolling.yaml` | safe, fast, blue-green (Deployment/StatefulSet/ReplicaSet rolling update) |

---

## Running a probe

```bash
cd pkg/presets/fixture

# Run the blueprint for the preset family you want to verify.
# crdFile and crFiles are embedded — Inrun applies the CRD and CR automatically:
inrun -f blueprint-resource.yaml    # resource presets
inrun -f blueprint-security.yaml   # security presets
inrun -f blueprint-probes.yaml     # probe presets
inrun -f blueprint-autoscale.yaml  # autoscale presets
inrun -f blueprint-hpa.yaml        # HPA behavior presets
inrun -f blueprint-pdb.yaml        # PDB disruption budget presets
inrun -f blueprint-rolling.yaml    # rolling update strategy presets

# Inspect child Deployments:
kubectl get deployments -o yaml | grep -A 10 "resources:"

# Clean up:
bash cleanup.sh
```

---

## Verifying expansion

### Resource presets

```bash
kubectl get deployment my-probe-medium -o jsonpath='{.spec.template.spec.containers[0].resources}'
```

Expected for `medium`:
```json
{"limits":{"cpu":"1","memory":"1Gi"},"requests":{"cpu":"250m","memory":"256Mi"}}
```

### Security presets

```bash
kubectl get deployment my-probe-hardened -o jsonpath='{.spec.template.spec.containers[0].securityContext}'
```

Expected for `hardened`:
```json
{"allowPrivilegeEscalation":false,"readOnlyRootFilesystem":true,"runAsNonRoot":true,"capabilities":{"drop":["ALL"]}}
```

### Probe presets

```bash
kubectl get deployment my-probe-fast -o jsonpath='{.spec.template.spec.containers[0].livenessProbe}'
```

Expected for `fast`:
```json
{"initialDelaySeconds":5,"periodSeconds":10,"failureThreshold":2,"successThreshold":1,"timeoutSeconds":5}
```

### HPA behavior presets

```bash
kubectl get hpa my-probe-web-hpa -o jsonpath='{.spec.behavior}'
```

Expected for `web` (70% CPU, fast scale-up, 5-min scale-down stabilization):
```json
{"scaleDown":{"policies":[{"periodSeconds":60,"type":"Percent","value":10}],"selectPolicy":"Min","stabilizationWindowSeconds":300},"scaleUp":{"policies":[{"periodSeconds":15,"type":"Percent","value":100},{"periodSeconds":15,"type":"Pods","value":4}],"selectPolicy":"Max","stabilizationWindowSeconds":0}}
```

```bash
kubectl get hpa my-probe-cost-hpa -o jsonpath='{.spec.metrics[0].resource.target.averageUtilization}'
# Expected: 80
```

---

## Adding a preset

When you add a preset name to any family, add a deployment entry to the matching blueprint file:

```yaml
- name: "{{ .metadata.name }}-xlarge"
  image: "{{ .spec.image }}"
  port: "{{ .spec.port }}"
  resources:
    preset: xlarge
  reconcile: true
```

Run `inrun -f blueprint-resource.yaml` and verify the Deployment's resource block matches the definition in `pkg/presets/resource.go`.
