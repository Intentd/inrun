# Contributing

Inrun is young and built by very few people, so any help moves it forward: a bug report, a confusing doc page pointed out, an example, a fix. You don't need to know the codebase to help.

## Ways to help

- **Try it and tell us what broke.** Work through [Learning Inrun](../getting-started/02-learning-inrun.md) and open an [issue](https://github.com/intentd/inrun/issues) for anything that failed, surprised you or took too long to understand.
- **Improve the docs.** If a page left you guessing, a PR that fixes it is welcome, however small.
- **Add an example.** A Blueprint for something you run, with its `test/simulate.yaml`, teaches more than a page of prose. The existing packs under `examples/` show the layout.
- **Fix or build something.** Issues labelled `good first issue` are a place  to start; for anything bigger, open an issue or a [discussion](https://github.com/intentd/inrun/discussions) first so we can agree on the shape before you write it.

## Setting up

Go 1.26 or later, Docker and kind for the end-to-end tests.

```bash
git clone https://github.com/intentd/inrun && cd inrun
```

```bash
make test
```

`make inrun` builds the CLI (it regenerates the function list and the generated
docs first). Every example can be checked with `inrun simulate` from its
directory, and end to end with `inrun e2e`.

## Pull requests

1. Branch from `develop` and open the PR against `develop`.
2. Run `make test`; for behaviour changes, also `inrun simulate` and `inrun e2e` on the example that covers it.
3. Keep the PR to one change, with a short description of what and why.
4. Commit messages are one plain sentence saying what changed ("Add a DaemonSet resource"), with detail in the body when it helps.

## Make targets

| Command | What it does |
|---------|-------------|
| `make inrun` | Codegen (function list, e2e example doc) + gofmt + build `inrun` binary to `~/.inrun/bin/` |
| `make test` | `go vet` + unit tests — fast, no cluster required |
| `make test-race` | Same with Go's race detector — run before every PR |
| `make test-unit` | Unit tests only, skipping vet |
| `make test-integration` | Integration tests via envtest (requires `setup-envtest`) |
| `make test-coverage` | HTML coverage report written to `coverage.html` |
| `make docs` | Start Hugo dev server at `localhost:8090` (requires `make hugo-install` once) |

`gofmt` is enforced by CI. Comments explain why, not what.

## Conduct and security

This project follows the [Code of Conduct](https://github.com/intentd/inrun/blob/main/CODE_OF_CONDUCT.md).
Report a vulnerability privately, as described in
[SECURITY.md](https://github.com/intentd/inrun/blob/main/SECURITY.md), not in
an issue.
