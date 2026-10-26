package prepare

import (
	"testing"
)

func TestSetNestedPatch_SingleLevel(t *testing.T) {
	patch := map[string]interface{}{}
	setNestedPatch(patch, "replicas", "3")
	if v, ok := patch["replicas"]; !ok || v != "3" {
		t.Errorf("expected patch[replicas]=3, got %v", patch)
	}
}

func TestSetNestedPatch_NestedPath(t *testing.T) {
	patch := map[string]interface{}{}
	setNestedPatch(patch, "spec.replicas", "3")
	spec, ok := patch["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("expected spec to be a map")
	}
	if spec["replicas"] != "3" {
		t.Errorf("expected spec.replicas=3, got %v", spec["replicas"])
	}
}

func TestSetNestedPatch_DeepNestedPath(t *testing.T) {
	patch := map[string]interface{}{}
	setNestedPatch(patch, "spec.database.engine", "postgres")
	spec := patch["spec"].(map[string]interface{})
	db := spec["database"].(map[string]interface{})
	if db["engine"] != "postgres" {
		t.Errorf("expected engine=postgres, got %v", db["engine"])
	}
}

func TestSetNestedPatch_MultipleFieldsSameParent(t *testing.T) {
	patch := map[string]interface{}{}
	setNestedPatch(patch, "spec.replicas", "3")
	setNestedPatch(patch, "spec.image", "nginx:1.25")
	spec := patch["spec"].(map[string]interface{})
	if spec["replicas"] != "3" {
		t.Errorf("replicas: expected 3, got %v", spec["replicas"])
	}
	if spec["image"] != "nginx:1.25" {
		t.Errorf("image: expected nginx:1.25, got %v", spec["image"])
	}
}

func TestSetNestedPatch_OverwritesExistingValue(t *testing.T) {
	patch := map[string]interface{}{}
	setNestedPatch(patch, "spec.replicas", "3")
	setNestedPatch(patch, "spec.replicas", "5")
	spec := patch["spec"].(map[string]interface{})
	if spec["replicas"] != "5" {
		t.Errorf("expected overwritten value 5, got %v", spec["replicas"])
	}
}

func TestSetNestedPatch_EmptyPath_SingleSegment(t *testing.T) {
	patch := map[string]interface{}{}
	setNestedPatch(patch, "key", "value")
	if patch["key"] != "value" {
		t.Errorf("expected key=value, got %v", patch["key"])
	}
}

func TestSetNestedPatch_SiblingPaths_DoNotClobber(t *testing.T) {
	patch := map[string]interface{}{}
	setNestedPatch(patch, "metadata.labels.app", "website")
	setNestedPatch(patch, "metadata.labels.env", "production")
	meta := patch["metadata"].(map[string]interface{})
	labels := meta["labels"].(map[string]interface{})
	if labels["app"] != "website" {
		t.Errorf("app label: expected website, got %v", labels["app"])
	}
	if labels["env"] != "production" {
		t.Errorf("env label: expected production, got %v", labels["env"])
	}
}
