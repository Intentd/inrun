# prometheus

Verifies `protocol: prometheus` using Inrun's own metrics server as the
Prometheus query target.

Inrun exposes `/api/v1/query` on the `inrun-runtime` health server (`:8080`). The endpoint
gathers from `prometheus.DefaultGatherer` and answers simple metric-name
lookups in the standard Prometheus instant query response format.

Two calls are declared:

- `goroutines` — queries `go_goroutines`. Always present on a running Go
  process. `cacheFor:` is not set so it re-fetches every reconcile.
- `cpuTotal` — queries `process_cpu_seconds_total`. Has `cacheFor: 10s` to
  verify the cache layer.

Status fields exercise:

| Field | Note function | What it proves |
|---|---|---|
| `goroutineCount` | `promValue` | scalar extracted from first series |
| `goroutinesAboveOne` | `promAboveThreshold` | threshold comparison |
| `goroutineSeriesCount` | `promSeriesCount` | series count from vector |
| `cpuSeconds` | `promValue` | counter value from cached call |

## Run

```sh
inrun e2e pkg/external/fixtures/prometheus/e2e.yaml
```

No `--dev-server` needed — the fixture queries the operator's own health server.
