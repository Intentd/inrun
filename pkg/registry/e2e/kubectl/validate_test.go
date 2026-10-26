package kubectl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

// ── Validate ─────────────────────────────────────────────────────────────────

func TestValidate_NoKubectlBlock(t *testing.T) {
	expects := []types.E2EExpectation{{Name: "plain"}}
	errs := Validate(expects, "")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidate_WrapsWithLocation(t *testing.T) {
	expects := []types.E2EExpectation{{
		Name:    "step",
		Kubectl: &types.E2EKubectl{Get: []types.E2EKubectlGet{{}}},
	}}
	errs := Validate(expects, "")
	if len(errs) == 0 {
		t.Fatal("expected errors")
	}
	if !strings.Contains(errs[0].Error(), `expect[0] "step" kubectl.get[0]`) {
		t.Errorf("location missing from %q", errs[0])
	}
}

// get ─────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_GetMissingKind(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Name:   "foo",
		Field:  ".status",
		Assert: &types.Assertions{TextAssertion: types.TextAssertion{Equals: "ok"}},
	})
	requireErr(t, errs, "kind")
}

func TestValidateKubectl_GetMissingNameAndSelector(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:   "Deployment",
		Field:  ".status",
		Assert: &types.Assertions{TextAssertion: types.TextAssertion{Equals: "ok"}},
	})
	requireErr(t, errs, "name or labelSelector")
}

func TestValidateKubectl_GetNameAndSelectorMutuallyExclusive(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		Name:          "app",
		LabelSelector: "app=foo",
		Assert:        &types.Assertions{TextAssertion: types.TextAssertion{Equals: "ok"}},
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_GetBySelector(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:          "Deployment",
		LabelSelector: "app=foo",
		Assert:        &types.Assertions{TextAssertion: types.TextAssertion{Equals: "ok"}},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
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
		Kind:   "Deployment",
		Name:   "app",
		Format: "xml",
		Assert: &types.Assertions{TextAssertion: types.TextAssertion{Equals: "ok"}},
	})
	requireErr(t, errs, "json or yaml")
}

func TestValidateKubectl_GetJQWithoutJSON(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:   "Deployment",
		Name:   "app",
		Format: "yaml",
		JQ:     ".metadata",
		Assert: &types.Assertions{TextAssertion: types.TextAssertion{Equals: "ok"}},
	})
	requireErr(t, errs, "jq requires format: json")
}

func TestValidateKubectl_GetYQWithoutYAML(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:   "Deployment",
		Name:   "app",
		Format: "json",
		YQ:     ".metadata",
		Assert: &types.Assertions{TextAssertion: types.TextAssertion{Equals: "ok"}},
	})
	requireErr(t, errs, "yq requires format: yaml")
}

func TestValidateKubectl_GetJSONPathWithoutAssertion(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:     "Deployment",
		Name:     "app",
		JSONPath: "{.status.phase}",
	})
	requireErr(t, errs, "jsonPath requires an assertion")
}

func TestValidateKubectl_GetJSONPathWithAssertion(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:     "Deployment",
		Name:     "app",
		JSONPath: "{.status.phase}",
		Assert:   &types.Assertions{TextAssertion: types.TextAssertion{Equals: "Running"}},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateKubectl_GetValid(t *testing.T) {
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:   "Deployment",
		Name:   "app",
		Field:  ".status.phase",
		Assert: &types.Assertions{TextAssertion: types.TextAssertion{Equals: "Running"}},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateKubectl_GetExistsFalseWithoutExtractor(t *testing.T) {
	f := false
	errs := validateKubectlGet("loc", types.E2EKubectlGet{
		Kind:   "Deployment",
		Name:   "app",
		Assert: &types.Assertions{Exists: &f},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// logs ────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_LogsMissingSelector(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		Assert: &types.Assertions{TextAssertion: types.TextAssertion{Contains: "started"}},
	})
	requireErr(t, errs, "name, labelSelector, or leaderElection")
}

func TestValidateKubectl_LogsNameAndSelectorMutuallyExclusive(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		Name:          "my-pod",
		LabelSelector: "app=foo",
		Assert:        &types.Assertions{TextAssertion: types.TextAssertion{Contains: "started"}},
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_LogsNoAssertion(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{Name: "my-pod"})
	requireErr(t, errs, "assertion")
}

func TestValidateKubectl_LogsLeaderElectionMissingLease(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		LeaderElection: &types.E2EKubectlLeaderElection{},
		Assert:         &types.Assertions{TextAssertion: types.TextAssertion{Contains: "started"}},
	})
	requireErr(t, errs, "leaderElection.lease")
}

func TestValidateKubectl_LogsLeaderElectionMutuallyExclusiveWithName(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		Name:           "my-pod",
		LeaderElection: &types.E2EKubectlLeaderElection{Lease: "my-lease"},
		Assert:         &types.Assertions{TextAssertion: types.TextAssertion{Contains: "started"}},
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_LogsLeaderElectionValid(t *testing.T) {
	errs := validateKubectlLogs("loc", types.E2EKubectlLogs{
		LeaderElection: &types.E2EKubectlLeaderElection{Lease: "my-lease"},
		Assert:         &types.Assertions{TextAssertion: types.TextAssertion{Contains: "started"}},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// describe ────────────────────────────────────────────────────────────────────

func TestValidateKubectl_DescribeNameAndSelectorMutuallyExclusive(t *testing.T) {
	errs := validateKubectlDescribe("loc", types.E2EKubectlDescribe{
		Kind:          "Pod",
		Name:          "x",
		LabelSelector: "app=foo",
		Assert:        &types.Assertions{TextAssertion: types.TextAssertion{Contains: "Running"}},
	})
	requireErr(t, errs, "mutually exclusive")
}

// exec ────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_ExecNameAndSelectorMutuallyExclusive(t *testing.T) {
	errs := validateKubectlExec("loc", types.E2EKubectlExec{
		Name:          "my-pod",
		LabelSelector: "app=foo",
		Command:       []string{"cat", "/x"},
		Assert:        &types.Assertions{TextAssertion: types.TextAssertion{Contains: "x"}},
	})
	requireErr(t, errs, "mutually exclusive")
}

// port-forward ────────────────────────────────────────────────────────────────

func TestValidateKubectl_PortForwardAssertionWithoutPath(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Port:    8080,
		Assert:  &types.Assertions{TextAssertion: types.TextAssertion{Equals: "ok"}},
	}, "")
	requireErr(t, errs, "path is required")
}

func TestValidateKubectl_PortForwardNoServiceOrPod(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{Port: 8080}, "")
	requireErr(t, errs, "service, pod, or leaderElection")
}

func TestValidateKubectl_PortForwardServiceAndPodMutuallyExclusive(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Pod:     "my-pod",
		Port:    8080,
	}, "")
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_PortForwardZeroPort(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Port:    0,
	}, "")
	requireErr(t, errs, "port must be > 0")
}

func TestValidateKubectl_PortForwardBodyWithoutPath(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Port:    8080,
		Method:  "POST",
		Body:    `{"x":1}`,
	}, "")
	requireErr(t, errs, "path is required when a body is set")
}

func TestValidateKubectl_PortForwardBodyAndBodyFileMutuallyExclusive(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service:  "my-svc",
		Port:     8080,
		Path:     "/x",
		Method:   "POST",
		Body:     `{"x":1}`,
		BodyFile: "body.json",
	}, "")
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_PortForwardGetWithBody(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Port:    8080,
		Path:    "/x",
		Method:  "GET",
		Body:    `{"x":1}`,
	}, "")
	requireErr(t, errs, "does not accept a body")
}

func TestValidateKubectl_PortForwardBodyFileMissing(t *testing.T) {
	dir := t.TempDir()
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service:  "my-svc",
		Port:     8080,
		Path:     "/x",
		Method:   "POST",
		BodyFile: "missing.json",
	}, dir)
	requireErr(t, errs, "bodyFile")
}

func TestValidateKubectl_PortForwardBodyFileExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "body.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service:  "my-svc",
		Port:     8080,
		Path:     "/x",
		Method:   "POST",
		BodyFile: "body.json",
	}, dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateKubectl_PortForwardValidNoAssertion(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Service: "my-svc",
		Port:    8080,
	}, "")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateKubectl_PortForwardLeaderElectionMissingLease(t *testing.T) {
	errs := validateKubectlPortForward("loc", types.E2EKubectlPortForward{
		Port:           8080,
		LeaderElection: &types.E2EKubectlLeaderElection{},
	}, "")
	requireErr(t, errs, "leaderElection.lease")
}

// delete ─────────────────────────────────────────────────────────────────────

func TestValidateKubectl_DeleteNeitherFileNorKind(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{}, "")
	requireErr(t, errs, "file or (kind + name)")
}

func TestValidateKubectl_DeleteMutuallyExclusive(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{
		File: "./foo.yaml",
		Kind: "Deployment",
		Name: "app",
	}, "")
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_DeleteTimeoutInvalid(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{
		File:    "./foo.yaml",
		Timeout: "not-a-duration",
	}, "")
	requireErr(t, errs, "valid duration")
}

func TestValidateKubectl_DeleteByFile(t *testing.T) {
	errs := validateKubectlDelete("loc", types.E2EKubectlDelete{File: "./foo.yaml"}, "")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// apply ──────────────────────────────────────────────────────────────────────

func TestValidateKubectl_ApplyNeitherFileNorInline(t *testing.T) {
	errs := validateKubectlApply("loc", types.E2EKubectlApply{}, "")
	requireErr(t, errs, "file or inline")
}

func TestValidateKubectl_ApplyMutuallyExclusive(t *testing.T) {
	errs := validateKubectlApply("loc", types.E2EKubectlApply{
		File:   "./foo.yaml",
		Inline: "apiVersion: v1",
	}, "")
	requireErr(t, errs, "mutually exclusive")
}

// A file an apply or delete names is checked from the step's directory; a
// URL or a templated path is left to the run.
func TestValidateKubectl_ApplyAndDeleteFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cr.yaml"), []byte("kind: X\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	requireErr(t, validateKubectlApply("loc", types.E2EKubectlApply{File: "./missing.yaml"}, dir), `file "./missing.yaml"`)
	requireErr(t, validateKubectlDelete("loc", types.E2EKubectlDelete{File: "../missing.yaml"}, dir), `file "../missing.yaml"`)

	for _, file := range []string{
		"./cr.yaml",
		filepath.Join(dir, "cr.yaml"),
		"https://example.com/cr.yaml",
		"./{{ .env.NAME }}.yaml",
		"./${NAME}.yaml",
	} {
		if errs := validateKubectlApply("loc", types.E2EKubectlApply{File: file}, dir); len(errs) != 0 {
			t.Errorf("apply %q: unexpected errors: %v", file, errs)
		}
		if errs := validateKubectlDelete("loc", types.E2EKubectlDelete{File: file}, dir); len(errs) != 0 {
			t.Errorf("delete %q: unexpected errors: %v", file, errs)
		}
	}
}

// patch ──────────────────────────────────────────────────────────────────────

func TestValidateKubectl_PatchInvalidType(t *testing.T) {
	errs := validateKubectlPatch("loc", types.E2EKubectlPatch{
		Kind:  "Deployment",
		Name:  "app",
		Patch: `{"spec":{}}`,
		Type:  "invalid",
	}, "")
	requireErr(t, errs, "merge, strategic, or json")
}

// A patch targets kind and name, or every object in a manifest by file: one
// or the other. With file, the manifest names each namespace.
func TestValidateKubectl_PatchFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cr.yaml"), []byte("kind: X\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := `{"spec":{}}`

	if errs := validateKubectlPatch("loc", types.E2EKubectlPatch{File: "./cr.yaml", Patch: patch}, dir); len(errs) != 0 {
		t.Errorf("file only: unexpected errors: %v", errs)
	}
	requireErr(t, validateKubectlPatch("loc", types.E2EKubectlPatch{File: "./cr.yaml", Kind: "X", Name: "a", Patch: patch}, dir), "mutually exclusive")
	requireErr(t, validateKubectlPatch("loc", types.E2EKubectlPatch{File: "./cr.yaml", Namespace: "team", Patch: patch}, dir), "namespace has no effect")
	requireErr(t, validateKubectlPatch("loc", types.E2EKubectlPatch{File: "./missing.yaml", Patch: patch}, dir), `file "./missing.yaml"`)
	requireErr(t, validateKubectlPatch("loc", types.E2EKubectlPatch{Patch: patch}, dir), "kind is required (or file)")
}

// The patch content comes inline or from patchFile, one or the other. One
// file as both target and patch must hold exactly one object: kubectl patches
// every object the file names with its first.
func TestValidateKubectl_PatchFileContent(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("one.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: a}\ndata: {b: \"2\"}\n")
	write("two.yaml", "kind: ConfigMap\nmetadata: {name: a}\n---\nkind: ConfigMap\nmetadata: {name: b}\n")
	write("patch.yaml", "data: {b: \"2\"}\n")

	ok := []types.E2EKubectlPatch{
		{File: "./one.yaml", PatchFile: "./one.yaml"},
		{File: "./two.yaml", PatchFile: "./patch.yaml"},
		{Kind: "ConfigMap", Name: "a", PatchFile: "./patch.yaml"},
	}
	for _, p := range ok {
		if errs := validateKubectlPatch("loc", p, dir); len(errs) != 0 {
			t.Errorf("%+v: unexpected errors: %v", p, errs)
		}
	}
	requireErr(t, validateKubectlPatch("loc", types.E2EKubectlPatch{File: "./two.yaml", PatchFile: "./two.yaml"}, dir), "must hold one object; it holds 2")
	requireErr(t, validateKubectlPatch("loc", types.E2EKubectlPatch{Kind: "ConfigMap", Name: "a", Patch: "{}", PatchFile: "./patch.yaml"}, dir), "mutually exclusive")
	requireErr(t, validateKubectlPatch("loc", types.E2EKubectlPatch{Kind: "ConfigMap", Name: "a"}, dir), "patch or patchFile is required")
	requireErr(t, validateKubectlPatch("loc", types.E2EKubectlPatch{Kind: "ConfigMap", Name: "a", PatchFile: "./missing.yaml"}, dir), `patchFile "./missing.yaml"`)
}

func TestValidateKubectl_PatchValid(t *testing.T) {
	errs := validateKubectlPatch("loc", types.E2EKubectlPatch{
		Kind:  "Deployment",
		Name:  "app",
		Patch: `{"spec":{}}`,
	}, "")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// cp ─────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_CpNameAndSelectorMutuallyExclusive(t *testing.T) {
	errs := validateKubectlCp("loc", types.E2EKubectlCp{
		Name:          "my-pod",
		LabelSelector: "app=foo",
		Src:           "/x",
		Assert:        &types.Assertions{TextAssertion: types.TextAssertion{Contains: "x"}},
	})
	requireErr(t, errs, "mutually exclusive")
}

// top ────────────────────────────────────────────────────────────────────────

func TestValidateKubectl_TopInvalidKind(t *testing.T) {
	errs := validateKubectlTop("loc", types.E2EKubectlTop{
		Kind:   "deployment",
		Assert: &types.Assertions{TextAssertion: types.TextAssertion{Contains: "cpu"}},
	})
	requireErr(t, errs, "pod or node")
}

func TestValidateKubectl_TopNameAndSelectorMutuallyExclusive(t *testing.T) {
	errs := validateKubectlTop("loc", types.E2EKubectlTop{
		Kind:          "pod",
		Name:          "x",
		LabelSelector: "app=foo",
		Assert:        &types.Assertions{TextAssertion: types.TextAssertion{Contains: "cpu"}},
	})
	requireErr(t, errs, "mutually exclusive")
}

func TestValidateKubectl_TopValidPod(t *testing.T) {
	errs := validateKubectlTop("loc", types.E2EKubectlTop{
		Kind:   "pod",
		Assert: &types.Assertions{TextAssertion: types.TextAssertion{Contains: "cpu"}},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// scale ──────────────────────────────────────────────────────────────────────

func TestValidateKubectl_ScaleNegativeReplicas(t *testing.T) {
	errs := validateKubectlScale("loc", types.E2EKubectlScale{
		Kind:     "Deployment",
		Name:     "app",
		Replicas: -1,
	})
	requireErr(t, errs, "replicas must be >= 0")
}

func TestValidateKubectl_ScaleValid(t *testing.T) {
	errs := validateKubectlScale("loc", types.E2EKubectlScale{
		Kind:     "Deployment",
		Name:     "app",
		Replicas: 3,
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// ── post blocks ────────────────────────────────────────
