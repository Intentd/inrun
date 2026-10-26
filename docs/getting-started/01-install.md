# Install

## The CLI

With Homebrew, `inrun` and the console:

```bash
brew install intentd/tap/inrun intentd/tap/inrun-console
```

Or the installer, which downloads the latest release for your OS and architecture, checks it against the release's checksums, and puts `inrun` and `inrun-console` in `~/.inrun/bin`. With [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) installed, it also checks that the checksums were signed by Inrun's release workflow:

```bash
curl -sSL https://get.inrun.dev | bash
```

| Variable | Default | Effect |
| --- | --- | --- |
| `INRUN_VERSION` | latest release | Install a specific tag, e.g. `v0.1.0` |
| `INRUN_INSTALL_DIR` | `~/.inrun/bin` | Where the binaries go |
| `INRUN_SKIP_CONSOLE` | `false` | Skip `inrun-console` |
| `INRUN_SKIP_COMPLETION` | `false` | Skip shell completion |
| `INRUN_REQUIRE_SIGNATURE` | `false` | Fail when cosign is not installed |

With the installer, open a new shell (it adds the directory to your `PATH`). Then:

```bash
inrun version
```

`inrun upgrade` moves to a newer release; `inrun uninstall` removes the CLI and its cache.

## Editor support

Blueprints, Stacks, Modules and the `simulate` and `e2e` files have JSON
Schemas, so an editor completes fields, shows each one's description on
hover, and flags unknown fields and wrong types as you type. In VS Code,
install the [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml);
JetBrains IDEs and editors using `yaml-language-server` support it already.

Files from `inrun init` start with a line that names their schema:

```yaml
# yaml-language-server: $schema=https://inrun.dev/schema/v1/blueprint.json
apiVersion: inrun.dev/v1
kind: Blueprint
```

Add the same line to files you write yourself, with `stack.json`,
`module.json`, `simulate.json` or `e2e.json` for the other kinds.

## Verifying a release by hand

Releases are signed keyless: the release workflow signs `checksums.txt` with its GitHub identity, and the signature and certificate are published as `checksums.txt.sigstore.json`. There is no key to download. For a release `v0.1.0`:

```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity https://github.com/intentd/inrun/.github/workflows/sign-and-release.yml@refs/tags/v0.1.0 \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum -c checksums.txt --ignore-missing
```

## From source

With Go 1.26 or later:

```bash
git clone https://github.com/intentd/inrun && cd inrun
```

```bash
go build -o ~/.inrun/bin/inrun ./cmd/inrun
```

## In a cluster

The Helm chart installs the runtime, the gateway and the console. The CLI is enough to try everything in [Learning Inrun](02-learning-inrun.md); the chart is for running Inrun as a service. See the [Helm repository](https://intentd.github.io/inrun/).
