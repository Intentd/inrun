# Args — Access to External Systems for Typed Operators

Declarative operators in Inrun have always had access to `external:`,
`cross:`, notes, and the full resolver context. Typed operators — hooks and
constructors — run Go code but were isolated from that context.

`args:` bridges the gap. Template expressions in `args:` are resolved against
the full per-CR context before the hook or constructor runs:

```yaml
reconcile:
  external:
    - name: flags
      url: "{{ .spec.serviceUrl }}/flags/{{ .metadata.name }}/v2Enabled"
  args:
    featureEnabled: '{{ .external.flags.body }}'
```

The hook reads `req.Args.String("featureEnabled")` — no HTTP client,
no flag service SDK, no environment wiring. The Catalog handles the IO.

This fixture composes two self-contained examples showing the same access
story with the two typed operator patterns:

| Sub-directory | CRD | Pattern | World state | External call |
|---|---|---|---|---|
| [`01-hooks/`](01-hooks/README.md) | `BlockchainApp` | hooks | note evaluated by runtime → passed via args | runtime calls flag service when in business hours |
| [`02-constructor/`](02-constructor/README.md) | `BlockchainNode` | constructor | constructor checks clock using window from args | constructor calls flag service when in business hours |
| [`03-hooks-targets/`](03-hooks-targets/README.md) | `BlockchainAppWithTargets` | hooks + per-target operatorBox | same hook binary; `featureEnabled` and gate vary by target surface | no HTTP call — flag value resolved from target args |

**Requirement:** `inrun` CLI — install from [inrun-install](https://github.com/inrundev/inrun#getting-started)

---

## Step 1 — Generate the registry

```bash
make registry
```

## Step 2 — Build and validate

```bash
make clean && make build
inrun validate 01-hooks/catalog.yaml
inrun validate 02-constructor/catalog.yaml
inrun simulate
```

## Step 3 — Run

```bash
inrun --dev-server
```

The dev server returns `"true"` for all flag requests. Both operators stamp the
result on their managed Deployment and on the CR status:

### hooks example
```bash
kubectl get deployment 01-hooks-my-chain \
  -o jsonpath='{.metadata.annotations.feature\.demo/v2-enabled}' && echo
```

```bash
kubectl get blockchainapp 01-hooks-my-chain \
  -o jsonpath='{.status.featureEnabled}' && echo
```

```bash
kubectl get blockchainapp 01-hooks-my-chain \
  -o jsonpath='{.status.inBusinessHours}' && echo
```

### constructors example
```bash
kubectl get deployment 02-constructor-my-node \
  -o jsonpath='{.metadata.annotations.feature\.demo/v2-enabled}' && echo
```

```bash
kubectl get blockchainnode 02-constructor-my-node \
  -o jsonpath='{.status.featureEnabled}' && echo
```

```bash
kubectl get blockchainnode 02-constructor-my-node \
  -o jsonpath='{.status.inBusinessHours}' && echo
```

## E2E

```bash
make docker push IMAGE_REPO=yourregistry/blockchain-operator IMAGE_TAG=latest

inrun e2e --dev-server \
  --set runtime.image.repository=yourregistry/blockchain-operator \
  --set runtime.image.tag=latest
```

## Cleanup

```bash
./cleanup.sh
```
