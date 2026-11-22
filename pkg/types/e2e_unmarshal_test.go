package types

import (
	"reflect"
	"strings"
	"testing"

	"github.com/intentd/tools/cluster"
	"gopkg.in/yaml.v3"
)

// Compile-time guards. A custom UnmarshalYAML only runs when it satisfies the
// Unmarshaler of the yaml library the decoder uses (gopkg.in/yaml.v3). A type
// whose method takes a *yaml.Node from another module (go.yaml.in/yaml/v3)
// compiles fine but is silently ignored, and its short forms stop parsing.
var (
	_ yaml.Unmarshaler = (*cluster.Image)(nil)
	_ yaml.Unmarshaler = (*E2EEnvironment)(nil)
	_ yaml.Unmarshaler = (*Prepare)(nil)
	_ yaml.Unmarshaler = (*ProvisionApplyEntry)(nil)
)

// Provision is embedded inline in E2EEnvironment and Prepare. If it had an
// UnmarshalYAML, Go would promote it to both, and yaml would decode the whole
// parent mapping as a bare Provision, rejecting environment.cluster,
// environment.images and prepare.ecosystem.
func TestProvision_HasNoUnmarshalYAML(t *testing.T) {
	unmarshaler := reflect.TypeOf((*yaml.Unmarshaler)(nil)).Elem()
	if reflect.TypeOf(&Provision{}).Implements(unmarshaler) {
		t.Fatal("Provision implements yaml.Unmarshaler; it would be promoted to every type that embeds it inline")
	}
}

func decodeE2ESpec(t *testing.T, spec string) E2E {
	t.Helper()
	doc := "apiVersion: inrun.dev/v1\nkind: E2E\nmetadata:\n  name: t\nspec:\n" + spec
	var e E2E
	if err := strictUnmarshal([]byte(doc), &e); err != nil {
		t.Fatalf("decode:\n%s\nerr: %v", doc, err)
	}
	return e
}

func TestE2EEnvironment_Unmarshal(t *testing.T) {
	e := decodeE2ESpec(t, `
  environment:
    cluster:
      provider: none
      name: c1
      reuse: true
    images:
      - nginx:1.27
      - image: my-app:ci-123
        build:
          context: ./app
          dockerfile: Dockerfile.dev
        deploy:
          port: 8080
    apply:
      - ./ns.yaml
      - path: ./secret.yaml
        wait:
          - kind: Secret
            name: s
    helm:
      - chart: ./chart
    wait:
      - kind: Deployment
        name: d
`)
	env := e.Spec.Environment
	if env == nil {
		t.Fatal("environment is nil")
	}
	if env.Cluster.Provider != ProviderNone || env.Cluster.Name != "c1" || !env.Cluster.Reuse {
		t.Errorf("cluster = %+v", env.Cluster)
	}

	if len(env.Images) != 2 {
		t.Fatalf("images = %+v, want 2", env.Images)
	}
	if got := env.Images[0]; got.Image != "nginx:1.27" || got.Build != nil || got.Deploy != nil {
		t.Errorf("images[0] (short form) = %+v", got)
	}
	img := env.Images[1]
	if img.Image != "my-app:ci-123" || img.Build == nil || img.Build.Context != "./app" ||
		img.Build.Dockerfile != "Dockerfile.dev" || img.Deploy == nil || img.Deploy.Port != 8080 {
		t.Errorf("images[1] (long form) = %+v build=%+v deploy=%+v", img, img.Build, img.Deploy)
	}

	if len(env.Apply) != 2 || env.Apply[0].Path != "./ns.yaml" ||
		env.Apply[1].Path != "./secret.yaml" || len(env.Apply[1].Wait) != 1 {
		t.Errorf("apply = %+v", env.Apply)
	}
	if len(env.Helm) != 1 || env.Helm[0].Chart != "./chart" {
		t.Errorf("helm = %+v", env.Helm)
	}
	if len(env.Wait) != 1 || env.Wait[0].Name != "d" {
		t.Errorf("wait = %+v", env.Wait)
	}
}

func TestE2EEnvironment_UnmarshalListShorthand(t *testing.T) {
	e := decodeE2ESpec(t, `
  environment:
    - ./ns.yaml
    - ./ingress.yaml
`)
	env := e.Spec.Environment
	if env == nil || len(env.Apply) != 2 || env.Apply[0].Path != "./ns.yaml" || env.Apply[1].Path != "./ingress.yaml" {
		t.Fatalf("environment = %+v", env)
	}
	if env.Provider() != ProviderKind {
		t.Errorf("provider = %q, want the kind default", env.Provider())
	}
}

func TestPrepare_Unmarshal(t *testing.T) {
	var p Prepare
	src := `
ecosystem: {}
apply:
  - ./workload.yaml
helm:
  - chart: ./chart
`
	if err := strictUnmarshal([]byte(src), &p); err != nil {
		t.Fatal(err)
	}
	if p.Ecosystem == nil {
		t.Error("ecosystem dropped")
	}
	if len(p.Apply) != 1 || p.Apply[0].Path != "./workload.yaml" || len(p.Helm) != 1 {
		t.Errorf("prepare = %+v", p)
	}
}

func TestPrepare_UnmarshalListShorthand(t *testing.T) {
	var p Prepare
	if err := strictUnmarshal([]byte("- ./a.yaml\n- ./b.yaml\n"), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Apply) != 2 || p.Apply[1].Path != "./b.yaml" {
		t.Errorf("apply = %+v", p.Apply)
	}
}

// Strictness must survive the custom unmarshalers: a typo is an error, not a
// silently dropped field.
func TestE2EEnvironmentAndPrepare_RejectUnknownFields(t *testing.T) {
	tests := []struct {
		name  string
		out   any
		src   string
		field string
	}{
		{"environment", &E2EEnvironment{}, "cluster: {provider: none}\nclustr: {}\n", "clustr"},
		{"environment.cluster", &E2EEnvironment{}, "cluster: {provder: none}\n", "provder"},
		{"environment.images[]", &E2EEnvironment{}, "images:\n  - image: x\n    biuld: {}\n", "biuld"},
		{"prepare", &Prepare{}, "ecosystem: {}\napplly: []\n", "applly"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := strictUnmarshal([]byte(tt.src), tt.out)
			if err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("err = %v, want it to name %q", err, tt.field)
			}
		})
	}
}
