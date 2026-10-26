package blueprint_test

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
)

const clusterCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: vaults.secrets.io
spec:
  group: secrets.io
  scope: Cluster
  names:
    kind: Vault
    plural: vaults
  versions:
    - name: v1
      served: true
      storage: true
`

func customEntry(cr types.CustomResourceTemplateSource) *types.CRDEntry {
	return &types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
		OnCreate: &types.HookTemplates{CustomResource: []types.CustomResourceTemplateSource{cr}},
	}}}
}

func TestPopulateCustomResources_FillsFromFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "vault.yaml", clusterCRD)
	entry := customEntry(types.CustomResourceTemplateSource{CRDFile: "vault.yaml"})

	if err := blueprint.PopulateCustomResourcesFromCRDFiles(entry, dir); err != nil {
		t.Fatal(err)
	}
	cr := entry.OperatorBox.Reconcile.OnCreate.CustomResource[0]
	if cr.APIVersion != "secrets.io/v1" || cr.Kind != "Vault" {
		t.Errorf("got %s %s; want secrets.io/v1 Vault", cr.APIVersion, cr.Kind)
	}
	if cr.Metadata.Namespaced == nil || *cr.Metadata.Namespaced {
		t.Errorf("namespaced = %v; want false from the Cluster scope", cr.Metadata.Namespaced)
	}
	if cr.CRDFile != "" || cr.ResolvedCRDFile == "" {
		t.Errorf("crdFile %q, resolved %q; want crdFile cleared and the path kept", cr.CRDFile, cr.ResolvedCRDFile)
	}
}

func TestPopulateCustomResources_DeclaredMustMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "vault.yaml", clusterCRD)
	yes := true
	for name, cr := range map[string]types.CustomResourceTemplateSource{
		"kind":       {CRDFile: "vault.yaml", Kind: "Safe"},
		"apiVersion": {CRDFile: "vault.yaml", APIVersion: "secrets.io/v2"},
		"namespaced": {CRDFile: "vault.yaml", Metadata: types.CustomResourceMetadata{Namespaced: &yes}},
	} {
		err := blueprint.PopulateCustomResourcesFromCRDFiles(customEntry(cr), dir)
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s mismatch: err = %v; want one naming %s", name, err, name)
		}
	}
}
