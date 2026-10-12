# Intent

The intent loop end to end. A caller sends a flat intent to the gateway, which
turns it into a `Website` CR. The Catalog maps the CR to flat args for a Python
reconciler ([`reconciler/main.py`](./reconciler/main.py)), whose result
Inrun applies. The caller reads back a view.

```console
caller ──intent──▶ gateway ──▶ Website CR ──args──▶ reconciler

caller ◀──view──── gateway ◀── status ◀────result── reconciler
```

The caller never writes `apiVersion`, `kind` or `spec`, and the reconciler
never reads the CR: each side sees only its own flat fields, and
`catalog.yaml` maps between them.

## Without a cluster

### List the tokens

The tokens the gateway accepts, declared in `catalog.yaml`.

```bash
inrun token list
```

### Play the intent

Runs the intent through the gateway's stages (target, token, CR construction,
admission, response) and prints the CR it becomes. The intent can be JSON or
YAML: `intent.json` and `intent.yaml` are the same intent, and either works
anywhere below.

```bash
inrun serve play -i intent.json --token dev
```

### Simulate

`test/simulate.yaml` names the intent instead of a CR: simulate builds the CR the same way, reconciles it against an in-memory cluster, and checks the result. Each cycle calls the reconciler, so start it first, in a second terminal:

```bash
RECONCILER_TOKEN=inrun-demo-token python3 reconciler/main.py
```

```bash
inrun simulate
```

Add `--envtest` to run against a real API server instead.

## On a cluster

### 1. Set the tokens

The value the `dev` token reads from `${INTENT_TOKEN}`, and the same value for the console, which creates resources through the gateway.

```bash
export INTENT_TOKEN=dev-token
export GATEWAY_TOKEN=$INTENT_TOKEN      # used by the console to send intents
```

### 2. Start the reconciler

Skip if it is still running from the simulate step. Listens on :8025.

```bash
RECONCILER_TOKEN=inrun-demo-token python3 reconciler/main.py
```

### 3. Start Inrun

In a second terminal, with the same tokens. Applies the CRD and the reconciler's token Secret, and starts the runtime on :8080. Because the Catalog serves a CRD, it also starts the gateway on :8082 (`INRUN_GATEWAY_PORT` changes it) and the console on :8081.

```bash
inrun -f catalog.yaml
```

### 4. Send the intent

Returns `accepted: true` and a `pollUrl`. Add `?dryRun=true` to preview without writing.

```bash
curl -s -X POST localhost:8082/api/v1/apply \
  -H "Authorization: Bearer $INTENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d @intent.json
```

The gateway takes the YAML form too. Send it with `--data-binary`, which keeps
the newlines YAML needs (`-d` strips them):

```bash
curl -s -X POST localhost:8082/api/v1/apply \
  -H "Authorization: Bearer $INTENT_TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @intent.yaml
```

The CLI can send the same intent:

```bash
inrun serve apply -f intent.json -a http://localhost:8082 -t $INTENT_TOKEN
```

### 5. Read the view

Returns only what `serve.view` declares: `name`, `image`, `phase`
and `endpoint`. `phase` reads `Running` once the runtime has reconciled.

```bash
curl -s localhost:8082/api/v1/resources/website/default/hello-intent \
  -H "Authorization: Bearer $INTENT_TOKEN"
```

Or open the console at [localhost:8081](http://localhost:8081) and log in
with `inrun` / `inrun`: the Website, its status, and the runtime's and
gateway's health are there.

### 6. Create one from the console

In the console, open the `website` CRD and choose **+ Create**. Enter a name,
say `hello-console`, and an image, then choose **Create Website**. The console
sends it through the gateway, the same path as the intent in step 4. Read its
view the same way:

```bash
curl -s localhost:8082/api/v1/resources/website/default/hello-console \
  -H "Authorization: Bearer $INTENT_TOKEN"
```

### 7. Clean up

Stop `inrun` and the reconciler after the script deletes the Websites; it needs the runtime running to remove its finalizer.

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

### 8. E2E

Runs the whole loop in a kind cluster, with the reconciler deployed from
[`reconciler/deploy.yaml`](./reconciler/deploy.yaml): sends the intent to the
in-cluster gateway, reads the view back, and deletes through the gateway.

```bash
inrun e2e
```
