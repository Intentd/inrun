package deployments

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/testutil"
)

func spec() Spec {
	return Spec{Name: "web", Image: "nginx:1.27", Replicas: 2, Port: 8080, Labels: map[string]string{"tier": "web"}}
}

func TestBuild(t *testing.T) {
	d := Build(testutil.Owner(), spec())

	if d.Name != "web" || d.Namespace != "team" {
		t.Errorf("name/namespace = %s/%s, want team/web (the owner's namespace)", d.Namespace, d.Name)
	}
	if !testutil.Owned(d) {
		t.Errorf("owner references = %+v, want one to the owner", d.OwnerReferences)
	}
	if *d.Spec.Replicas != 2 {
		t.Errorf("replicas = %d, want 2", *d.Spec.Replicas)
	}
	sel := d.Spec.Selector.MatchLabels
	if len(sel) != 1 || sel["app.kubernetes.io/name"] != "web" {
		t.Errorf("selector = %v, want the default app.kubernetes.io/name: web", sel)
	}
	pod := d.Spec.Template.Labels
	if pod["app.kubernetes.io/name"] != "web" || pod["tier"] != "web" {
		t.Errorf("pod labels = %v, want the selector and the spec's labels", pod)
	}
	c := d.Spec.Template.Spec.Containers[0]
	if c.Image != "nginx:1.27" || c.Ports[0].ContainerPort != 8080 {
		t.Errorf("container = %s port %d, want nginx:1.27 port 8080", c.Image, c.Ports[0].ContainerPort)
	}
}

func TestBuild_Selector(t *testing.T) {
	s := spec()
	s.Namespace = "other"
	s.Selector = map[string]string{"owner": "app"}
	d := Build(testutil.Owner(), s)

	if d.Namespace != "other" {
		t.Errorf("namespace = %s, want the spec's", d.Namespace)
	}
	if got := d.Spec.Selector.MatchLabels; len(got) != 1 || got["owner"] != "app" {
		t.Errorf("selector = %v, want the spec's", got)
	}
	if d.Spec.Template.Labels["owner"] != "app" {
		t.Errorf("pod labels = %v, want the selector set on them", d.Spec.Template.Labels)
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
			return cs.AppsV1().Deployments("team").Get(ctx, "web", metav1.GetOptions{})
		},
	}.Check(t)
}
