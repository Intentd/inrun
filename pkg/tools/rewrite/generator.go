// pkg/migrate/generator.go
//
// Generates the Inrun scaffolding files that accompany a migrated reconciler:
// catalog.yaml, simulate.yaml, e2e.yaml, and go.mod.
//
// All generated files are stubs with TODO markers. The user fills in
// CRD details (group, kind, location) and resource assertions.
package rewrite

import (
	"fmt"
	"strings"
)

// Files holds all generated file contents keyed by filename.
type Files struct {
	Catalog    string
	Simulate   string
	E2E        string
	GoMod      string
	Makefile   string
	Dockerfile string
	README     string
}

// Options controls what the generator emits.
type Options struct {
	// ModulePath is the Go module path of the migrated operator (e.g. github.com/myorg/my-operator).
	// Used in go.mod and as a hint in catalog.yaml location fields.
	ModulePath string

	// OperatorName is the kebab-case name for the operator (e.g. webapp-operator).
	// Derived from ReceiverType if not set.
	OperatorName string

	// InrunVersion is the Inrun CLI version (from pkg/version.Short()).
	// Written into go.mod as the inrun require version.
	InrunVersion string
}

// Generate produces all scaffolding files from a Rewrite result.
func Generate(res *Result, opts Options) Files {
	if opts.OperatorName == "" {
		opts.OperatorName = toKebab(res.ReceiverType)
	}
	if opts.ModulePath == "" {
		opts.ModulePath = "github.com/myorg/" + opts.OperatorName
	}

	crdName := strings.TrimSuffix(opts.OperatorName, "-operator")
	crdName = strings.TrimSuffix(crdName, "-reconciler")
	constructorFn := "New" + res.ReceiverType

	return Files{
		Catalog:    generateCatalog(res, opts, crdName, constructorFn),
		Simulate:   generateSimulate(opts, crdName),
		E2E:        generateE2E(opts, crdName),
		GoMod:      generateGoMod(opts),
		Makefile:   generateMakefile(opts),
		Dockerfile: generateDockerfile(),
		README:     generateREADME(),
	}
}

func generateCatalog(res *Result, opts Options, crdName, constructorFn string) string {
	resources := buildResourcesBlock(res.Owns)
	watchBlock := buildWatchBlock(res.Watches)
	p := res.Primary

	kind := todoField(p.Kind, "set your CRD kind (e.g. WebApp)")
	version := p.Version
	if version == "" {
		version = "v1alpha1"
	}
	object := todoField(p.Object, "set the Go type name (e.g. WebApp)")
	objectList := p.ObjectList
	if objectList == "" {
		objectList = "TODO  # TODO(inrun rewrite): set the Go list type (e.g. WebAppList)"
	}
	location := p.Location
	if location == "" {
		location = opts.ModulePath + "/api/" + version + "  # TODO(inrun rewrite): adjust to your API types package"
	}
	alias := p.Alias
	if alias == "" {
		alias = "apiv1alpha1"
	}

	return fmt.Sprintf(`# Schema reference: https://inrun.dev/docs/reference/schema/catalog/
apiVersion: inrun.dev/v1
kind: Catalog
metadata:
  name: %s
  author: myorg  # TODO(inrun rewrite): set your name or org
  version: 0.1.0
  description: >
    Migrated from controller-runtime. The constructor lifted from %s
    runs inside Inrun's reconcile loop — informer, workqueue, worker pool,
    leader election, and metrics are provided by the runtime.
  tags:
    - migration
    - constructor
    - typed

spec:
  crds:
    %s:
      apiTypes:
        group: TODO  # TODO(inrun rewrite): set your CRD group (e.g. apps.myorg.io)
        version: %s
        kind: %s
        plural: TODO # TODO(inrun rewrite): set the plural (e.g. webapps)
        object: %s
        objectList: %s
        location: %s
        alias: %s

      operatorBox:
%s        runtime:
          allowedNamespaces:
            - default

        reconcile:
          # default: false — the generic.Reconciler is not used.
          # Your constructor owns the full reconcile loop.
          default: false

          managedResources:
%s
          constructor:
            location: %s/%s  # TODO(inrun rewrite): adjust to your reconciler package
            function: %s
`, opts.OperatorName, res.PkgName, crdName,
		version, kind, object, objectList, location, alias,
		watchBlock, resources, opts.ModulePath, res.PkgName, constructorFn)
}

// todoField returns the value if non-empty, or a TODO placeholder with the given hint.
func todoField(value, hint string) string {
	if value != "" {
		return value
	}
	return "TODO  # TODO(inrun rewrite): " + hint
}

// buildResourcesBlock renders the reconcile.managedResources list from Owns() detections.
func buildResourcesBlock(owns []DetectedType) string {
	if len(owns) == 0 {
		return "            - kind: TODO  # TODO(inrun rewrite): list every resource kind your operator manages\n"
	}
	var b strings.Builder
	for _, o := range owns {
		fmt.Fprintf(&b, "            - kind: %s\n", o.Kind)
		if o.APIVersion != "" && !strings.HasPrefix(o.APIVersion, "TODO") {
			fmt.Fprintf(&b, "              apiVersion: %s\n", o.APIVersion)
		} else if strings.HasPrefix(o.APIVersion, "TODO:") {
			fmt.Fprintf(&b, "              # TODO(inrun rewrite): apiVersion for %s (%s)\n",
				o.Kind, strings.TrimPrefix(o.APIVersion, "TODO: "))
		}
	}
	return b.String()
}

// buildWatchBlock renders the operatorBox.observe.watch block from Watches() detections.
// Returns an empty string when no watch entries were detected.
func buildWatchBlock(watches []DetectedType) string {
	if len(watches) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("        observe:\n          watch:\n")
	for _, w := range watches {
		apiVer := w.APIVersion
		suffix := ""
		if strings.HasPrefix(apiVer, "TODO:") {
			// Emit the raw import path as a comment so the user knows where to look.
			suffix = "  # TODO(inrun rewrite): verify apiVersion (" + strings.TrimPrefix(apiVer, "TODO: ") + ")"
			apiVer = "TODO"
		}
		fmt.Fprintf(&b, "            - apiVersion: %s%s\n", apiVer, suffix)
		fmt.Fprintf(&b, "              kind: %s\n", w.Kind)
		fmt.Fprintf(&b, "              on: [create, update, delete]\n")
	}
	return b.String()
}

func generateSimulate(opts Options, crdName string) string {
	return fmt.Sprintf(`# Schema reference: https://inrun.dev/docs/reference/schema/simulate/
apiVersion: inrun.dev/v1
kind: Simulate
metadata:
  name: %s-sim
  description: >
    Verify resources are created in cycle 1 — no cluster needed.
    TODO(inrun rewrite): fill in the resource kinds your operator creates.

spec:
  catalog: ../catalog.yaml
  cr: ../manifests/cr.yaml
  cycles: 3

  expect:
    steady: true
    noErrors: true
    ops:
      - cycle: 1
        verb: create
        resource: TODO  # TODO(inrun rewrite): e.g. deployments, services, configmaps
      # - cycle: 1
      #   verb: create
      #   resource: services
`, opts.OperatorName)
}

func generateE2E(opts Options, crdName string) string {
	return fmt.Sprintf(`# Schema reference: https://inrun.dev/docs/reference/schema/e2e/
apiVersion: inrun.dev/v1
kind: E2E
metadata:
  name: %s-e2e
  description: >
    End-to-end test for %s.
    TODO(inrun rewrite): fill in your CRD kind, CR name, and assertions.

spec:
  catalog: ../catalog.yaml
  crd: ../manifests/crd.yaml
  cr: ../manifests/cr.yaml

  cluster:
    provider: kind
    name: inrun-e2e
    reuse: false

  expect:
    - name: CR created
      after: cr-applied
      timeout: 30s
      resources:
        - kind: TODO  # TODO(inrun rewrite): set your CRD kind
          name: TODO  # TODO(inrun rewrite): set your CR name from manifests/cr.yaml
          namespace: default

    - name: Status written
      after: cr-applied
      timeout: 60s
      commands:
        - run: kubectl get TODO my-cr -o jsonpath='{.status.phase}'  # TODO(inrun rewrite): adjust
          contains: Running

    - name: Resources created
      after: cr-applied
      timeout: 90s
      resources:
        - kind: TODO  # TODO(inrun rewrite): e.g. Deployment
          name: TODO
          namespace: default
          ready: true

    - name: Cleanup verified
      after: cr-deleted
      timeout: 30s
      resources:
        - kind: TODO  # TODO(inrun rewrite): your managed resource
          name: TODO
          namespace: default
          count: 0
`, opts.OperatorName, opts.OperatorName)
}

func generateGoMod(opts Options) string {
	inrunVer := opts.InrunVersion
	if inrunVer == "" || inrunVer == "dev" {
		inrunVer = "v0.0.0  // TODO(inrun rewrite): replace with the published Inrun version"
	}
	return fmt.Sprintf(`module %s

go 1.22

require (
	github.com/intentd/inrun %s
	k8s.io/api v0.29.3
	k8s.io/apimachinery v0.29.3
	k8s.io/client-go v0.29.3
)

// Run: go mod tidy
// to resolve all indirect dependencies.
`, opts.ModulePath, inrunVer)
}

func generateMakefile(opts Options) string {
	return fmt.Sprintf(`# ── Typed Inrun Operator ───────────────────────────────────────────────────
BINARY_NAME ?= inrun
DEV_OUTPUT_DIR  ?= $(HOME)/.inrun/bin
PROD_OUTPUT_DIR ?= $(HOME)/.inrun/bin/runtime
CATALOG     ?= catalog.yaml

IMAGE_REPO  ?= myorg/%s
IMAGE_TAG   ?= latest
IMAGE       ?= $(IMAGE_REPO):$(IMAGE_TAG)

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

.PHONY: validate
validate:
	$(DEV_OUTPUT_DIR)/$(BINARY_NAME) validate -f $(CATALOG)

.PHONY: e2e
e2e:
	$(DEV_OUTPUT_DIR)/$(BINARY_NAME) e2e

.PHONY: docker
docker:
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

.PHONY: help
help:
	@echo "  registry       generate type registry from Catalog"
	@echo "  build          compile full development CLI"
	@echo "  build-runtime  compile production binary (runtime only)"
	@echo "  validate       run catalog validation"
	@echo "  e2e            run end-to-end tests"
	@echo "  docker         build-runtime + Docker image"
	@echo "  push           push Docker image"
	@echo "  release        docker + push"
	@echo "  clean          remove all local builds"
`, opts.OperatorName)
}

func generateDockerfile() string {
	return `FROM gcr.io/distroless/static-debian12:nonroot
COPY inrun /usr/local/bin/inrun
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/inrun"]
`
}

func generateREADME() string {
	bt := "```"
	return `# Migrated operator

Generated by ` + "`" + `inrun rewrite` + "`" + `. Your ` + "`" + `Reconcile` + "`" + ` is unchanged; Inrun now runs the
informer, queue, workers, leader election and metrics.

### 1. Resolve the TODOs

` + bt + `bash
grep -rn "TODO(inrun rewrite)" .
` + bt + `

Set ` + "`" + `apiTypes` + "`" + ` and ` + "`" + `managedResources` + "`" + ` in ` + "`" + `catalog.yaml` + "`" + `, add assertions to
` + "`" + `test/simulate.yaml` + "`" + ` and ` + "`" + `test/e2e.yaml` + "`" + `, and delete ` + "`" + `main.go` + "`" + `, the scheme
setup and ` + "`" + `SetupWithManager` + "`" + `.

### 2. Generate the registry

` + bt + `bash
make registry
` + bt + `

### 3. Build your inrun

` + bt + `bash
make build
` + bt + `

### 4. Simulate

` + bt + `bash
inrun simulate
` + bt + `

### 5. Run

` + bt + `bash
inrun
` + bt + `

### 6. Deploy

` + bt + `bash
make release IMAGE=<registry>/<image>:<tag>
` + bt + `

` + bt + `bash
inrun generate bundle -o bundle.yaml
` + bt + `

` + bt + `bash
kubectl apply -f bundle.yaml
` + bt + `

Then install the Inrun Helm chart with ` + "`" + `runtime.image` + "`" + ` set to your image.`
}

// toKebab converts a PascalCase type name to kebab-case.
// "WebAppReconciler" → "webapp-reconciler"
func toKebab(s string) string {
	if s == "" {
		return "my-operator"
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
