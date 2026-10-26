# notes-validation (webhook)

Same scenario as `pkg/runtime/reconciler/fixture/notes-validation`, but with the gateway
and admission webhooks enabled. Validation and mutation run synchronously at `kubectl apply`
time — the API server calls the webhook before the CR is persisted.

## Notes declared

| Note | Value |
|---|---|
| `inBusinessHours` | `true` during Mon–Fri 09:00–18:00 UTC |
| `allowedRegistry` | `"myorg/"` — the only permitted image prefix |
| `defaultReplicas` | `"2"` — fallback when spec.replicas is absent |

## What the webhook enforces

**Mutation** (runs first, `mutateFirst: true`):
- Defaults `spec.replicas` to `{{ defaultReplicas }}` before validation

**Validation**:
- Denies if `{{ inBusinessHours }}` is `false`
- Denies if `spec.image` does not start with `{{ allowedRegistry }}`
- Denies if `spec.replicas` is less than `1`

Denial is synchronous — `kubectl apply` exits non-zero with the webhook message in stderr.

## Bootstrap CR

`cr-placeholder.yaml` (a Namespace) is used as the e2e bootstrap CR. The real
`DeploymentRequest` cannot be auto-applied at cluster start because the webhook
would block it outside business hours. Each test step applies `cr.yaml` explicitly,
guarded by `when: inBusinessHours`.

## Run

```sh
inrun e2e pkg/gateway/webhook/fixture/notes-validation/e2e.yaml
```
