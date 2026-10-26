package domain

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
)

func TestBindToObjectHooks_PassesRequestAndResult(t *testing.T) {
	var gotArg string
	hooks := ReconcileHooks[*unstructured.Unstructured]{
		OnReconcile: func(_ context.Context, req Request, obj *unstructured.Unstructured) (Result, error) {
			prep := req.Prepared
			gotArg = prep.Args.String("region")
			return Result{RequeueAfter: time.Minute, Status: map[string]interface{}{"name": obj.GetName()}}, nil
		},
		OnDelete: func(_ context.Context, _ Request, _ *unstructured.Unstructured) (Result, error) {
			return Result{Requeue: &Requeue{After: time.Second, Reason: "teardown"}}, nil
		},
	}.BindToObjectHooks()

	obj := object.New("", "", "a")
	req := Request{Key: "default/a", Prepared: &PreparedRequest{
		Args: Args{"region": "eu"},
	}}
	res, err := hooks.OnReconcile(context.Background(), req, obj)
	if err != nil || gotArg != "eu" || res.RequeueAfter != time.Minute || res.Status["name"] != "a" {
		t.Fatalf("OnReconcile: res=%+v err=%v arg=%q", res, err, gotArg)
	}
	res, err = hooks.OnDelete(context.Background(), req, obj)
	if err != nil || res.Requeue == nil || res.Requeue.Reason != "teardown" {
		t.Fatalf("OnDelete: res=%+v err=%v", res, err)
	}
}

func TestBindToObjectHooks_NilHooksStayNil(t *testing.T) {
	oh := ReconcileHooks[*unstructured.Unstructured]{}.BindToObjectHooks()
	if oh.OnReconcile != nil || oh.OnDelete != nil {
		t.Fatal("unset hooks must stay nil so the generic reconciler skips them")
	}
}
