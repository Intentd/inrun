# Gateway and intents

Callers should not need to know a CRD. The gateway lets them send an **intent** (a small JSON or YAML payload in their own field names, with no `apiVersion`, `kind` or `spec`) and read back a **view** of the result.

```text
caller ──intent──▶ gateway ──▶ CR ──▶ runtime ──▶ reconciler
caller ◀──view──── gateway ◀── status
```

## Serving a CRD

`serve` on a CRD entry makes it reachable through the gateway:

```yaml
spec:
  crds:
    website:
      serve:
        enabled: true
        namespace: default
        fields:
          image: {}
          replicas: {type: integer}
        view:
          fullObject: false
          payload:
            name: '{{ .metadata.name }}'
            phase: '{{ .status.phase }}'
          poll:
            field: status.phase
```

- **`fields`** are the intent's vocabulary: what a caller may send, its type, and where it lands in the CR.
- **`view`** is what comes back: its `payload` and, with `fullObject: false`, nothing else, so the CR's full shape never leaves the cluster.
- **`poll`** tells the caller when the view is final.
- **Serve targets** (`serve.target`) let one CRD offer several shapes; each target can carry its own fields, view and `reconcile`, and the caller picks one with the intent.

## The API

`gateway.api.enabled` turns it on; tokens are declared in the Blueprint (`gateway.api.auth.tokens`, values from the environment or OIDC).

| Call | Does |
| --- | --- |
| `POST /api/v1/apply` | Sends an intent; returns `accepted` and a poll URL. `?dryRun=true` previews it |
| `GET /api/v1/resources/{kind}/{namespace}/{name}` | Reads the view |
| `DELETE /api/v1/resources/{kind}/{namespace}/{name}` | Deletes through the gateway |
| `GET /api/v1/schema` | The served targets; `?target=<name>` returns one target's fields |

Admission runs on every intent: the CRD's validation and mutation rules apply
before a CR is written, so a caller gets the same answer the cluster would
give.

## Locally

```bash
inrun token list
```

```bash
inrun serve play -i intent.json --token dev
```

`serve play` runs an intent through every stage (target, token, CR
construction, admission, response) and prints the CR it becomes, with no
cluster. `inrun serve apply` sends one to a running gateway.

## Where the gateway runs

In a cluster it is its own Deployment (stateless, several replicas), and it
also serves the admission and conversion webhooks over TLS. Outside a pod
(`inrun` on your machine) it serves the API on plain HTTP and leaves the
webhooks out. The console sends its intents through the same API.

## Try it

```bash
inrun init --pack intent
```
