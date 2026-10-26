# Console

A web view of one or more running runtimes: their Blueprints, CRDs, worker
pools, queues, custom resources and status. For a served CRD it also has a
form that sends an intent through the gateway, the same path any other
caller takes.

```text
console ──GET /blueprint──▶ runtime   (what is running, and the gateway's address)
console ──intent─────────▶ gateway   (the create form)
```

The console reads; it never writes to the cluster. It needs no RBAC and no
kubeconfig, only the runtimes' URLs.

## What it shows

| Page | Shows |
| --- | --- |
| Dashboard | Every connected Blueprint: health, CRD, worker and resource counts |
| Blueprint | Its CRDs, how each is reconciled, and each CRD's state |
| CRD | Its configuration, worker pool, queue and recent reconciles; the gateway's admission stats when there is one |
| Custom resources | Every CR of the CRD, then one CR's spec, status, children and events |
| Create | A form for a served CRD, one input per field the target accepts |
| Docs | The function reference, rendered |

The create form is built from the gateway's field list for a target
(`GET /api/v1/schema?target=<name>`). The console never sees a CRD schema,
`spec` or `metadata`: it sends the fields, and the gateway builds the CR and
runs admission, as for any intent.

## Connecting runtimes

Each runtime serves `/blueprint` on its health port. The console polls every
URL it is given and finds each runtime's gateway in the response, so a
gateway needs no configuration of its own.

```bash
inrun console --urls http://localhost:8080,http://localhost:8082
```

With the runtime manager on (the default), runtimes can also be added,
changed and removed from the console itself.

## Running it

On your machine (outside a pod), `inrun` starts the console next to the
runtime, on port 8081 (`INRUN_CONSOLE_PORT`), when the `inrun-console`
binary is installed. `inrun console` starts it alone; if the binary is
missing, it downloads the one from the same release into `~/.inrun/bin`.

In a cluster the Helm chart runs it as its own Deployment (`console.enabled`).
It is configured from the environment:

| Variable | Default | Does |
| --- | --- | --- |
| `INRUN_URLS` | — | Comma-separated runtime URLs |
| `PORT` | `8081` | Port to serve on |
| `REFRESH_INTERVAL` | `15s` | How often each runtime is polled |
| `GATEWAY_TOKEN` | — | Token the create form sends to the gateway |
| `ADMIN_USERNAME`, `ADMIN_PASSWORD` | `inrun`, `inrun` | The login; set both anywhere but your machine |
| `SESSION_SECRET` | — | Signs the session cookie |
| `NO_LOGIN` | `false` | Serves the console with no login |
| `ENABLE_RUNTIME_MANAGER` | `true` | Allows adding and removing runtimes from the UI |
| `PUBLIC_DEPLOYMENT` | `false` | Shorthand for `NO_LOGIN=true` and `ENABLE_RUNTIME_MANAGER=false` |

## Try it

```bash
inrun init --pack intent
```

Run the pack with `inrun`, open `http://localhost:8081`, and create a
resource from the served CRD's form.
