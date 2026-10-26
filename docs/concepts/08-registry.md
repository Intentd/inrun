# The registry

An operator built with Inrun is data, so what you share is data too. The registry distributes **behaviour, not binaries**: a pattern is a Blueprint or a Module with its CRD, its sample CR and its tests, pushed to an OCI registry. Whoever pulls it gets the whole operator declaration, versioned and immutable, and can run it on the stock `inrun`.

A pattern also carries its **proof**. `inrun push` runs the pattern's simulate and e2e tests first and records the results on the artifact, so a consumer can see whether it was verified, and when, before downloading anything.

## What a pattern is

| Kind | Primary file | Is |
| --- | --- | --- |
| **Blueprint** | `blueprint.yaml` | A complete operator: CRDs and how each is reconciled |
| **Module** | `module.yaml` | A reusable piece (resources with named inputs) that Blueprints import into a CRD entry |
| **Stack** | `stack.yaml` | A composition: several Blueprints and Modules, from the registry, files or Helm, run as one |

A pattern directory has the same layout as every example:

```text
blueprint.yaml        the primary file (or module.yaml)
manifests/crd.yaml    the CRD consumers apply
manifests/cr.yaml     a sample CR
test/simulate.yaml    the behavioural proof, no cluster
test/e2e.yaml         the cluster proof
```

Only the primary file is required; the rest strengthens what the artifact can show. A typed pattern also carries its `go.mod` and `Makefile`, and the consumer builds it with their own `inrun`.

## Publishing

```bash
docker login ghcr.io
```

```bash
inrun push .
```

`inrun push .` takes the name and version from the pattern's `metadata`, or `inrun push payments:v1.0.0 ./payments/` names them. Before anything is uploaded it runs two gates:

1. **simulate**: `test/simulate.yaml` against the in-memory cluster.
2. **e2e**: `test/e2e.yaml` on a fresh kind cluster (`--use-current` or `--cluster` reuses one).

A failing gate stops the push. A version that is already published is refused: bump `metadata.version`, or `--overwrite` to replace it (the artifact records the digest it replaced). `--no-simulate` and `--no-e2e` skip a gate, and `--force` pushes anyway; the artifact then records what did not pass. `--add-intent intent.json` also runs the intent through the gateway's stages and stores the result, so consumers see what an intent becomes.

The results travel as annotations on the artifact:

| Annotation | Holds |
| --- | --- |
| `io.inrun.pattern.name`, `.version`, `.kind`, `.author`, `.license`, `.tags` | What the pattern is |
| `io.inrun.simulate.status`, `.assertions`, `.duration` | The simulate gate |
| `io.inrun.e2e.status`, `.assertions`, `.duration`, `.runner` | The e2e gate |
| `io.inrun.blueprint.typed` | Whether it needs a typed build |
| `io.inrun.blueprint.deprecated`, `.deprecated.message` | Its deprecation, from `lifecycle` |
| `io.inrun.pattern.replaces` | The digest an `--overwrite` push replaced |
| `io.inrun.intent.target`, `.status` | The intent baked in with `--add-intent` |

## Consuming

```bash
inrun patterns --blueprints --tag database
```

```bash
inrun inspect postgres:v0.1.0
```

`inspect` reads the annotations without downloading the pattern: what it is, whether its gates passed, and whether it is deprecated. `--versions` lists recent versions with their simulate and e2e results, and `--view blueprint.yaml,simulate.yaml` prints files before you pull.

```bash
inrun pull postgres:v0.1.0
```

Pulled patterns are cached in `~/.inrun/registry`; `-o` extracts one into a directory, `--refresh` bypasses the cache, and `inrun pull -f stack.yaml` pulls every registry import a file declares.

A Blueprint imports a Module into a CRD's reconciler:

```yaml
spec:
  crds:
    database:
      operatorBox:
        reconcile:
          imports:
            - module: oci://ghcr.io/intentd/registry/patterns/modules/postgres:v0.1.0
              with:
                image: "{{ .spec.postgresImage }}"
```

and a Stack imports whole patterns from `imports.registry` (`url`, `version`).

## Registries

| Variable | Default |
| --- | --- |
| `INRUN_REGISTRY` | `ghcr.io/intentd/registry/patterns/blueprints` |
| `INRUN_MODULES_REGISTRY` | `ghcr.io/intentd/registry/patterns/modules` |

Set them to your own registry to publish and consume privately; a full `oci://` reference works anywhere a short `name:version` does. Credentials come from `~/.docker/config.json`, so `docker login` is the only setup.

## Lifecycle

A pattern says how far to trust it in a top-level `lifecycle` block. The tools read it: `inrun validate`, `push` and `inspect` show it, and `inrun` and `inrun gate run` refuse to start a deprecated pattern unless a Stack accepts it (below). The runtime in a cluster doesn't read it: by then the decision was made.

```yaml
lifecycle:
  maturity: beta                 # alpha, beta, stable or deprecated
  compatibility:
    kubernetes: ">=1.28"
    inrun: ">=0.1.0"
  deprecation:
    message: Use webapp-v2, which splits the Service into its own CRD.
    migratedTo: webapp-v2
    timeline:
      from: "2026-12-01"         # warn from this date
      to: "2027-03-01"           # end of life
```

A Stack decides what it accepts:

- `policy.lifecycle.minMaturity` sets a floor: an import below it fails `inrun validate` instead of warning.
- A deprecated pattern doesn't start: run directly, or imported by a Stack that hasn't acknowledged it. `inrun validate` warns and says how. The Stack acknowledges it under `lifecycle.accept.patterns` (by `name`, optionally `author` and a `version` range), so staying on a deprecated pattern is a decision, not a surprise.

## Try it

```bash
inrun init --pack registry
```
