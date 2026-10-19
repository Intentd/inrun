package secrets

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/testutil"
)

func spec() Spec {
	return Spec{Name: "web", StringData: map[string]string{"token": "abc"}}
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
	if o.StringData["token"] != "abc" && string(o.Data["token"]) != "abc" {
		t.Errorf("secret data = %v / %v, want token", o.StringData, o.Data)
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
			return cs.CoreV1().Secrets("team").Get(ctx, "web", metav1.GetOptions{})
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
		if _, err := cs.CoreV1().Secrets(ns).Get(ctx, "web", metav1.GetOptions{}); err != nil {
			t.Errorf("no copy in %s: %v", ns, err)
		}
	}
	copyAll() // existing copies are left as they are
}

func TestCopyToNamespaces_FromSource(t *testing.T) {
	ctx := context.Background()
	cs := fake.NewClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "src", Namespace: "shared"},
		Data:       map[string][]byte{"token": []byte("from-source")},
	})
	s := Spec{Name: "web", FromSecret: "src", FromNamespace: "shared"}
	if err := CopyToNamespaces(ctx, cs, testutil.Owner(), s, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	sec, err := cs.CoreV1().Secrets("a").Get(ctx, "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(sec.Data["token"]) != "from-source" {
		t.Errorf("data = %v, want the source's", sec.Data)
	}
}
