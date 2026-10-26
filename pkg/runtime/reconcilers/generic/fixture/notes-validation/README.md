# notes-validation

Demonstrates user-defined notes in validation and mutation rules.

Notes are declared once and used as live template expressions inside
`validation` and `mutation` — both declared directly on the CRD entry,
before anything enters the `operatorBox`.

## Notes declared

| Note | Value |
|---|---|
| `inBusinessHours` | `true` during Mon–Fri 09:00–18:00 UTC |
| `allowedRegistry` | `"myorg/"` — the only permitted image prefix |
| `defaultReplicas` | `"2"` — fallback when spec.replicas is absent |

## What runs

**Mutation** (runs first, `mutateFirst: true`):
- Defaults `spec.replicas` to `{{ defaultReplicas }}` when absent

**Validation** (runs after mutation):
- Denies if `{{ inBusinessHours }}` is `false` — blocks outside business hours
- Denies if `spec.image` does not start with `{{ allowedRegistry }}`
- Denies if `spec.replicas` is less than `1`

## Error message

When validation blocks, the CR status shows the original template expression
as the field name — not the resolved value — so the message is readable:

```text
field "{{ inBusinessHours }}": deployments are only allowed during business hours (got "false")
```

## Run

```sh
inrun e2e pkg/runtime/reconciler/fixture/notes-validation/e2e.yaml
```
