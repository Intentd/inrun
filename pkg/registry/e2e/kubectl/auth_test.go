package kubectl

import (
	"context"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"testing"

	"github.com/intentd/inrun/pkg/types"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// fakeAuthClientset returns a fake clientset whose SelfSubjectAccessReview
// and SubjectAccessReview creations are always answered with allowed,
// regardless of the requested verb/resource/user — the fake ObjectTracker
// has no built-in RBAC engine, so the real decision has to be injected via
// a reactor.
func fakeAuthClientset(allowed bool) *kubefake.Clientset {
	cs := kubefake.NewClientset()
	cs.Fake.PrependReactor("create", "selfsubjectaccessreviews", func(action kubefake.Action) (bool, runtime.Object, error) {
		ssar := action.(kubefake.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		ssar.Status.Allowed = allowed
		return true, ssar, nil
	})
	cs.Fake.PrependReactor("create", "subjectaccessreviews", func(action kubefake.Action) (bool, runtime.Object, error) {
		sar := action.(kubefake.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		sar.Status.Allowed = allowed
		return true, sar, nil
	})
	return cs
}

func TestKubectlAuth_Allowed(t *testing.T) {
	cs := fakeAuthClientset(true)
	e := types.E2EKubectlAuth{Verb: "get", Resource: "pods", E2EAssertions: types.E2EAssertions{Equals: "yes"}}
	if err := kubectlAuth(context.Background(), cs, e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKubectlAuth_Denied(t *testing.T) {
	cs := fakeAuthClientset(false)
	e := types.E2EKubectlAuth{Verb: "delete", Resource: "secrets", E2EAssertions: types.E2EAssertions{Equals: "no"}}
	if err := kubectlAuth(context.Background(), cs, e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKubectlAuth_UnexpectedResult(t *testing.T) {
	cs := fakeAuthClientset(false)
	e := types.E2EKubectlAuth{Verb: "delete", Resource: "secrets", E2EAssertions: types.E2EAssertions{Equals: "yes"}}
	if err := kubectlAuth(context.Background(), cs, e); err == nil {
		t.Fatal("expected error — access was denied but Equals: \"yes\" asserted it would be allowed")
	}
}

func TestKubectlAuth_Impersonated(t *testing.T) {
	cs := fakeAuthClientset(true)
	e := types.E2EKubectlAuth{
		Verb:          "list",
		Resource:      "websites",
		As:            "system:serviceaccount:default:site-a",
		E2EAssertions: types.E2EAssertions{Equals: "yes"},
	}
	if err := kubectlAuth(context.Background(), cs, e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
