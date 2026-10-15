package e2e

import (
	"context"
	"testing"

	"github.com/intentd/inrun/pkg/types"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// fakeAuthClientset returns a fake clientset whose SelfSubjectAccessReview
// and SubjectAccessReview creations are always answered with allowed,
// regardless of the requested verb/resource/user — the fake ObjectTracker
// has no built-in RBAC engine, so the real decision has to be injected via
// a reactor.
func fakeAuthClientset(allowed bool) *fake.Clientset {
	cs := fake.NewSimpleClientset()
	cs.Fake.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		ssar := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		ssar.Status.Allowed = allowed
		return true, ssar, nil
	})
	cs.Fake.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		sar := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		sar.Status.Allowed = allowed
		return true, sar, nil
	})
	return cs
}

func TestCheckKubectlAuth_Allowed(t *testing.T) {
	cs := fakeAuthClientset(true)
	e := types.E2EKubectlAuth{Verb: "get", Resource: "pods", E2EAssertions: types.E2EAssertions{Equals: "yes"}}
	if err := checkKubectlAuth(context.Background(), cs, e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckKubectlAuth_Denied(t *testing.T) {
	cs := fakeAuthClientset(false)
	e := types.E2EKubectlAuth{Verb: "delete", Resource: "secrets", E2EAssertions: types.E2EAssertions{Equals: "no"}}
	if err := checkKubectlAuth(context.Background(), cs, e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckKubectlAuth_UnexpectedResult(t *testing.T) {
	cs := fakeAuthClientset(false)
	e := types.E2EKubectlAuth{Verb: "delete", Resource: "secrets", E2EAssertions: types.E2EAssertions{Equals: "yes"}}
	if err := checkKubectlAuth(context.Background(), cs, e); err == nil {
		t.Fatal("expected error — access was denied but Equals: \"yes\" asserted it would be allowed")
	}
}

func TestCheckKubectlAuth_Impersonated(t *testing.T) {
	cs := fakeAuthClientset(true)
	e := types.E2EKubectlAuth{
		Verb:          "list",
		Resource:      "websites",
		As:            "system:serviceaccount:default:site-a",
		E2EAssertions: types.E2EAssertions{Equals: "yes"},
	}
	if err := checkKubectlAuth(context.Background(), cs, e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
