# Contributing Examples

Examples are the fastest way for new users to understand what Inrun can do. Each example is a self-contained operator project: a Blueprint, a CRD schema, a sample CR, and a README that explains the concept.

---

## Pack structure

Examples are grouped into packs by difficulty and topic:

```text
examples/
  beginner/
    01-hello-world/
    02-configmap-operator/
    ...
  intermediate/
    ...
  advanced/
    09-hooks/
    10-constructor/
    11-mixed-operator-pattern/
  security/
    ...
  use-cases/
    ...
```

Each example directory must contain:

```text
blueprint.yaml        — the Blueprint declaration
crd.yaml            — CRD schema for the managed resource
cr.yaml             — a sample CR to apply
README.md           — explains the concept, what to run, what to observe
cleanup.sh          — (optional) removes cluster resources after the demo
```

---

## Adding an example to an existing pack

1. Create the example directory under the right pack.
2. Add the required files.

No code changes are needed for examples within an existing pack.

---

## Adding a new pack

Adding a new pack (a new top-level directory under `examples/`) requires changes in four places. Miss any of them and CI fails.

See [Publishing a new pack](publishing-a-new-pack.md) for the exact checklist.

---

## What concepts need examples

The following Inrun features have no dedicated example yet:

- **Modules** — reusable resource building blocks assembled via `imports`
- **Stack** — multi-source Blueprint merge from Git, HTTP, and ConfigMap
- **Typed operators** — full typed-mode example with `inrun generate registry` and a custom Go type
- **Custom constructors** — `reconciler.default: false` with a user-written reconciler
- **Conversion webhooks** — serving multi-version CRDs through the gateway
- **Namespace protection** — preventing namespace deletion

Good examples are small (one concept per example), runnable with `kind`, and have a README that explains why you would want this, not just how to run it.

---

## README format

Follow this structure for each example README:

```markdown
# Example name

One sentence: what this example demonstrates.

## What it does

2-3 sentences describing the operator behaviour.

## Prerequisites

- kind / minikube / a real cluster
- inrun CLI installed

## Run it

\`\`\`bash
kubectl apply -f crd.yaml
inrun
# Inrun reads blueprint.yaml from the current directory and starts the runtime.
kubectl apply -f cr.yaml
\`\`\`

## What to observe

Describe what happens — what resources are created, what logs appear, what status fields change.

## Key Blueprint config

Highlight the relevant part of blueprint.yaml and explain why it is written that way.
```
