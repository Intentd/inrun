# Args via Hooks — BlockchainApp

The CR tells you *what* to run. It cannot tell you *when* the business is open or
*whether* a feature flag is enabled. Those answers come from the world — and
`args:` is how they reach the hook.

The Catalog declares a user-defined note for business hours and an external call
for the feature flag. The runtime evaluates both before invoking the hook:

```yaml
notes:
  functions:
    - name: inBusinessHours
      expression: '{{ and weekday (timeInWindow "09:00" "18:00") }}'

reconcile:
  external:
    - name: flags
      url: "{{ .spec.serviceUrl }}/flags/{{ .metadata.name }}/v2Enabled"
      continueOnError: true
      when:
        - field: '{{ inBusinessHours }}'
          equals: "true"   # runtime skips the call outside business hours
  args:
    featureEnabled: '{{ .external.flags.body }}'
    inBusinessHours: '{{ inBusinessHours }}'
```

The hook reads both from `req.Args` — no time library, no HTTP client,
no flag-service SDK. When the business-hours window changes, only the Catalog
changes. The binary is identical across environments and schedules.

```go
inBusinessHours := req.Args.String("inBusinessHours") == "true"
featureEnabled  := req.Args.String("featureEnabled") == "true"

annotation := "false"
if inBusinessHours && featureEnabled {
    annotation = "true"
}
```

**Requirement:** `inrun` CLI — install from [inrun-install](https://github.com/inrundev/inrun#getting-started)

---

## Step 1 — Generate the registry

```bash
make registry
```

## Step 2 — Build

```bash
make clean && make build
inrun validate catalog.yaml
inrun simulate --dev-server
```

## Step 3 — Run

```bash
inrun --dev-server

kubectl get deployment 01-hooks-my-chain \
  -o jsonpath='{.metadata.annotations.feature\.demo/v2-enabled}' && echo

kubectl get blockchainapp 01-hooks-my-chain \
  -o jsonpath='{.status.featureEnabled}' && echo

kubectl get blockchainapp 01-hooks-my-chain \
  -o jsonpath='{.status.inBusinessHours}' && echo
```

> Outside business hours (Mon–Fri 09:00–18:00 UTC) the external call is skipped
> and the annotation is `"false"`. Inside hours with the dev server running it is `"true"`.

## E2E

```bash
make docker push IMAGE_REPO=yourregistry/blockchainapp-operator IMAGE_TAG=latest

inrun e2e --dev-server \
  --set runtime.image.repository=yourregistry/blockchainapp-operator \
  --set runtime.image.tag=latest
```

## Cleanup

```bash
./cleanup.sh
```
