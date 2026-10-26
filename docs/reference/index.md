# Reference

Technical reference for the Inrun runtime, schemas, functions and CLI.

!!! tip "Writing a resource declaration?"
    See the [Resources reference](./schema/resources/index.md): one page per Kubernetes built-in and custom resource kind, with every field and a worked YAML example.

---

## Schema

| Page | Description |
|------|-------------|
| [Resources](./schema/resources/index.md) | Every Kubernetes built-in and custom resource declarable under `onCreate` / `onReconcile` / `onDelete`, one page per kind |
| [E2E](./schema/e2e/complete-example.md) | A complete `e2e.yaml`: setup, steps and assertions |

The Blueprint's own blocks are described in [The Blueprint](../concepts/01-blueprint.md).

---

## Functions

[Functions](./inrun-functions/index.md) are what every `{{ }}` expression in a
Blueprint can call, one page per domain. They come from
[kel](https://github.com/intentd/kel); `inrun functions` searches them from the
CLI.

---

## CLI

Every command documents its flags with `--help`.

| Command | Description |
|---------|-------------|
| `inrun` | Run the runtime and gateway against the current cluster (`--dev` creates a kind cluster) |
| `inrun init` | Scaffold an operator, or copy an example pack (`--pack`) |
| `inrun validate` | Validate a Blueprint, Stack, Module, E2E or Simulate file; `--full` prints the RBAC it needs |
| `inrun simulate` | Reconcile in memory, or against envtest (`--envtest`) |
| `inrun e2e` | Run declarative end-to-end tests on kind |
| `inrun template` | Render the fully expanded Blueprint the runtime would run |
| `inrun migrate` | Move stored CRs to a new field path or a new CRD; `rollback` and `clean` |
| `inrun patterns`, `inspect`, `pull`, `push` | Find, read, fetch and publish patterns in a registry |
| `inrun serve play` | Run an intent through the gateway's stages locally |
| `inrun serve view` | Show the view a serve target returns |
| `inrun token` | List and check gateway tokens |
| `inrun gate` | Evaluate admission rules against a CR, with no cluster |
| `inrun generate` | Generate RBAC, a ConfigMap, a bundle or the type registry |
| `inrun functions` | Browse and search the template functions |
| `inrun console` | Start the console |

---

## Where to go next

- **[Concepts](../concepts/index.md)**: what each part of a Blueprint does and why
- **[Learning Inrun](../getting-started/02-learning-inrun.md)**: the example packs, in order
