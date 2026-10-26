# validation-external (webhook)

Demonstrates `fires.reconcile: false` — the admission side of the story.

Two external calls are declared under `validation.external`:

- `admissionCheck` — `fires.reconcile: false`. Fires only at `kubectl apply`.
  A CR whose `healthCheckUrl` returns non-200 is denied before entering etcd.
  The health-check rule is gated on `external.admissionCheck.called == "true"`,
  so it is automatically skipped on every reconcile resync.

- `configFetch` — no `fires:` restriction. Runs at both admission and reconcile,
  driving `status.phase` and `status.configFetchStatus`.

After a CR is accepted, `status.admissionOnlyRan` stays empty — proving the
reconciler never re-ran the admission-only call.

The [`reconciler fixture`](../../../../runtime/reconciler/fixture/validation-external/README.md)
tests the other half: the reconciler accepts a bad-URL CR because `healthCheck`
is skipped there.

## Run

```sh
inrun e2e pkg/gateway/webhook/fixture/validation-external/e2e.yaml --dev-server
```

The [`reconciler fixture`](../../../../runtime/reconciler/fixture/validation-external/README.md)
tests the other half: the reconciler accepts a bad-URL CR because `healthCheck`
is skipped there.
