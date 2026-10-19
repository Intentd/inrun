package statefulsets

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/testutil"
)

func spec() Spec {
	return Spec{Name: "web", Image: "postgres:16", Replicas: 1, ServiceName: "web"}
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
	if o.Spec.ServiceName != "web" || o.Spec.Selector.MatchLabels["app.kubernetes.io/name"] != "web" || o.Spec.Template.Labels["app.kubernetes.io/name"] != "web" {
		t.Errorf("serviceName %q selector %v pod labels %v, want web and matching labels", o.Spec.ServiceName, o.Spec.Selector.MatchLabels, o.Spec.Template.Labels)
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
			return cs.AppsV1().StatefulSets("team").Get(ctx, "web", metav1.GetOptions{})
		},
	}.Check(t)
}
