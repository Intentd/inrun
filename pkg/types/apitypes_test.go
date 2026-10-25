package types

import (
	"testing"

	"github.com/intentd/inrun/pkg/util/kube"
)

func TestAPITypesFromCRDHeader(t *testing.T) {
	h := &kube.CRDFileHeader{}
	h.Spec.Group = "next.io"
	h.Spec.Names.Kind = "NewConfig"
	h.Spec.Names.Plural = "newconfigs"
	h.Spec.Versions = []kube.CRDFileVersion{{Name: "v1", Served: true}, {Name: "v2", Storage: true}}

	got := APITypesFromCRDHeader(h)
	want := APITypes{Group: "next.io", Version: "v2", Kind: "NewConfig", Plural: "newconfigs"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
