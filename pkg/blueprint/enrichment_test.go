// pkg/blueprint/enrichment_test.go
package blueprint

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func TestEnrichCRDEntry_KindOnly_Deployment(t *testing.T) {
	entry := &types.CRDEntry{
		Name: "deployment-governance",
		APITypes: types.APITypes{
			Kind: "Deployment", // only kind set
		},
	}

	outcome, err := EnrichCRDEntry(entry)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if outcome != types.EnrichmentApplied {
		t.Errorf("expected EnrichmentApplied, got %v", outcome)
	}
	if entry.APITypes.Group != "apps" {
		t.Errorf("expected group=apps, got %q", entry.APITypes.Group)
	}
	if entry.APITypes.Version != "v1" {
		t.Errorf("expected version=v1, got %q", entry.APITypes.Version)
	}
	if entry.APITypes.Plural != "deployments" {
		t.Errorf("expected plural=deployments, got %q", entry.APITypes.Plural)
	}
	if !entry.IsNamespaced() {
		t.Error("expected Namespaced=true for Deployment")
	}
	if !entry.IsBuiltIn {
		t.Error("expected IsBuiltIn=true after enrichment")
	}
}

func TestEnrichCRDEntry_KindOnly_Pod(t *testing.T) {
	entry := &types.CRDEntry{
		Name:     "pod-governance",
		APITypes: types.APITypes{Kind: "Pod"},
	}

	outcome, err := EnrichCRDEntry(entry)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != types.EnrichmentApplied {
		t.Errorf("expected EnrichmentApplied")
	}
	if entry.APITypes.Group != "" {
		t.Errorf("expected empty group for Pod, got %q", entry.APITypes.Group)
	}
	if entry.APITypes.Version != "v1" {
		t.Errorf("expected version=v1, got %q", entry.APITypes.Version)
	}
	if entry.APITypes.APIPath != "/api" {
		t.Errorf("expected APIPath=/api for core resource, got %q", entry.APITypes.APIPath)
	}
	if entry.BuiltInGroup != "core" {
		t.Errorf("expected BuiltInGroup=core, got %q", entry.BuiltInGroup)
	}
}

func TestEnrichCRDEntry_KindOnly_Namespace_ClusterScoped(t *testing.T) {
	entry := &types.CRDEntry{
		Name:     "namespace-governance",
		APITypes: types.APITypes{Kind: "Namespace"},
	}

	_, err := EnrichCRDEntry(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.IsNamespaced() {
		t.Error("Namespace is cluster-scoped — Namespaced should be false")
	}
}

func TestEnrichCRDEntry_FullySpecified_NotNeeded(t *testing.T) {
	entry := &types.CRDEntry{
		Name: "my-website",
		APITypes: types.APITypes{
			Kind:    "Website",
			Group:   "demo.inrun.dev",
			Version: "v1alpha1",
			Plural:  "websites",
		},
	}

	outcome, err := EnrichCRDEntry(entry)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != types.EnrichmentNotNeeded {
		t.Errorf("expected EnrichmentNotNeeded for fully specified entry")
	}
	if entry.APITypes.Group != "demo.inrun.dev" {
		t.Errorf("group should not have changed")
	}
}

func TestEnrichCRDEntry_UnknownKind_Error(t *testing.T) {
	entry := &types.CRDEntry{
		Name:     "my-custom-crd",
		APITypes: types.APITypes{Kind: "MyCustomResource"},
	}

	outcome, err := EnrichCRDEntry(entry)

	if err == nil {
		t.Fatal("expected error for unknown kind with missing apiTypes fields")
	}
	if outcome != types.EnrichmentFailed {
		t.Errorf("expected EnrichmentFailed")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "MyCustomResource") {
		t.Errorf("error should mention the kind: %q", errStr)
	}
	if !strings.Contains(errStr, "group") {
		t.Errorf("error should mention group: %q", errStr)
	}
}

func TestEnrichCRDEntry_PartiallySpecified_Error(t *testing.T) {
	entry := &types.CRDEntry{
		Name: "partial",
		APITypes: types.APITypes{
			Kind:  "Website",
			Group: "demo.inrun.dev",
		},
	}

	outcome, err := EnrichCRDEntry(entry)

	if err == nil {
		t.Fatal("expected error for partially specified apiTypes")
	}
	if outcome != types.EnrichmentFailed {
		t.Errorf("expected EnrichmentFailed")
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Errorf("error should mention incomplete apiTypes: %q", err.Error())
	}
}

func TestEnrichCRDEntry_CaseNormalization(t *testing.T) {
	entry := &types.CRDEntry{
		Name:     "dep-gov",
		APITypes: types.APITypes{Kind: "deployment"},
	}

	_, err := EnrichCRDEntry(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.APITypes.Kind != "Deployment" {
		t.Errorf("expected Kind=Deployment after normalization, got %q", entry.APITypes.Kind)
	}
}
