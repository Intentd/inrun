package jobs

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/testutil"
)

func spec() Spec {
	return Spec{Name: "web", Image: "busybox:1.36"}
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
	if o.Spec.Template.Spec.Containers[0].Image != "busybox:1.36" {
		t.Errorf("image = %q, want busybox:1.36", o.Spec.Template.Spec.Containers[0].Image)
	}
	s := spec()
	s.Detached = true
	if refs := Build(testutil.Owner(), s).OwnerReferences; len(refs) != 0 {
		t.Errorf("detached job owner references = %+v, want none", refs)
	}
}

func TestLifecycle(t *testing.T) {
	owner, s := testutil.Owner(), spec()
	testutil.Lifecycle{
		Create: func(ctx context.Context, cs kubernetes.Interface) (types.Result, error) {
			return Create(ctx, cs, owner, s)
		},
		Delete: func(ctx context.Context, cs kubernetes.Interface) (types.Result, error) {
			return Delete(ctx, cs, owner, s)
		},
		Get: func(ctx context.Context, cs kubernetes.Interface) (metav1.Object, error) {
			return cs.BatchV1().Jobs("team").Get(ctx, "web", metav1.GetOptions{})
		},
	}.Check(t)
}
