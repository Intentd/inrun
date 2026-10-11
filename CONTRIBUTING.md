# Contributing to Inrun

Thank you for your interest in contributing to Inrun.

Inrun is the intent runner for Kubernetes. A caller sends an intent through the gateway; Inrun turns it into a custom resource, reconciles it, and returns a view.

---

## Start here

Full contribution guides live in [`docs/contributing/`](docs/contributing/):

| Guide | What it covers |
|-------|---------------|
| [index](docs/contributing/index.md) | Overview and where to go |
| [Codebase map](docs/contributing/codebase-map.md) | Which packages belong to which binary; how to navigate the code |
| [Resources](docs/contributing/contributing-resources.md) | Add or improve resource types in `pkg/resources` |
| [Console](docs/contributing/contributing-console.md) | Improve the web UI — metrics, CR status, multi-instance views |
| [Examples](docs/contributing/contributing-examples.md) | Add example operator packs |
| [Publishing a new pack](docs/contributing/publishing-a-new-pack.md) | Exact checklist for adding a new examples pack without breaking CI |

---

## Quick start

```bash
# Clone
git clone https://github.com/intentd/inrun.git
cd inrun

# Build (runs codegen + gofmt + go build)
make inrun

# Test
make test
```

---

## Pull requests

1. Fork and create a branch from `main`.
2. Read the relevant guide above.
3. Run `make test-race` and confirm it passes (vet + unit tests + race detector).
4. Open a PR with a short description of what and why. Link any related issues.

---

## Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(registry): add DaemonSet handler
fix(reconciler): correct window-based rollback trigger
docs(contributing): add resources contribution guide
```

---

## Code style

- `gofmt` is enforced by CI.
- Exported functions must have a doc comment.
- Wrap errors with context: `fmt.Errorf("creating deployment: %w", err)`.
- No comments that describe *what* the code does — only *why* when the reason is non-obvious.

---

## Code of Conduct

This project follows the [Inrun Code of Conduct](CODE_OF_CONDUCT.md). Report unacceptable behaviour to conduct@inrun.dev.

---

## Questions?

Open a [GitHub Discussion](https://github.com/intentd/inrun/discussions).
