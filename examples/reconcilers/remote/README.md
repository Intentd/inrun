# Remote

A reconciler that lives outside Inrun, called over HTTP. Inrun owns the
informer, queue, backoff, owner references, server-side apply and health.

| Example | Shows |
|---------|-------|
| [endpoint](endpoint/) | A reconciler written for Inrun, in any language: Inrun POSTs the CR and applies what comes back |
| [existing-service](existing-service/) | An API that knows nothing about Inrun: one call per reconcile, its answer mapped into status |
| [existing-service-resources](existing-service-resources/) | The same API, and Kubernetes resources made from its answer that the API knows nothing about |
| [existing-service-ids](existing-service-ids/) | An API that assigns the id itself: create once by POST, then read by id; the idempotency key makes a retried create safe |

### Simulate all four

No cluster needed. Start the endpoint's reconciler first (see
[endpoint](endpoint/)); `--dev-server` serves the existing service's API.

```bash
inrun simulate ./... --dev-server
```
