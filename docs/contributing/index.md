# Contributing to Inrun

Inrun is the Kubernetes operator runtime. Operators are described as YAML — the runtime, gateway, and console turn those descriptions into running reconciliation loops, webhook servers, and an observable UI.

This section is for people who want to extend, improve, or fix Inrun itself.

---

## Where to go first

If you are new to the codebase, the best entry point is understanding how the project is laid out and which binary owns what. Start here:

→ [Codebase map](codebase-map.md) — which packages belong to the runtime, gateway, console, and shared layer; where to read and where to start

Once oriented, pick the area you want to work in:

| Area | Guide |
|------|-------|
| Add a new resource type | [Inrun resources](contributing-resources.md) |
| Improve the console UI | [console](contributing-console.md) |
| Add or improve an example pack | [examples](contributing-examples.md) |
| Add a note function to make operators more declarative (`pkg/note`) | Add to `pkg/note/<domain>.go`, register in `buildNotes()` in `pkg/note/note.go` |

---

## Build and test commands

| Command | What it does |
|---------|-------------|
| `make inrun` | Codegen (note list, e2e example doc) + gofmt + build `inrun` binary to `~/.inrun/bin/` |
| `make test` | `go vet` + unit tests — fast, no cluster required |
| `make test-race` | Same with Go's race detector — run before every PR |
| `make test-unit` | Unit tests only, skipping vet |
| `make test-integration` | Integration tests via envtest (requires `setup-envtest`) |
| `make test-coverage` | HTML coverage report written to `coverage.html` |
| `make docs` | Start Hugo dev server at `localhost:8090` (requires `make hugo-install` once) |

> **`go build ./...` is not a substitute for `make inrun`.** The build step runs `hack/generate-notes` first — skipping it leaves `pkg/note/list_generated.go` stale, which breaks `inrun notes`.

## General contribution workflow

1. Fork the repository and create a branch from `main`.
2. Read the guide for the area you are changing.
3. Run `make inrun` after any Go changes.
4. Run `make test-race` to confirm nothing is broken.
5. Open a pull request with a short description of what and why.

Questions? Open a [GitHub Discussion](https://github.com/intentd/inrun/discussions).
