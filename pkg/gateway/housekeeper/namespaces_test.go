package housekeeper

import (
	"testing"
)

func TestLabelsPresent(t *testing.T) {
	required := map[string]string{"a": "1", "b": "2"}
	if !labelsPresent(map[string]string{"a": "1", "b": "2", "c": "3"}, required) {
		t.Error("extra labels should not matter")
	}
	if labelsPresent(map[string]string{"a": "1"}, required) {
		t.Error("a missing label should be reported")
	}
	if labelsPresent(map[string]string{"a": "1", "b": "x"}, required) {
		t.Error("a changed value should be reported")
	}
}
