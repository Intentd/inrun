# unique

Demonstrates `operator: unique` enforced at **both** points: reconcile time
(a live, authoritative check against the API server) and admission time (a
fast, best-effort check against the runtime's own informer cache, reached
over HTTP — see [`pkg/gateway/webhook/uniqueness.go`](../../../../gateway/webhook/uniqueness.go) and
`GET /blueprint/{crd}/cr?field=` in `pkg/runtime/coordinator`).

`cr.yaml` (`site-a`, `spec.domain: a.example.com`) is applied first and
reconciles to `Ready`. `cr-duplicate.yaml` (`site-b`, same domain) is then
applied directly with `kubectl apply` and must be rejected synchronously —
`kubectl` exits non-zero with the webhook's denial message in stderr, and
`site-b` is never created at all, not just left un-reconciled.

This is the scenario that isn't provable via `inrun simulate` — simulate
doesn't register admission webhooks (see
[`pkg/registry/simulate/docs/03-limitations.md`](../../../simulate/docs/03-limitations.md)), so it can only exercise the
reconcile-time half of `operator: unique` (see
[`pkg/registry/simulate/fixture/unique/`](../../../simulate/fixture/unique/README.md)). A real cluster is what proves the
admission-time half actually rejects at `kubectl apply` time, not just "gets
denied eventually."

## Run

```sh
inrun e2e pkg/registry/e2e/fixture/unique/e2e.yaml
```
