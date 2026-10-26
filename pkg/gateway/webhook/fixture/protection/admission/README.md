# admission (webhook)

Verifies `ValidatingWebhookConfiguration` + `MutatingWebhookConfiguration` on a
CRD: defaults are filled in by the mutating webhook before the validating
webhook runs, and a CR that still fails validation after mutation is denied
at `kubectl apply` time — before it ever reaches the reconciler.

`cr-valid.yaml` is accepted and reconciles to a Deployment. `cr-bad.yaml` is
rejected synchronously; `kubectl apply` exits non-zero with the denial
message in stderr.

## Run

```sh
inrun e2e pkg/gateway/webhook/fixture/protection/admission/e2e.yaml
```
