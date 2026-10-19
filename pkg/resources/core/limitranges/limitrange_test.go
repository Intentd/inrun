package limitranges

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/testutil"
)

func spec() Spec {
	return Spec{Name: "web", Limits: []types.LimitRangeItem{{Type: "Container", Default: map[string]string{"cpu": "500m"}}}}
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
	if len(o.Spec.Limits) != 1 || o.Spec.Limits[0].Default.Cpu().String() != "500m" {
		t.Errorf("limits = %+v, want the one item", o.Spec.Limits)
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
			return cs.CoreV1().LimitRanges("team").Get(ctx, "web", metav1.GetOptions{})
		},
	}.Check(t)
}

func TestCopyToNamespaces(t *testing.T) {
	ctx := context.Background()
	cs := fake.NewClientset()
	copyAll := func() {
		t.Helper()
		if err := CopyToNamespaces(ctx, cs, testutil.Owner(), spec(), []string{"a", "b", ""}); err != nil {
			t.Fatal(err)
		}
	}

	copyAll()
	for _, ns := range []string{"a", "b"} {
		if _, err := cs.CoreV1().LimitRanges(ns).Get(ctx, "web", metav1.GetOptions{}); err != nil {
			t.Errorf("no copy in %s: %v", ns, err)
		}
	}
	copyAll() // existing copies are left as they are
}
