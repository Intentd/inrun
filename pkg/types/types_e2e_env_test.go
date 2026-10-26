package types

import (
	"reflect"
	"testing"
)

func TestExpandEnv_ResolvesAnExpectation(t *testing.T) {
	t.Setenv("INRUN_TEST_PROCESS", "from-process")
	spec := &E2ESpec{Env: map[string]string{"NS": "team", "TOKEN": "t0k"}}
	exp := E2EExpectation{
		Name: "uses ${NS}",
		Env:  map[string]string{"POD": "web", "RAW": "${NS}"},
		Commands: []E2ECommand{{
			Run: `test "$NS" = "${NS}"`, // the shell's own expansion: left as written
		}},
		Kubectl: &E2EKubectl{
			Get:   []E2EKubectlGet{{Kind: "Pod", Name: "${POD}", Namespace: "${NS}"}},
			Patch: []E2EKubectlPatch{{Kind: "Deployment", Name: "web", Patch: `{"spec":{"$patch":"replace"}}`}},
			PortForward: []E2EKubectlPortForward{{
				Path:    "/${INRUN_TEST_PROCESS}",
				Headers: map[string]string{"Authorization": "Bearer ${TOKEN}"},
			}},
		},
	}

	env := spec.ResolveEnv(exp)
	got := ExpandEnv(env, exp)

	if got.Name != "uses team" {
		t.Errorf("name = %q", got.Name)
	}
	if got.Commands[0].Run != exp.Commands[0].Run {
		t.Errorf("run expanded to %q: the shell expands it", got.Commands[0].Run)
	}
	if got.Env["RAW"] != "${NS}" {
		t.Errorf("env values are the source, not expanded: got %q", got.Env["RAW"])
	}
	g := got.Kubectl.Get[0]
	if g.Name != "web" || g.Namespace != "team" {
		t.Errorf("get = %+v", g)
	}
	if got.Kubectl.Patch[0].Patch != `{"spec":{"$patch":"replace"}}` {
		t.Errorf("patch = %q: a bare $ is not a reference", got.Kubectl.Patch[0].Patch)
	}
	pf := got.Kubectl.PortForward[0]
	if pf.Path != "/from-process" || pf.Headers["Authorization"] != "Bearer t0k" {
		t.Errorf("port-forward = %+v", pf)
	}

	// The original is untouched: the copy is deep.
	if exp.Kubectl.Get[0].Name != "${POD}" || exp.Kubectl.PortForward[0].Headers["Authorization"] != "Bearer ${TOKEN}" {
		t.Error("ExpandEnv modified its input")
	}
}

func TestUnknownEnv(t *testing.T) {
	t.Setenv("INRUN_TEST_PROCESS", "set")
	exp := E2EExpectation{
		Name:     "${INRUN_TEST_PROCESS}",
		Commands: []E2ECommand{{Run: "echo ${IN_SHELL_ONLY}"}}, // not ours to resolve
		Kubectl: &E2EKubectl{Get: []E2EKubectlGet{
			{Kind: "Pod", Name: "${MISSING_B}", Namespace: "${MISSING_A}"},
			{Kind: "Pod", Name: "${MISSING_A}"},
		}},
	}
	env := (&E2ESpec{}).ResolveEnv(exp)
	if got, want := UnknownEnv(env, exp), []string{"MISSING_A", "MISSING_B"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("UnknownEnv = %v, want %v", got, want)
	}
}

func TestEnvResolver_Map(t *testing.T) {
	t.Setenv("INRUN_TEST_PROCESS", "from-process")
	t.Setenv("SHARED", "from-process")
	m := (&E2ESpec{Env: map[string]string{"SHARED": "declared"}}).ResolveEnv(E2EExpectation{}).Map()
	if m["INRUN_TEST_PROCESS"] != "from-process" || m["SHARED"] != "declared" {
		t.Fatalf("Map: process %q, shared %q", m["INRUN_TEST_PROCESS"], m["SHARED"])
	}
}
