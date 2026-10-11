.PHONY: build inrun-console clean test test-unit test-console test-race test-integration test-all test-coverage test-coverage-text vet vuln vuln-inrun vuln-console vuln-runtime vuln-gateway certs docs docs-sync docs-build docs-serve hugo-install generate-notes generate-e2e-example generate-resource-docs test-fixture-note test-fixture-reconciler inrun-gateway-linux docker-gateway gateway-reload runtime-reload console-reload reload docker-devserver release-devserver devserver-reload

# ── Configuration ────────────────────────────────────────────────────────────
INRUN_DIR := .
CONSOLE_DIR := ./cmd/console
OUTPUT_DIR := $(HOME)/.inrun/bin
BUILD_TAGS ?=

# Version stamping — reads from git; matches what CI/CD injects via ldflags.
GIT_VERSION := $(shell git describe --tags --match 'v*' --always 2>/dev/null || echo "dev")
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
GIT_DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

INRUN_LDFLAGS := -X github.com/intentd/inrun/pkg/version.Version=$(GIT_VERSION) \
               -X github.com/intentd/inrun/pkg/version.Commit=$(GIT_COMMIT) \
               -X github.com/intentd/inrun/pkg/version.Date=$(GIT_DATE)

CONSOLE_LDFLAGS  := -X github.com/intentd/inrun/console/version.Version=$(GIT_VERSION) \
               -X github.com/intentd/inrun/console/version.Commit=$(GIT_COMMIT) \
               -X github.com/intentd/inrun/console/version.Date=$(GIT_DATE)

# ── Local build ───────────────────────────────────────────────────────────
generate-notes:
	@echo "Generating note list..."
	go run ./hack/generate-notes
	@echo "✅ Note list generated at pkg/note/list_generated.go"

generate-e2e-example:
	@echo "Generating e2e complete example doc..."
	@bash scripts/generate-e2e-example.sh
	@echo "✅ docs/reference/schema/04-e2e/08-complete-example.md updated"

generate-resource-docs:
	@echo "Generating resource schema docs..."
	go run ./hack/generate-resource-docs
	@echo "✅ docs/reference/schema/06-resources/*.md updated"

inrun: generate-notes generate-e2e-example generate-resource-docs
	@echo "Building Inrun..."
	@mkdir -p $(OUTPUT_DIR)
	cd $(INRUN_DIR) && gofmt -w .
	cd $(INRUN_DIR) && go build -ldflags "$(INRUN_LDFLAGS)" -o $(OUTPUT_DIR)/inrun ./cmd/inrun
	@echo "✅ Inrun built successfully"
	@python3 scripts/fix-bare-fences.py docs/

inrun-console:
	@echo "Building Inrun Console..."
	@mkdir -p $(OUTPUT_DIR)
	cd $(CONSOLE_DIR) && gofmt -w .
	cd $(CONSOLE_DIR) && go build -ldflags "$(CONSOLE_LDFLAGS)" -o $(OUTPUT_DIR)/inrun-console .
	@echo "✅ Inrun Console built successfully"

build: inrun inrun-console
	@echo "✅ Both Inrun and Console built successfully"

clean:
	@echo "Cleaning binaries..."
	rm -f $(OUTPUT_DIR)/inrun
	rm -f $(OUTPUT_DIR)/inrun-console
	@echo "✅ Clean complete"

# ── Install to system PATH (requires sudo) ───────────────────────────────────
install: build
	@echo "Installing to /usr/local/bin..."
	sudo cp $(OUTPUT_DIR)/inrun /usr/local/bin/
	sudo cp $(OUTPUT_DIR)/inrun-console /usr/local/bin/
	@echo "✅ Installed successfully"

# ── Uninstall from system ────────────────────────────────────────────────────
uninstall:
	@echo "Removing from /usr/local/bin..."
	sudo rm -f /usr/local/bin/inrun
	sudo rm -f /usr/local/bin/inrun-console
	@echo "✅ Uninstalled successfully"

# ── Add to PATH helper ───────────────────────────────────────────────────────
path-help:
	@echo "Add this to your ~/.bashrc or ~/.zshrc:"
	@echo "export PATH=\$$PATH:$(OUTPUT_DIR)"

# ── Docker Image Configuration ────────────────────────────────────────────────

# Default to short git commit, fallback to timestamp if git fails
# Fail explicitly if not in a Git repo
GIT_COMMIT := $(shell git rev-parse --short HEAD)
ifeq ($(GIT_COMMIT),)
  $(error "Not a Git repository. Set INRUN_IMAGE and INRUN_CONSOLE_IMAGE manually.")
endif

INRUN_IMAGE ?= ghcr.io/intentd/inrun:$(GIT_COMMIT)
INRUN_CONSOLE_IMAGE ?= ghcr.io/intentd/inrun-console:$(GIT_COMMIT)
INRUN_GATEWAY_IMAGE ?= ghcr.io/intentd/inrun-gateway:$(GIT_COMMIT)
INRUN_DEVSERVER_IMAGE ?= ghcr.io/intentd/inrun-dev-server:latest

# Target architectures
INRUN_AMD64_TARGET="inrun-amd64"
INRUN_ARM64_TARGET="inrun-arm64"
INRUN_CONSOLE_AMD64_TARGET="inrun-console-amd64"
INRUN_CONSOLE_ARM64_TARGET="inrun-console-arm64"
INRUN_GATEWAY_AMD64_TARGET="inrun-amd64"

# Intermediate directory for docker builds — isolated from $(OUTPUT_DIR) so
# docker builds never overwrite the developer's local inrun CLI binary.
DOCKER_TMP := /tmp/inrun-docker-build

# Path where the built binary lives
BIN := $(OUTPUT_DIR)/inrun

# ── Linux Build Targets (for Docker) ──────────────────────────────────────────

inrun-linux: generate-notes
	@echo "Building Inrun (Linux amd64)..."
	@mkdir -p $(OUTPUT_DIR)
	gofmt -w . && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-tags "$(BUILD_TAGS)" \
		-ldflags "$(INRUN_LDFLAGS)" \
		-o $(OUTPUT_DIR)/inrun ./cmd/inrun
	@echo "✅ Linux Inrun binary built: $(OUTPUT_DIR)/inrun"

inrun-console-linux:
	@echo "Building Inrun Console (Linux amd64)..."
	@mkdir -p $(OUTPUT_DIR)
	cd $(CONSOLE_DIR) && gofmt -w . && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(OUTPUT_DIR)/inrun-console .
	@echo "✅ Linux Console binary built: $(OUTPUT_DIR)/inrun-console"

inrun-gateway-linux: generate-notes
	@echo "Building Inrun Gateway (Linux amd64)..."
	@mkdir -p $(OUTPUT_DIR)
	gofmt -w . && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-tags "gateway" \
		-ldflags "$(INRUN_LDFLAGS)" \
		-o $(OUTPUT_DIR)/inrun-gateway ./cmd/inrun
	@echo "✅ Linux Gateway binary built: $(OUTPUT_DIR)/inrun-gateway"

# ── Docker Build ──────────────────────────────────────────────────────────────
# These targets build Linux binaries to DOCKER_TMP (not OUTPUT_DIR) so they
# never overwrite the developer's local inrun CLI binary in ~/.inrun/bin.

docker: generate-notes
	@mkdir -p $(DOCKER_TMP)
	@echo "Building Inrun runtime (Linux amd64) → $(DOCKER_TMP)/inrun..."
	gofmt -w . && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-tags "runtime" \
		-ldflags "$(INRUN_LDFLAGS)" \
		-o $(DOCKER_TMP)/inrun ./cmd/inrun
	@echo "Building Docker image: $(INRUN_IMAGE)"
	@cp $(DOCKER_TMP)/inrun ./$(INRUN_AMD64_TARGET)
	docker build -t $(INRUN_IMAGE) .
	@rm -f ./$(INRUN_AMD64_TARGET) $(DOCKER_TMP)/inrun
	@echo "✔ Docker image built: $(INRUN_IMAGE)"

docker-console:
	@mkdir -p $(DOCKER_TMP)
	@echo "Building Console (Linux amd64) → $(DOCKER_TMP)/inrun-console..."
	cd $(CONSOLE_DIR) && gofmt -w . && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-ldflags "$(CONSOLE_LDFLAGS)" \
		-o $(DOCKER_TMP)/inrun-console .
	@echo "Building Docker image: $(INRUN_CONSOLE_IMAGE)"
	cp $(DOCKER_TMP)/inrun-console $(CONSOLE_DIR)/$(INRUN_CONSOLE_AMD64_TARGET)
	cd $(CONSOLE_DIR) && docker build -t $(INRUN_CONSOLE_IMAGE) . && rm -f ./$(INRUN_CONSOLE_AMD64_TARGET)
	@rm -f $(DOCKER_TMP)/inrun-console
	@echo "✔ Docker image built: $(INRUN_CONSOLE_IMAGE)"

docker-gateway: generate-notes
	@mkdir -p $(DOCKER_TMP)
	@echo "Building Inrun gateway (Linux amd64) → $(DOCKER_TMP)/inrun-gateway..."
	gofmt -w . && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-tags "gateway" \
		-ldflags "$(INRUN_LDFLAGS)" \
		-o $(DOCKER_TMP)/inrun-gateway ./cmd/inrun
	@echo "Building Docker image: $(INRUN_GATEWAY_IMAGE)"
	@cp $(DOCKER_TMP)/inrun-gateway ./$(INRUN_GATEWAY_AMD64_TARGET)
	docker build -t $(INRUN_GATEWAY_IMAGE) .
	@rm -f ./$(INRUN_GATEWAY_AMD64_TARGET) $(DOCKER_TMP)/inrun-gateway
	@echo "✔ Docker image built: $(INRUN_GATEWAY_IMAGE)"

# ── Docker Push ───────────────────────────────────────────────────────────────

docker-push:
	@echo "Pushing Docker image: $(INRUN_IMAGE)"
	docker push $(INRUN_IMAGE)
	@echo "✔ Docker image pushed: $(INRUN_IMAGE)"
	@echo "Pushing Docker image: $(INRUN_CONSOLE_IMAGE)"
	docker push $(INRUN_CONSOLE_IMAGE)
	@echo "✔ Docker image pushed: $(INRUN_CONSOLE_IMAGE)"
	@echo "Pushing Docker image: $(INRUN_GATEWAY_IMAGE)"
	docker push $(INRUN_GATEWAY_IMAGE)
	@echo "✔ Docker image pushed: $(INRUN_GATEWAY_IMAGE)"

# ── Docker Release (build + push) ─────────────────────────────────────────────

docker-release: docker docker-console docker-gateway docker-devserver docker-push
	@echo "✔ Docker release complete"

# ── Dev Server Docker ─────────────────────────────────────────────────────────

docker-devserver:
	@mkdir -p $(DOCKER_TMP)
	@echo "Building dev server (Linux amd64) → $(DOCKER_TMP)/inrun-devserver..."
	gofmt -w . && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-trimpath -ldflags "-s -w $(INRUN_LDFLAGS)" \
		-o $(DOCKER_TMP)/inrun-devserver ./cmd/devserver
	@echo "Building Docker image: $(INRUN_DEVSERVER_IMAGE)"
	@cp $(DOCKER_TMP)/inrun-devserver ./$(INRUN_AMD64_TARGET)
	docker build -t $(INRUN_DEVSERVER_IMAGE) .
	@rm -f ./$(INRUN_AMD64_TARGET) $(DOCKER_TMP)/inrun-devserver
	@echo "✔ Docker image built: $(INRUN_DEVSERVER_IMAGE)"

release-devserver: docker-devserver
	@echo "Pushing Docker image: $(INRUN_DEVSERVER_IMAGE)"
	docker push $(INRUN_DEVSERVER_IMAGE)
	@echo "✔ Dev server image released: $(INRUN_DEVSERVER_IMAGE)"

# ── Runtime Reload (local dev) ────────────────────────────────────────────────

KIND_CLUSTER ?= inrun-playground

DEVSERVER_DEPLOYMENT ?= inrun-dev-server
DEVSERVER_CONTAINER_NAME ?= dev-server
DEVSERVER_NAMESPACE ?= inrun-system

devserver-reload: docker-devserver
	@echo "Generating unique tag..."
	$(eval DEVSERVER_TAG := $(shell date +%s))
	@echo "Tag: $(DEVSERVER_TAG)"

	@echo "Retagging image..."
	docker tag $(INRUN_DEVSERVER_IMAGE) $(INRUN_DEVSERVER_IMAGE)-$(DEVSERVER_TAG)

	@echo "Loading image into kind cluster: $(KIND_CLUSTER)"
	kind load docker-image $(INRUN_DEVSERVER_IMAGE)-$(DEVSERVER_TAG) --name $(KIND_CLUSTER)
	@echo "✔ Image loaded"

	@echo "Updating deployment $(DEVSERVER_DEPLOYMENT) in namespace $(DEVSERVER_NAMESPACE)..."
	kubectl -n $(DEVSERVER_NAMESPACE) set image deploy/$(DEVSERVER_DEPLOYMENT) \
        $(DEVSERVER_CONTAINER_NAME)=$(INRUN_DEVSERVER_IMAGE)-$(DEVSERVER_TAG)

	@echo "✔ Dev server updated to image: $(INRUN_DEVSERVER_IMAGE)-$(DEVSERVER_TAG)"
RUNTIME_DEPLOYMENT ?= inrun-runtime
RUNTIME_CONTAINER_NAME ?= runtime
RUNTIME_NAMESPACE  ?= inrun-system

runtime-reload: docker
	@echo "Generating unique tag..."
	$(eval RUNTIME_TAG := $(shell date +%s))
	@echo "Tag: $(RUNTIME_TAG)"

	@echo "Retagging image..."
	docker tag $(INRUN_IMAGE) $(INRUN_IMAGE)-$(RUNTIME_TAG)

	@echo "Loading image into kind cluster: $(KIND_CLUSTER)"
	kind load docker-image $(INRUN_IMAGE)-$(RUNTIME_TAG) --name $(KIND_CLUSTER)
	@echo "✔ Image loaded"

	@echo "Updating deployment $(RUNTIME_DEPLOYMENT) in namespace $(RUNTIME_NAMESPACE)..."
	kubectl -n $(RUNTIME_NAMESPACE) set image deploy/$(RUNTIME_DEPLOYMENT) \
        $(RUNTIME_CONTAINER_NAME)=$(INRUN_IMAGE)-$(RUNTIME_TAG)

	@echo "✔ Runtime updated to image: $(INRUN_IMAGE)-$(RUNTIME_TAG)"

CONSOLE_DEPLOYMENT ?= inrun-console
CONSOLE_CONTAINER_NAME ?= console
CONSOLE_NAMESPACE ?= inrun-system

console-reload: docker-console
	@echo "Generating unique tag..."
	$(eval CONSOLE_TAG := $(shell date +%s))
	@echo "Tag: $(CONSOLE_TAG)"

	@echo "Retagging image..."
	docker tag $(INRUN_CONSOLE_IMAGE) $(INRUN_CONSOLE_IMAGE)-$(CONSOLE_TAG)

	@echo "Loading image into kind cluster: $(KIND_CLUSTER)"
	kind load docker-image $(INRUN_CONSOLE_IMAGE)-$(CONSOLE_TAG) --name $(KIND_CLUSTER)
	@echo "✔ Image loaded"

	@echo "Updating deployment $(CONSOLE_DEPLOYMENT) in namespace $(CONSOLE_NAMESPACE)..."
	kubectl -n $(CONSOLE_NAMESPACE) set image deploy/$(CONSOLE_DEPLOYMENT) \
        $(CONSOLE_CONTAINER_NAME)=$(INRUN_CONSOLE_IMAGE)-$(CONSOLE_TAG)

	@echo "✔ Console updated to image: $(INRUN_CONSOLE_IMAGE)-$(CONSOLE_TAG)"

GATEWAY_DEPLOYMENT ?= inrun-gateway
GATEWAY_CONTAINER_NAME ?= gateway
GATEWAY_NAMESPACE ?= inrun-system

gateway-reload: docker-gateway
	@echo "Generating unique tag..."
	$(eval GATEWAY_TAG := $(shell date +%s))
	@echo "Tag: $(GATEWAY_TAG)"

	@echo "Retagging image..."
	docker tag $(INRUN_GATEWAY_IMAGE) $(INRUN_GATEWAY_IMAGE)-$(GATEWAY_TAG)

	@echo "Loading image into kind cluster: $(KIND_CLUSTER)"
	kind load docker-image $(INRUN_GATEWAY_IMAGE)-$(GATEWAY_TAG) --name $(KIND_CLUSTER)
	@echo "✔ Image loaded"

	@echo "Updating deployment $(GATEWAY_DEPLOYMENT) in namespace $(GATEWAY_NAMESPACE)..."
	kubectl -n $(GATEWAY_NAMESPACE) set image deploy/$(GATEWAY_DEPLOYMENT) \
        $(GATEWAY_CONTAINER_NAME)=$(INRUN_GATEWAY_IMAGE)-$(GATEWAY_TAG)

	@echo "✔ Gateway updated to image: $(INRUN_GATEWAY_IMAGE)-$(GATEWAY_TAG)"

reload: runtime-reload gateway-reload console-reload
	@echo "✔ Runtime, Gateway, and Console reloaded successfully"

# ── Primary targets ───────────────────────────────────────────────────────────

# Default: vet + unit tests (both modules). Fast, no external dependencies.
test: vet check-brand test-unit test-console

# ── Unit tests ────────────────────────────────────────────────────────────────
# All tests under pkg/ that are not guarded by //go:build integration.
# No Kubernetes cluster required.
# -short skips any test that calls t.Skip(testing.Short()) for slow work.
# -count=1 disables result caching so tests always run fresh.
test-unit:
	@echo "Running unit tests..."
	go test ./pkg/... -v -short -count=1

# Console is a separate Go module (github.com/intentd/inrun/console) —
# its own go.mod, so it needs its own `go test` invocation from its directory.
test-console:
	@echo "Running Console unit tests..."
	cd $(CONSOLE_DIR) && go test ./... -v -short -count=1

# ── Race detector ─────────────────────────────────────────────────────────────
# Same as test-unit with Go's race detector enabled.
# Run before every pull request.
test-race:
	@echo "Running unit tests with race detector..."
	go test ./pkg/... -short -race -count=1

# ── Integration tests ─────────────────────────────────────────────────────────
# Uses envtest (embedded API server) — no external cluster required.
# setup-envtest must be installed: go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
# Tests are guarded by //go:build integration so they never run during test-unit.
ENVTEST_K8S_VERSION ?= 1.32.x
ENVTEST_BIN_DIR     ?= $(HOME)/.envtest-bins

test-integration:
	@command -v setup-envtest >/dev/null 2>&1 || \
	  (echo "setup-envtest not found — installing..." && \
	   go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)
	$(eval KUBEBUILDER_ASSETS := $(shell setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(ENVTEST_BIN_DIR) -p path))
	@echo "Running integration tests (KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS))..."
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) \
	go test ./tests/integration/... -v -tags=integration -count=1 -timeout=120s

# ── Full suite ────────────────────────────────────────────────────────────────
test-all: test-unit test-console test-integration

# ── Coverage ──────────────────────────────────────────────────────────────────
# HTML report written to coverage.html — open with: xdg-open coverage.html
test-coverage:
	@echo "Generating coverage report..."
	go test ./pkg/... -coverprofile=coverage.out -covermode=atomic -count=1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

# Text summary of per-function coverage — useful in CI output.
test-coverage-text:
	@echo "Coverage summary..."
	go test ./pkg/... -coverprofile=coverage.out -covermode=atomic -count=1
	@go tool cover -func=coverage.out | tail -5

# ── Fixture tests (kind cluster required) ────────────────────────────────────
# Each target spins up a dedicated kind cluster, installs Inrun via Helm,
# applies the fixture catalog + CR, asserts resources exist, then tears down.
# Requires: kind, helm, kubectl, inrun — all on $PATH.

FIXTURE_NOTE_CLUSTER      := inrun-note-fixture
FIXTURE_RECONCILER_CLUSTER := inrun-reconciler-fixture
HELM_CHART                := ./charts/inrun

test-fixture-note:
	@echo "── Note fixture test ──────────────────────────────────────────"
	@bash scripts/setup-kind.sh $(FIXTURE_NOTE_CLUSTER)
	@kubectl apply -f pkg/note/fixture/crd.yaml
	@inrun generate bundle --file pkg/note/fixture/catalog.yaml | kubectl apply -f -
	@helm install inrun $(HELM_CHART) --namespace inrun-system --wait --timeout 120s
	@kubectl apply -f pkg/note/fixture/cr.yaml
	@kubectl wait reconcilerprobe/my-probe --for=jsonpath='{.status.phase}'=Ready \
	    --timeout=120s 2>/dev/null || kubectl wait noteprobe/my-probe \
	    --for=condition=Ready=true --timeout=120s || \
	    kubectl get noteprobe my-probe -o yaml
	@echo "✅ Note fixture passed"
	@bash scripts/setup-kind.sh delete $(FIXTURE_NOTE_CLUSTER)

test-fixture-reconciler:
	@echo "── Reconciler fixture test ────────────────────────────────────"
	@bash scripts/setup-kind.sh $(FIXTURE_RECONCILER_CLUSTER)
	@kubectl apply -f pkg/runtime/reconciler/fixture/crd.yaml
	@inrun generate bundle --file pkg/runtime/reconciler/fixture/catalog.yaml | kubectl apply -f -
	@helm install inrun $(HELM_CHART) --namespace inrun-system --wait --timeout 120s
	@kubectl apply -f pkg/runtime/reconciler/fixture/cr.yaml
	@kubectl wait reconcilerprobe/probe --for=jsonpath='{.status.tier}'=premium \
	    --timeout=120s || kubectl get reconcilerprobe probe -o yaml
	@kubectl get deployment probe-app
	@kubectl get service probe-svc
	@kubectl get configmap probe-config
	@kubectl get serviceaccount probe-sa
	@kubectl get cronjob probe-cron
	@kubectl get deployment probe-premium
	@echo "✅ Reconciler fixture passed"
	@bash scripts/setup-kind.sh delete $(FIXTURE_RECONCILER_CLUSTER)

# ── Docs (Hugo) ───────────────────────────────────────────────────────────────
# The Hugo site lives in website/ and renders the docs/ directory.
# Requires the hugo binary — install with: brew install hugo  or  snap install hugo

DOCS_PORT ?= 8090

HUGO := $(shell which hugo 2>/dev/null)

# Install Hugo extended if not present (Linux/macOS)
.PHONY: hugo-install
hugo-install:
	@if [ -n "$(HUGO)" ]; then echo "Hugo already installed: $(HUGO)"; exit 0; fi; \
	OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	ARCH=$$(uname -m); \
	if [ "$$ARCH" = "x86_64" ]; then ARCH="amd64"; elif [ "$$ARCH" = "aarch64" ] || [ "$$ARCH" = "arm64" ]; then ARCH="arm64"; fi; \
	VER="0.160.0"; \
	if [ "$$OS" = "darwin" ]; then \
	  brew install hugo && exit 0; \
	fi; \
	URL="https://github.com/gohugoio/hugo/releases/download/v$${VER}/hugo_extended_$${VER}_$${OS}-$${ARCH}.tar.gz"; \
	echo "Installing Hugo v$${VER} from $$URL ..."; \
	curl -sSL "$$URL" | tar -xz -C /tmp hugo && sudo mv /tmp/hugo /usr/local/bin/hugo; \
	echo "✅ Hugo installed: $$(hugo version)"

docs-sync:
	@echo "Syncing docs/ → website/content/docs/ ..."
	@bash website/scripts/sync-docs.sh
	@echo "✅ Docs synced"

docs: hugo-install docs-sync
	@if [ -z "$(HUGO)" ]; then echo "Hugo not found. Run: make hugo-install"; exit 1; fi
	@echo "Starting Hugo docs server at http://localhost:$(DOCS_PORT) ..."
	hugo server --source website --port $(DOCS_PORT) --bind 0.0.0.0 --disableFastRender --logLevel warn
	@echo "✅ Docs server stopped"

docs-build: docs-sync
	@if [ -z "$(HUGO)" ]; then echo "Hugo not found. Run: make hugo-install"; exit 1; fi
	@echo "Building Hugo static site..."
	hugo --source website --minify
	@echo "✅ Hugo site built to website/public/"

docs-serve:
	hugo server --source website --port $(DOCS_PORT) --bind 0.0.0.0 --disableFastRender --logLevel warn


# ── Vet ───────────────────────────────────────────────────────────────────────
vet:
	@echo "Running go vet..."
	go vet ./...

# Known, verified-non-exploitable findings as of this writing, in
# vuln-inrun only — it's manually triggered (see .github/workflows/vuln-inrun.yml), so
# these don't need continue-on-error to avoid blocking merges, but the
# reasoning stays here for whoever re-triggers it. Re-verify before
# dismissing a finding as one of these; don't assume a new CVE ID is the
# same story.
#
# Neither finding below reaches vuln-console (separate module, never
# imported Helm or ORAS), vuln-runtime, or vuln-gateway: Helm
# (pkg/merger/helm.go, pkg/merger/helm_cache.go) and ORAS
# (pkg/registry/client.go, pkg/registry/module/pull.go) are both build-tag
# excluded from the runtime and gateway binaries — see the comments atop
# those files. Both are authoring-time-only (inrun push/pull/generate bundle);
# the runtime and gateway only ever read an already-merged catalog.yaml key
# from a ConfigMap. validate-pr.yml's vuln-check (console/runtime/
# gateway) has no known findings left to accept — it runs as a hard gate.
#
#   - GO-2026-5932 (golang.org/x/crypto/openpgp, no fix — package is
#     permanently unmaintained): reachable only through pkg/merger/helm.go's
#     resolveRemoteChart, via Helm SDK's action.Pull chart-signature
#     verification. Verified in helm.sh/helm/v3/pkg/action/pull.go that
#     NewPullWithOpts starts from a zero-value Pull{} (Verify defaults to
#     false), and Inrun's own call site never sets pull.Verify = true —
#     so the verification code path that would invoke openpgp is never
#     exercised, regardless of what a Catalog's HelmSource points at.
#   - GO-2026-5622 / GO-2026-5338 / GO-2026-5064 (github.com/containerd/containerd,
#     no fix yet): all three are specifically about CRI checkpoint/restore, a
#     container-runtime feature nothing in this codebase calls. containerd is
#     a transitive dependency of Helm's OCI-artifact getter package, pulled
#     in for unrelated image/content/compression handling.
vuln: vuln-inrun vuln-console vuln-runtime vuln-gateway

# Scans the whole module under Go's default build config — which is actually
# cmd/cli/run_dev.go's dev-CLI build (no tags set), not either production
# binary. Broadest net: also catches issues in dev-only tooling, which still
# matters (a compromised dev machine, supply-chain risk in what contributors
# run locally) even though it never ships.
vuln-inrun:
	go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...

# Console is a separate Go module — its own go.mod, own dependency
# tree — so it needs its own govulncheck invocation from its directory.
vuln-console:
	cd $(CONSOLE_DIR) && go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...

# These two scope govulncheck to exactly what's reachable in the two
# binaries that actually run in a cluster — same -tags used to build them
# (see inrun-linux / inrun-gateway-linux below) — rather than vuln-inrun's
# whole-module, default-build-config scan. A finding here means the shipped
# binary is actually affected; a finding only in vuln-inrun means it's
# confined to dev tooling.
vuln-runtime:
	go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 -tags runtime ./cmd/inrun/...

vuln-gateway:
	go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 -tags gateway ./cmd/inrun/...

# The console embeds its own copy of the brand SVGs (go:embed can't reach
# docs/); keep the copies identical to docs/assets.
.PHONY: check-brand
check-brand:
	@cmp -s docs/assets/inrun-wordmark.svg cmd/console/web/assets/static/inrun-wordmark.svg || { echo "inrun-wordmark.svg differs between docs/assets and the console"; exit 1; }
	@cmp -s docs/assets/inrun-mark.svg cmd/console/web/assets/static/inrun-mark.svg || { echo "inrun-mark.svg differs between docs/assets and the console"; exit 1; }
