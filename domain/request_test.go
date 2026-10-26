package domain

import (
	"testing"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
)

func TestRequest_Finalizing(t *testing.T) {
	if (Request{}).Finalizing() {
		t.Error("no prepared state: want false")
	}
	if !(Request{Prepared: &PreparedRequest{Finalizing: true}}).Finalizing() {
		t.Error("prepared as finalizing: want true")
	}
}

func TestRequest_Object(t *testing.T) {
	if (Request{}).Object() != nil {
		t.Error("no Prepared: no object")
	}
	cr := object.New("demo.inrun.dev/v1", "App", "web")
	if got := (Request{Prepared: &PreparedRequest{Object: cr}}).Object(); got != cr {
		t.Errorf("Object() = %v, want the prepared CR", got)
	}
}
