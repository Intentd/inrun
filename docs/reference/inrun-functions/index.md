# inrun functions

Inrun's functions are pure transformation functions available in every `{{ }}` expression across a Blueprint — status fields, `when:` conditions, resource names, `onCreate`, `onReconcile`, mutation rules, conversion paths. They are the vocabulary of declarative operator behavior. Most come from [KEL](https://github.com/intentd/kel), the Kubernetes Expression Library; Inrun adds a few of its own.

**Contract:** pure (same input → same output), safe (nil/empty input never panics), stateless (no I/O, no side effects). Use blueprint `external` block or [hooks](../../concepts/02-reconcilers.md) for anything requiring external calls.

---

## Why functions matter

Without functions, a Blueprint can only copy field values as-is. With them, it can reason about them — fall back to a default when a field is absent, gate a deployment on a job completing, compute a name from multiple fields. Functions are what make declarative operators expressive enough to replace hand-written reconcilers for most use cases.

---

## Two functions to know first

### `default` — supply a fallback value

```yaml
status:
  fields:
    - path: replicas
      value: "{{ default .spec.replicas 1 }}"
```

If `spec.replicas` is absent or zero, the status field shows `1`. If it is set, it passes through unchanged.

### `jobSucceeded` — gate on a completed Job

```yaml
onReconcile:
  jobs:
    - name: "{{ .metadata.name }}-migration"
      image: "{{ .spec.image }}"
      command: "{{ .spec.command }}"

  deployments:
    - name: "{{ .metadata.name }}-api"
      image: "{{ .spec.image }}"
      when:
        - field: "{{ jobSucceeded .children.job }}"
          equals: "true"
```

The api deployment is not created or updated until `status.succeeded > 0` on the migration job.

---

## inrun functions — explore from your terminal

`inrun functions` is the fastest way to discover and learn about every function available in your version of Inrun.

**List all functions:**
```bash
inrun functions
```

**Search by keyword:**
```bash
inrun functions search job
```

**Show full detail and an example for one function:**
```bash
inrun functions show jobSucceeded
```

---

## All functions by domain

Each domain has its own reference page in this folder, generated from the
functions' doc comments: Kubernetes objects, pods, replicas, jobs, services and
more from KEL, and Inrun's own. `inrun functions` lists every function your
version of Inrun has.

---

## Adding a function

- **A question any Kubernetes project would ask** belongs in
  [KEL](https://github.com/intentd/kel): see its contributing guide.
- **A function only Inrun needs** goes in `pkg/template/funcs`, with a doc
  comment in KEL's format (description, `Keywords:`, `Example:`). Then run
  `make generate-functions` to update the registry and these pages.

**Contract:** handle nil/empty input with a safe zero value, not a panic. Return `(value, error)` for functions that can meaningfully fail; return just `value` for infallible ones.
