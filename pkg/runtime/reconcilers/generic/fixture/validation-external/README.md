# validation-external (reconciler)

Demonstrates `fires.reconcile: false` — the reconciler side of the story.

`healthCheck` is declared under `validation.external` with `fires.reconcile: false`.
The reconciler never calls it. A CR with a bad `healthCheckUrl` (`/status/503`)
passes reconcile because the health-check rule is gated on
`external.healthCheck.called == "true"` — which is never true at reconcile time.

Only `configFetch` runs on every resync, driving `status.phase` and
`status.configFetchStatus`. `status.admissionOnlyRan` stays empty,
proving the admission-only call was suppressed.

The [`webhook fixture`](../../../../gateway/webhook/fixture/validation-external/README.md) tests
the other half: at `kubectl apply`, `healthCheck` fires and blocks the bad URL
before it reaches etcd.

## Run

```sh
inrun e2e pkg/runtime/reconciler/fixture/validation-external/e2e.yaml --dev-server
```

The `--dev-server` flag deploys the mock HTTP server into the cluster at
`http://inrun-dev-server.inrun-system.svc:9999`.
