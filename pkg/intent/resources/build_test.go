package resources

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSupportedIntentTypesAreSorted(t *testing.T) {
	got := Types()

	if !sortStringsAreSorted(got) {
		t.Fatalf("Types() is not sorted: %v", got)
	}

	if len(got) != len(resourceBuilders) {
		t.Fatalf("supported type count = %d, want %d", len(got), len(resourceBuilders))
	}

	for _, typ := range got {
		if _, ok := resourceBuilders[typ]; !ok {
			t.Errorf("supported type %q has no registered builder", typ)
		}
	}
}

func TestBuildFromIntentRejectsNilOwner(t *testing.T) {
	_, err := Build("deployment", map[string]interface{}{}, nil)

	if err == nil {
		t.Fatal("expected error for nil owner")
	}

	if !strings.Contains(err.Error(), "requires the owner object") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildFromIntentRejectsUnknownType(t *testing.T) {
	owner := &unstructured.Unstructured{}
	owner.SetName("owner")

	_, err := Build("unknown", map[string]interface{}{}, owner)

	if err == nil {
		t.Fatal("expected error for unknown type")
	}

	if !strings.Contains(err.Error(), "unknown resource type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildFromIntentChecksOwnerBeforeType(t *testing.T) {
	// nil owner AND unknown type — error should be about the owner, not the type.
	_, err := Build("unknown", map[string]interface{}{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "requires the owner object") {
		t.Fatalf("expected owner error, got: %v", err)
	}
}

func TestSupportedIntentTypesErrorIsDeterministic(t *testing.T) {
	first := strings.Join(Types(), ", ")
	second := strings.Join(Types(), ", ")

	if first != second {
		t.Fatalf("supported types are not deterministic:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestIntentBuildersRegistryMatchesSupportedTypes(t *testing.T) {
	got := Types()

	want := make([]string, 0, len(resourceBuilders))
	for typ := range resourceBuilders {
		want = append(want, typ)
	}

	// Types already sorts its result.
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registered types mismatch:\ngot:  %v\nwant: %v", got, want)
	}
}

func sortStringsAreSorted(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] > values[i] {
			return false
		}
	}
	return true
}
