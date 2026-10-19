# pkg/gateway/fixture/crs

Test CRs that exercise the admission rules in `../admission/platformresource.yaml`.

Each file is a minimal `PlatformResource` CR targeting one specific rule or combination.

| File | Rule exercised | Expected outcome |
|------|---------------|-----------------|
| `app-staging.yaml` | Happy path — no conditions trigger | Accepted, no violations |
| `app-production-warn.yaml` | `warn when: environment=production` + missing `productionApproval` | Accepted with `ValidationWarning` |
| `app-production-approved.yaml` | Same `when:` guard — condition met but field present | Accepted, warn rule skipped |
| `cert-valid.yaml` | `deny when: workloadType=cert` + domain present | Accepted, deny rule skipped |
| `cert-missing-domain.yaml` | `deny when: workloadType=cert` + domain absent | Rejected — `spec.domain is required for cert workloads` |
| `monitoring-valid.yaml` | `deny or: workloadType=app\|monitoring` + repoURL present | Accepted, deny rule skipped |
| `monitoring-missing-repo.yaml` | `deny or: workloadType=app\|monitoring` + repoURL absent | Rejected — `spec.repoURL is required for app and monitoring workloads` |
| `app-direct-apply.yaml` | `warn when: isDirectApply .` — no provenance annotation | Accepted with `ValidationWarning` — direct apply detected |
| `app-via-gateway.yaml` | `warn when: isDirectApply .` — `serve-target` annotation present | Accepted, warn rule skipped |

## Rules being tested

```
deny  — spec.team exists                        (unconditional)
deny  — spec.workloadType in app,cert,monitoring (unconditional)
deny  — spec.environment in staging,production   (unconditional)
deny  — spec.domain exists      WHEN workloadType=cert
deny  — spec.repoURL exists     OR workloadType=app | workloadType=monitoring
deny  — spec.domain unique                       (unconditional)
warn  — spec.productionApproval exists  WHEN environment=production
warn  — (isDirectApply detection)       WHEN isDirectApply . == true
```

## Key assertions

- `when:` rules are **skipped entirely** when the condition does not match — no violation, no log entry.
- `or:` rules are **skipped entirely** when none of the listed values match.
- A staging CR never triggers the `productionApproval` warn — even if the field is absent.
- A cert CR never triggers the `repoURL` deny — even though repoURL is absent.
- A monitoring CR with repoURL present passes the `or` deny rule cleanly.
- A CR without any provenance annotation triggers the `isDirectApply` warn — `serve-target` annotation present skips it.

## Running manually

### Step 1 — apply the CRD and namespaces

```bash
kubectl apply -f pkg/gateway/fixture/crd.yaml
kubectl apply -f pkg/gateway/fixture/setup.yaml
```

### Step 2 — start Inrun with the gateway fixture

```bash
inrun -f pkg/gateway/fixture/blueprint.yaml
```

### Step 3 — apply the test CRs

```bash
# Happy path — no rules fire
kubectl apply -f pkg/gateway/fixture/crs/app-staging.yaml

# Accepted with ValidationWarning (production, no approval ticket)
kubectl apply -f pkg/gateway/fixture/crs/app-production-warn.yaml

# Accepted, warn rule skipped (approval ticket present)
kubectl apply -f pkg/gateway/fixture/crs/app-production-approved.yaml

# Accepted, deny when rule skipped (domain present)
kubectl apply -f pkg/gateway/fixture/crs/cert-valid.yaml

# Rejected — spec.domain is required for cert workloads
kubectl apply -f pkg/gateway/fixture/crs/cert-missing-domain.yaml

# Accepted, deny or rule skipped (repoURL present)
kubectl apply -f pkg/gateway/fixture/crs/monitoring-valid.yaml

# Rejected — spec.repoURL is required for app and monitoring workloads
kubectl apply -f pkg/gateway/fixture/crs/monitoring-missing-repo.yaml
```

### Full cluster e2e (all scenarios automated)

```bash
inrun e2e -f pkg/gateway/fixture/e2e.yaml
```
