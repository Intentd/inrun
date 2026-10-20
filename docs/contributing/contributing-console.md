# Contributing to the Console

The console (`cmd/console`) is a lightweight Go web server with embedded HTML templates. It connects to one or more running Inrun runtimes via their `/blueprint` HTTP API to present what is running, and — for CRDs with `serve.enabled: true` — to a companion gateway's Gateway API to let a developer create new instances through a form.

---

## Current state

The console currently surfaces:

| Page | Path | What it shows |
|------|------|---------------|
| Dashboard | `/` | All connected Blueprints — health, CRD count, worker count, resource count |
| Blueprint panel | `/blueprint/{name}` | CRD list for one Blueprint, status per CRD |
| CRD detail | `/blueprint/{name}/crd/{crd}` | CRD config, worker count, recent reconcile status |
| CR list | `/blueprint/{name}/crd/{crd}/cr` | All custom resource instances |
| CR detail | `/blueprint/{name}/crd/{crd}/cr/{ns}/{name}` | One CR — spec, status, children, events |
| IDP form | `/blueprint/{name}/crd/{crd}/cr/create` | Self-service create form for CRDs with `serve.enabled: true` — submits target mode to the gateway, builds no CR itself |
| In-package docs | `/docs` | Rendered inrun-docs for the connected operator |
| Metrics | `/metrics` | Metrics page — template exists, data not yet wired |
| Runtime manager | (conditional) | Add / remove / update connected runtime instances |

---

## Where contribution is needed

### Metrics page

The `/metrics` route renders `metrics.html` but currently passes `nil` data. The runtime exposes metrics via `pkg/metrics` (Prometheus counters and gauges on reconcile cycles, errors, webhook calls, queue depth). The work here is:

1. Add a metrics endpoint to the runtime's HTTP API (or query Prometheus directly).
2. Populate a `MetricsData` struct in `handleMetricsPage`.
3. Render charts or a summary table in `metrics.html`.

This is one of the highest-value additions — operators running in production need to see reconcile rates, error rates, and queue depth at a glance.

### CR detail — richer status

The CR detail page shows spec and status but does not yet show:
- Error state (how many consecutive failures?)
- Condition history (not just current conditions)

All of this data is available from the runtime API — it needs to be threaded through `handleCRDetail` and rendered in `cr_detail.html`.

### CRD health timeline

The CRD detail page shows current health but not history. A small time-series chart of reconcile success/failure over the last hour would make degradation and recovery visible.

### Multi-instance diff view

When two runtime instances are connected, there is no view that compares them. A diff view showing which Blueprints / CRDs differ between instances would help multi-cluster operators.

### Gateway status

When a Blueprint has a `gatewayEndpoint`, the console knows the gateway URL from the `/blueprint` response. It does not currently show webhook stats (admission calls, conversion calls, deletion protection blocks). The gateway exposes these on `/webhook-stats`.

### IDP mode — follow-on improvements

When `gateway.api.enabled: true`, the console renders a **[+ Create]** button per CRD and a form generated from the gateway's flat field schema (`GET {gatewayEndpoint}/api/v1/schema?target=<target>`) — one input per declared `serve.fields`/`serve labels/annotations` entry, no CRD or Kubernetes shape involved. The console never sees `spec`, `metadata`, or an OpenAPI schema; the gateway resolves all of that server-side from `target`. See [Target Mode](../concepts/self-service/02-target-mode.md).

Per-CRD, per-token authorization already exists (`serve.tokens` — which operations, in which namespaces, for a given token) and needs no contribution here. See [Token Scoping](../concepts/self-service/03-token-scoping.md). What's still open:

**OIDC authentication**

Today's tokens are static bearer values (`gateway.api.auth.tokens`). OIDC would let the console use the user's existing session as the Gateway API credential — no separate token needed. The gateway would validate the OIDC token against the configured issuer. See [05-auth.md](../../pkg/gateway/api/docs/05-auth.md) for the current auth pipeline this would extend.

**Service account token review**

For in-cluster callers (CI pipelines running inside the cluster), the gateway can validate Kubernetes service account tokens via the `TokenReview` API. This removes the need for any static token configuration for in-cluster use cases.

---

## Development setup

The console is a separate Go module at the root of `cmd/console/`.

```bash
# Run against a live runtime
INRUN_RUNTIME_URLS=http://localhost:8080 go run ./cmd/console

# Run in NO_LOGIN mode (no auth, useful during local dev)
NO_LOGIN=true INRUN_RUNTIME_URLS=http://localhost:8080 go run ./cmd/console
```

Templates live in `cmd/console/web/assets/templates/`. They use Go `html/template` with a shared `_partials.html` for nav, header, and footer.

CSS is in `assets/static/css/style.css`. The console supports light and dark themes via a `data-theme` attribute.

---

## Adding a new page

1. Add a handler function `func (c *Console) handleMyPage(...)` in `console.go`.
2. Register the route in `ServeHTTP`.
3. Create `assets/templates/mypage.html` extending `_partials.html`.
4. Call `c.renderTemplate(w, "mypage.html", data)`.

Keep handlers thin — data collection belongs in helper methods, not inline in the handler.
