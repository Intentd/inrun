package roles

import (
	"context"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/testutil"
)

func spec() Spec {
	return Spec{Name: "web", Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get"}}}}
}

func TestBuild(t *testing.T) {
	o := Build(testutil.Owner(), spec())

	if o.Name != "web" {
		t.Errorf("name = %q, want web", o.Name)
	}
	if o.Namespace != "team" {
		t.Errorf("namespace = %q, want the owner's", o.Namespace)
	}
	if !testutil.Owned(o) {
		t.Errorf("owner references = %+v, want one to the owner", o.OwnerReferences)
	}
	if len(o.Rules) != 1 {
		t.Errorf("rules = %+v, want the one rule", o.Rules)
	}
}

func TestLifecycle(t *testing.T) {
	owner, s := testutil.Owner(), spec()
	testutil.Lifecycle{
		Create: func(ctx context.Context, cs kubernetes.Interface) (types.Result, error) {
			return Create(ctx, cs, owner, s)
		},
		Apply: func(ctx context.Context, cs kubernetes.Interface) (types.Result, error) {
			return Apply(ctx, cs, owner, s, types.Options{})
		},
		Delete: func(ctx context.Context, cs kubernetes.Interface) (types.Result, error) {
			return Delete(ctx, cs, owner, s)
		},
		DeleteIfOwned: func(ctx context.Context, cs kubernetes.Interface, owned func(metav1.Object) bool) (types.Result, error) {
			return DeleteIfOwned(ctx, cs, "web", "team", owned)
		},
		Get: func(ctx context.Context, cs kubernetes.Interface) (metav1.Object, error) {
			return cs.RbacV1().Roles("team").Get(ctx, "web", metav1.GetOptions{})
		},
	}.Check(t)
}
