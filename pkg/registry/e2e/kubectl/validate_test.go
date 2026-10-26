package kubectl

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

// ── ValidateKubectl ───────────────────────────────────────────────────────────

func TestValidateKubectl_NoKubectlBlock(t *testing.T) {
	expects := []types.E2EExpectation{{Name: "plain"}}
	errs := Validate(expects)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// get ─────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_GetMissingKind(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Name:          "foo",
		Field:         ".status",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "kind")
}

func TestValidateKubectl_GetMissingName(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Field:         ".status",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "name")
}

func TestValidateKubectl_GetNoAssertion(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:  "Deployment",
		Name:  "app",
		Field: ".status",
	})
	requireErr(t, errs, "assertion")
}

func TestValidateKubectl_GetInvalidFormat(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		Format:        "xml",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "json or yaml")
}

func TestValidateKubectl_GetJQWithoutJSON(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		Format:        "yaml",
		JQ:            ".metadata",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "jq requires format: json")
}

func TestValidateKubectl_GetYQWithoutYAML(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		Format:        "json",
		YQ:            ".metadata",
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "yq requires format: yaml")
}

func TestValidateKubectl_GetValid(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		Field:         ".status.phase",
		E2EAssertions: types.E2EAssertions{Equals: "Running"},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateKubectlGetFull(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	tests := []struct {
		name    string
		get     types.E2EKubectlGet
		wantErr []string
	}{
		{
			name: "field with legacy assertion",
			get: types.E2EKubectlGet{
				Kind: "appconfigs.demo.inrun.dev",
				Name: "api-server",
				E2EAssertions: types.E2EAssertions{
					Equals: "Pending",
				},
				Field: ".status.phase",
			},
		},
		{
			name: "field with JSON assertion",
			get: types.E2EKubectlGet{
				Kind:  "appconfigs.demo.inrun.dev",
				Name:  "api-server",
				Field: ".status.phase",
				JSON:  []types.JSONAssertion{{Equals: "Pending"}},
			},
		},
		{
			name: "exists false without extractor",
			get: types.E2EKubectlGet{
				Kind:   "newconfigs.demo.inrun.dev",
				Name:   "api-server",
				Exists: boolPtr(false),
			},
		},
		{
			name: "exists true without extractor",
			get: types.E2EKubectlGet{
				Kind:   "newconfigs.demo.inrun.dev",
				Name:   "api-server",
				Exists: boolPtr(true),
			},
		},
		{
			name: "notExists true without extractor",
			get: types.E2EKubectlGet{
				Kind:      "newconfigs.demo.inrun.dev",
				Name:      "api-server",
				NotExists: boolPtr(true),
			},
		},
		{
			name: "JSONPath with legacy assertion",
			get: types.E2EKubectlGet{
				Kind:          "appconfigs.demo.inrun.dev",
				Name:          "api-server",
				JSONPath:      "{.status.phase}",
				E2EAssertions: types.E2EAssertions{Equals: "Pending"},
			},
		},
		{
			name: "JSONPath with only JSON assertion",
			get: types.E2EKubectlGet{
				Kind:     "appconfigs.demo.inrun.dev",
				Name:     "api-server",
				JSONPath: "{.status.phase}",
				JSON:     []types.JSONAssertion{{Equals: "Pending"}},
			},
		},
		{
			name: "jq requires JSON format",
			get: types.E2EKubectlGet{
				Kind:          "appconfigs.demo.inrun.dev",
				Name:          "api-server",
				Format:        "yaml",
				JQ:            ".status.phase",
				E2EAssertions: types.E2EAssertions{Equals: "Pending"},
			},
			wantErr: []string{"jq requires format: json"},
		},
		{
			name: "yq requires YAML format",
			get: types.E2EKubectlGet{
				Kind:          "appconfigs.demo.inrun.dev",
				Name:          "api-server",
				Format:        "json",
				YQ:            ".status.phase",
				E2EAssertions: types.E2EAssertions{Equals: "Pending"},
			},
			wantErr: []string{"yq requires format: yaml"},
		},
		{
			name: "invalid format",
			get: types.E2EKubectlGet{
				Kind:          "appconfigs.demo.inrun.dev",
				Name:          "api-server",
				Format:        "toml",
				E2EAssertions: types.E2EAssertions{Equals: "Pending"},
			},
			wantErr: []string{"format must be json or yaml"},
		},
		{
			name: "missing assertion",
			get: types.E2EKubectlGet{
				Kind:  "appconfigs.demo.inrun.dev",
				Name:  "api-server",
				Field: ".status.phase",
			},
			wantErr: []string{assertionListMsg},
		},
		{
			name: "missing extractor and object assertion is valid",
			get: types.E2EKubectlGet{
				Kind:          "appconfigs.demo.inrun.dev",
				Name:          "api-server",
				E2EAssertions: types.E2EAssertions{Equals: "Pending"},
			},
		},
		{
			name: "missing kind and name",
			get: types.E2EKubectlGet{
				Field:         ".status.phase",
				E2EAssertions: types.E2EAssertions{Equals: "Pending"},
			},
			wantErr: []string{"kind is required", "name is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateKubectlGet("test", tt.get)
			if len(errs) != len(tt.wantErr) {
				t.Fatalf("got %d errors (%v), want %d errors containing %v", len(errs), errs, len(tt.wantErr), tt.wantErr)
			}
			for i, want := range tt.wantErr {
				if !strings.Contains(errs[i].Error(), want) {
					t.Errorf("error %d = %q, want it to contain %q", i, errs[i], want)
				}
			}
		})
	}
}

// logs ────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_LogsMissingSelector(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		E2EAssertions: types.E2EAssertions{Contains: "started"},
	})
	requireErr(t, errs, "name, labelSelector, or leaderElection")
}

func TestValidateKubectl_LogsNoAssertion(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		Name: "my-pod",
	})
	requireErr(t, errs, "assertion")
}

func TestValidateKubectl_LogsLeaderElectionMissingLease(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		LeaderElection: &types.E2EKubectlLeaderElection{},
		E2EAssertions:  types.E2EAssertions{Contains: "started"},
	})
	requireErr(t, errs, "leaderElection.lease")
}

func TestValidateKubectl_LogsLeaderElectionMutuallyExclusiveWithName(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		Name: "my-pod",
		LeaderElection: &types.E2EKubectlLeaderElection{
			Lease: "my-lease",
		},
		E2EAssertions: types.E2EAssertions{Contains: "started"},
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_LogsLeaderElectionMutuallyExclusiveWithSelector(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		LabelSelector: "app=foo",
		LeaderElection: &types.E2EKubectlLeaderElection{
			Lease: "my-lease",
		},
		E2EAssertions: types.E2EAssertions{Contains: "started"},
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_LogsLeaderElectionValid(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		LeaderElection: &types.E2EKubectlLeaderElection{
			Lease: "my-lease",
		},
		E2EAssertions: types.E2EAssertions{Contains: "started"},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// delete ──────────────────────────────────────────────────────────────────────

func TestValidateKubectl_DeleteNeitherFileNorKind(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{})
	requireErr(t, errs, "file or (kind + name)")
}

func TestValidateKubectl_DeleteKindWithoutName(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{Kind: "Deployment"})
	requireErr(t, errs, "file or (kind + name)")
}

func TestValidateKubectl_DeleteMutuallyExclusive(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{
		File: "./foo.yaml",
		Kind: "Deployment",
		Name: "app",
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_DeleteByFile(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{File: "./foo.yaml"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateKubectl_DeleteByKindAndName(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{Kind: "Deployment", Name: "app"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// port-forward ────────────────────────────────────────────────────────────────

func TestValidateKubectl_PortForwardAssertionWithoutPath(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service:       "my-svc",
		Port:          8080,
		E2EAssertions: types.E2EAssertions{Equals: "ok"},
	})
	requireErr(t, errs, "path is required")
}

func TestValidateKubectl_PortForwardNoServiceOrPod(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Port: 8080,
	})
	requireErr(t, errs, "service, pod, or leaderElection")
}

func TestValidateKubectl_PortForwardZeroPort(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Port:    0,
	})
	requireErr(t, errs, "port must be > 0")
}

func TestValidateKubectl_PortForwardValidNoAssertion(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Port:    8080,
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateKubectl_PortForwardLeaderElectionMissingLease(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Port:           8080,
		LeaderElection: &types.E2EKubectlLeaderElection{},
	})
	requireErr(t, errs, "leaderElection.lease")
}

// apply ───────────────────────────────────────────────────────────────────────

func TestValidateKubectl_ApplyNeitherFileNorInline(t *testing.T) {
	errs := validateKubectlApply("loc", types.E2EKubectlApply{})
	requireErr(t, errs, "file or inline")
}

func TestValidateKubectl_ApplyMutuallyExclusive(t *testing.T) {
	errs := validateKubectlApply("loc", types.E2EKubectlApply{
		File:   "./foo.yaml",
		Inline: "apiVersion: v1",
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_ApplyByFile(t *testing.T) {
	errs := validateKubectlApply("loc", types.E2EKubectlApply{File: "./foo.yaml"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// patch ───────────────────────────────────────────────────────────────────────

func TestValidateKubectl_PatchInvalidType(t *testing.T) {
	errs := validateKubectlPatch("loc", types.E2EKubectlPatch{
		Kind:  "Deployment",
		Name:  "app",
		Patch: `{"spec":{}}`,
		Type:  "invalid",
	})
	requireErr(t, errs, "merge, strategic, or json")
}

func TestValidateKubectl_PatchMissingPatch(t *testing.T) {
	errs := validateKubectlPatch("loc", types.E2EKubectlPatch{
		Kind: "Deployment",
		Name: "app",
	})
	requireErr(t, errs, "patch is required")
}

func TestValidateKubectl_PatchValid(t *testing.T) {
	errs := validateKubectlPatch("loc", types.E2EKubectlPatch{
		Kind:  "Deployment",
		Name:  "app",
		Patch: `{"spec":{}}`,
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// top ─────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_TopInvalidKind(t *testing.T) {
	errs := validateKubectlTop("loc", types.E2EKubectlTop{
		Kind:          "deployment",
		E2EAssertions: types.E2EAssertions{Contains: "cpu"},
	})
	requireErr(t, errs, "pod or node")
}

func TestValidateKubectl_TopMissingKind(t *testing.T) {
	errs := validateKubectlTop("loc", types.E2EKubectlTop{E2EAssertions: types.E2EAssertions{Contains: "cpu"}})
	requireErr(t, errs, "kind is required")
}

func TestValidateKubectl_TopValidPod(t *testing.T) {
	errs := validateKubectlTop("loc", types.E2EKubectlTop{
		Kind:          "pod",
		E2EAssertions: types.E2EAssertions{Contains: "cpu"},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}
