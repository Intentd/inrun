# deprecate

Retire the pattern you published. You can't take it away from the teams that run it, so you tell them: what to
use instead, and when it stops being supported. They keep running it only by accepting that on purpose.

## 1. Mark it deprecated

Add a `lifecycle` block to the [publish](../publish/blueprint.yaml) step's `blueprint.yaml`, next to `metadata`:

```yaml
lifecycle:
  maturity: deprecated
  deprecation:
    message: "WebService adds nothing deployment-stack doesn't. Use deployment-stack directly."
    migratedTo: ghcr.io/intentd/registry/patterns/blueprints/deployment-stack:v0.1.0
    timeline:
      from: "2026-10-26"   # warn from this date
      to: "2027-01-31"     # end of life
```

`inrun validate` shows what consumers will see:

```
⚠  DEPRECATION WARNING
  End of life in 97 days (2027-01-31).
  WebService adds nothing deployment-stack doesn't. Use deployment-stack directly.
  Migrate to:  ghcr.io/intentd/registry/patterns/blueprints/deployment-stack:v0.1.0
```

## 2. Publish the deprecation

From the publish folder, push the same version again:

```bash
cd ../publish
inrun push . --overwrite
```

`--overwrite` is needed because `v0.1.0` is already published. The gates run again, so the version keeps its proof;
`inrun inspect webservice:v0.1.0` now shows the deprecation, and records which digest this push replaced.

## 3. A deprecated pattern doesn't start on its own

```bash
inrun webservice:v0.1.0
```

```
✗ ⚠ This blueprint is deprecated and cannot start without explicit acknowledgement.
  End of life in 97 days (2027-01-31).
  WebService adds nothing deployment-stack doesn't. Use deployment-stack directly.
  Migrate to: ghcr.io/intentd/registry/patterns/blueprints/deployment-stack:v0.1.0

  To proceed — in your Stack:

    lifecycle:
      accept:
        patterns:
          - name: <pattern-name>
```

The author can deprecate a pattern but can't decide for the teams using it. That choice lives in the consumer's
Stack.

## 4. Accept it, as a consumer

[stack.yaml](stack.yaml) imports the pattern. Replace `<you>` in its `url` with your registry path, then:

```bash
cd ../deprecate
inrun validate -f stack.yaml
```

It warns, and says how to acknowledge it:

```
  To acknowledge this import, add it to lifecycle.accept.patterns in your Stack.
```

Uncomment the `lifecycle` block at the end of [stack.yaml](stack.yaml):

```yaml
lifecycle:
  accept:
    patterns:
      - name: webservice
```

Validate again and the warning is gone. Run it; the Stack installs the pattern's CRD before it starts:

```bash
inrun -f stack.yaml
```

In another terminal, apply the pattern's sample CR:

```bash
inrun inspect webservice:v0.1.0 --view cr.yaml | kubectl apply -f -
```

```bash
kubectl get webservice orders -o jsonpath='{.status.phase} {.status.endpoint}'
```

```
Running http://orders-svc.default.svc.cluster.local
```

The team runs the pattern knowingly, until the end-of-life date gives them a reason to move to `deployment-stack`.
To limit the acceptance to versions you have checked, add a range: `version: ">= 0.1.0, < 0.2.0"`.

## Clean up

While `inrun` is still running:

```bash
./cleanup.sh
```

Then stop `inrun` with Ctrl+C.
