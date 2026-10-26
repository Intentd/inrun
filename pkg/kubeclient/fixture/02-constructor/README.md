# Args via Constructor — BlockchainNode

A Go reconciler driven by args alone. The Blueprint maps the CR's spec into
args, and the constructor reads only those — never the spec — the same contract
a remote reconciler has. The constructor owns its full reconcile loop, including
the time check and the HTTP call; the Blueprint declares *what* to use and
*when*, the constructor decides *how*:

```yaml
reconcile:
  args:
    image: '{{ .spec.image }}'
    replicas: '{{ .spec.replicas }}'
    network: '{{ .spec.network }}'
    flagUrl: '{{ .spec.serviceUrl }}/flags/{{ .metadata.name }}/v2Enabled'
    businessHoursStart: "09:00"
    businessHoursEnd: "18:00"
```

The reconciler binds the args into its own input type — not the CRD's:

```go
type nodeArgs struct {
	Image              string `json:"image"`
	Replicas           int32  `json:"replicas"`
	Network            string `json:"network"`
	FlagURL            string `json:"flagUrl"`
	BusinessHoursStart string `json:"businessHoursStart"`
	BusinessHoursEnd   string `json:"businessHoursEnd"`
}

var args nodeArgs
if err := req.Args().Bind(&args); err != nil { ... }

featureEnabled := r.inBusinessHours(args.BusinessHoursStart, args.BusinessHoursEnd) &&
	r.checkFlag(ctx, args.FlagURL)

spec := deployments.Spec{Image: args.Image, Replicas: args.Replicas, ...}
```

Rendered args arrive as native values, so `replicas` binds to a number.
`req.Args().String(...)` and friends read single keys without a struct.

The CR itself (`req.Object()`) is only the Deployment's owner and the
status patch's target. Rename a spec field and only the Blueprint's args change.

The constructor checks the clock itself and makes the HTTP call itself — but the
window and the URL pattern come from the Blueprint, not from the binary. Change the
business-hours window or the flag endpoint in YAML; the binary stays the same.

This is the difference from hooks: the runtime evaluates the note and makes the
external call for hooks. The constructor owns those steps — the Blueprint hands it
the configuration to make its own decisions.

**Requirement:** `inrun` CLI — install from [inrun-install](https://github.com/intentd/inrun#getting-started)

---

## Step 1 — Generate the registry

```bash
make registry
```

## Step 2 — Build

```bash
make clean && make build
inrun validate blueprint.yaml
inrun simulate --dev-server
```

## Step 3 — Run

```bash
inrun --dev-server

kubectl get deployment 02-constructor-my-node \
  -o jsonpath='{.metadata.annotations.feature\.demo/v2-enabled}' && echo

kubectl get blockchainnode 02-constructor-my-node \
  -o jsonpath='{.status.featureEnabled}' && echo

kubectl get blockchainnode 02-constructor-my-node \
  -o jsonpath='{.status.inBusinessHours}' && echo
```

> Outside business hours the constructor skips the flag call and the annotation
> is `"false"`. Inside hours with the dev server running it is `"true"`.

## E2E

```bash
make docker push IMAGE_REPO=yourregistry/blockchainnode-operator IMAGE_TAG=latest

inrun e2e --dev-server \
  --set runtime.image.repository=yourregistry/blockchainnode-operator \
  --set runtime.image.tag=latest
```

## Cleanup

```bash
./cleanup.sh
```
