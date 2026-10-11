# pkg/registry/e2e/fixture

Living integration fixture for the `kubectl:` DSL block.

## Why this exists

The unit tests in `pkg/registry/e2e/` verify the runner and validator logic in isolation. This fixture verifies that every `kubectl:` subcommand works against a real Kubernetes cluster with a real operator — not a mock. It catches regressions that only surface when kubectl, curl, jq, and the Kubernetes API all interact together.

**Rule: when you add a new `kubectl:` subcommand to the DSL, you must add at least one checkpoint to `e2e.yaml` that exercises it.**

`e2e.yaml` is also the source for `documentation/reference/schema/04-e2e/08-complete-example.md`. Run `make inrun` after any change here to keep the doc page in sync.

---

## What is covered

`e2e.yaml` has one checkpoint per subcommand:

| Subcommand | Checkpoint | What it verifies |
|---|---|---|
| `resources:` | All resources created and ready | both Deployments ready, both Services exist |
| `kubectl.get` | Both probes reach Ready / Field assertions | jsonpath field extraction, multiple entries |
| `kubectl.logs` | Pod logs are clean | devserver: `outputNotContains: panic`; nginx: `outputContains: nginx` |
| `kubectl.describe` | Describe shows expected resource state | describe on Deployment and Service |
| `kubectl.exec` | Exec into nginx pod (has sh) | `sh -c "echo e2e-ready"` + equals assertion |
| `kubectl.port-forward` | Port-forward to devserver health endpoints | `/health`, `/startup`, `/ready` via JSON API |
| `kubectl.apply` | Apply a ConfigMap inline | inline manifest + follow-up `kubectl.get` |
| `kubectl.patch` | Patch server probe and assert | merge patch on CR spec field |
| `commands:` | Arbitrary commands still work | raw kubectl alongside the DSL |
| `onFailure:` (spec-level) | Diagnostics printed when any expectation fails | get, logs, describe, events, exec + raw command |
| `onFailure:` (per-expectation) | Diagnostics printed immediately when that expectation fails | get, describe + raw command on "Both probes reach Ready status" |

---

## Running the fixture

```bash
cd pkg/registry/e2e/fixture

# Start the runtime against a kind cluster:
inrun

# In a second terminal, apply the CR:
kubectl apply -f cr.yaml
kubectl get e2eprobe my-probe -o yaml -w   # watch until phase: Ready

# Run the full e2e suite:
inrun e2e

# Clean up:
bash cleanup.sh
```

---

## Operator overview

The `E2EProbe` CRD creates one `Deployment` and one `Service` per CR. Two CRs are applied from `cr.yaml` (multi-document):

| CR | Image | Port | Purpose |
|---|---|---|---|
| `my-probe-server` | `ghcr.io/intentd/inrun-dev-server:latest` | 9999 | Port-forward and JSON endpoint assertions (`/health`, `/startup`, `/ready`) |
| `my-probe-exec` | `nginx:alpine` | 80 | Exec assertions — nginx has `sh`, the devserver is distroless |

`status.phase` progresses `Pending` → `Deploying` → `Ready` on each CR independently.

---

## Adding a new subcommand

1. Implement the subcommand in `pkg/registry/e2e/verify.go` and `pkg/registry/e2e/validate.go`
2. Add the type to `pkg/types/e2e.go` and wire it into `E2EKubectl`
3. Add a checkpoint to `e2e.yaml` that exercises it
4. Add the subcommand to the `onFailure:` block in `e2e.yaml`
5. Update the table above
6. Update `documentation/reference/schema/04-e2e/07-kubectl.md` with the field reference
