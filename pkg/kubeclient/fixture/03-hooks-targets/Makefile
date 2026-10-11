# ── Typed Inrun Operator — Hooks with Targets ──────────────────────────────
BINARY_NAME ?= inrun
DEV_OUTPUT_DIR  ?= $(HOME)/.inrun/bin
PROD_OUTPUT_DIR ?= $(HOME)/.inrun/bin/runtime
CATALOG     ?= catalog.yaml

IMAGE_REPO  ?= myorg/my-operator
IMAGE_TAG   ?= latest
IMAGE       ?= $(IMAGE_REPO):$(IMAGE_TAG)
BUILD_TAGS	?=

GOOS        ?= linux
GOARCH      ?= amd64
CGO_ENABLED  = 0

INRUN_LDFLAGS := -X github.com/intentd/inrun/pkg/version.Version=$(GIT_VERSION) \
               -X github.com/intentd/inrun/pkg/version.Commit=$(GIT_COMMIT) \
               -X github.com/intentd/inrun/pkg/version.Date=$(GIT_DATE)

.PHONY: registry
registry:
	@[ -f go.mod.txt ] && mv go.mod.txt go.mod || true
	@[ -f go.sum.txt ] && mv go.sum.txt go.sum || true
	@find . -name "*.go" | xargs grep -l "^//go:build ignore$$" 2>/dev/null | while read f; do tail -n +3 "$$f" > "$$f.tmp" && mv "$$f.tmp" "$$f"; done || true
	inrun generate registry --file $(CATALOG)

.PHONY: build
build:
	@[ -f go.mod.txt ] && mv go.mod.txt go.mod || true
	@[ -f go.sum.txt ] && mv go.sum.txt go.sum || true
	@find . -name "*.go" | xargs grep -l "^//go:build ignore$$" 2>/dev/null | while read f; do tail -n +3 "$$f" > "$$f.tmp" && mv "$$f.tmp" "$$f"; done || true
	@mkdir -p $(DEV_OUTPUT_DIR)
	go mod tidy
	gofmt -w .
	go build \
		-ldflags "$(INRUN_LDFLAGS)" \
		-o $(DEV_OUTPUT_DIR)/$(BINARY_NAME) ./cmd/inrun
	@echo "✅ Development build: $(DEV_OUTPUT_DIR)/$(BINARY_NAME)"

.PHONY: build-runtime
build-runtime:
	@[ -f go.mod.txt ] && mv go.mod.txt go.mod || true
	@[ -f go.sum.txt ] && mv go.sum.txt go.sum || true
	@find . -name "*.go" | xargs grep -l "^//go:build ignore$$" 2>/dev/null | while read f; do tail -n +3 "$$f" > "$$f.tmp" && mv "$$f.tmp" "$$f"; done || true
	@mkdir -p $(PROD_OUTPUT_DIR)
	go mod tidy
	gofmt -w .
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-tags "runtime" \
		-ldflags "$(INRUN_LDFLAGS)" \
		-o $(PROD_OUTPUT_DIR)/$(BINARY_NAME) ./cmd/inrun
	@echo "✅ Production runtime build: $(PROD_OUTPUT_DIR)/$(BINARY_NAME)"

.PHONY: validate
validate:
	$(DEV_OUTPUT_DIR)/$(BINARY_NAME) validate -f $(CATALOG)

.PHONY: simulate
simulate:
	$(DEV_OUTPUT_DIR)/$(BINARY_NAME) simulate

.PHONY: e2e
e2e:
	$(DEV_OUTPUT_DIR)/$(BINARY_NAME) e2e

.PHONY: docker
docker: build-runtime
	@cp $(PROD_OUTPUT_DIR)/$(BINARY_NAME) ./$(BINARY_NAME)
	docker build -t $(IMAGE) .
	@rm -f ./$(BINARY_NAME)
	@echo "✅ Docker image built: $(IMAGE)"

.PHONY: push
push:
	docker push $(IMAGE)

.PHONY: release
release: docker push

.PHONY: clean
clean:
	@rm -f $(DEV_OUTPUT_DIR)/$(BINARY_NAME)
	@rm -rf $(PROD_OUTPUT_DIR)
	@echo "✅ Removed all local builds"

.PHONY: help
help:
	@echo "  registry       generate type registry from Catalog"
	@echo "  build          compile full development CLI (all commands)"
	@echo "  build-runtime  compile production binary (runtime only)"
	@echo "  validate       run catalog validation (using development binary)"
	@echo "  e2e            run end‑to‑end tests (using development binary)"
	@echo "  simulate       run simulation tests (using development binary)"
	@echo "  docker         build-runtime + copy into Docker image"
	@echo "  push           push Docker image to registry"
	@echo "  release        docker + push"
	@echo "  clean          remove all local builds"
