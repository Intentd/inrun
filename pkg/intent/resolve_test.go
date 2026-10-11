package intent

import (
	"reflect"
	"testing"

	"github.com/inrundev/inrun/pkg/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSetTarget(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAnnotations(map[string]string{"keep": "me"})

	SetTarget(obj, "")
	if Target(obj.GetAnnotations()) != "" {
		t.Error("empty target: want no target set")
	}

	SetTarget(obj, "v2")
	if got := Target(obj.GetAnnotations()); got != "v2" {
		t.Errorf("Target = %q, want v2", got)
	}
	if obj.GetAnnotations()["keep"] != "me" {
		t.Error("other annotations were dropped")
	}
	if obj.GetAnnotations()[labels.AnnotationServeTarget] != "v2" {
		t.Error("serve-target annotation not set")
	}
}

func TestSetTarget_NoAnnotations(t *testing.T) {
	obj := &unstructured.Unstructured{}
	SetTarget(obj, "v2")
	if got := Target(obj.GetAnnotations()); got != "v2" {
		t.Errorf("Target = %q, want v2", got)
	}
}

func TestDecode_JSONAndYAMLMatch(t *testing.T) {
	fromJSON, err := Decode([]byte(`{"target":"website","name":"a","replicas":2,"port":80,"tags":["x"],"debug":true}`))
	if err != nil {
		t.Fatal(err)
	}
	fromYAML, err := Decode([]byte("target: website\nname: a\nreplicas: 2\nport: 80\ntags: [x]\ndebug: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fromJSON, fromYAML) {
		t.Errorf("JSON %#v\nYAML %#v", fromJSON, fromYAML)
	}
	if _, ok := fromYAML["replicas"].(float64); !ok {
		t.Errorf("replicas is %T, want float64 like json.Unmarshal", fromYAML["replicas"])
	}
}

func TestDecode_Errors(t *testing.T) {
	for name, in := range map[string]string{"empty": "", "not a map": "- a\n- b\n", "broken": "{target: "} {
		if _, err := Decode([]byte(in)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
