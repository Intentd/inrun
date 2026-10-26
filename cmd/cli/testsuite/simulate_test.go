//go:build !runtime && !gateway

package testsuite

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const twoWebsitesOneCert = `
apiVersion: example.com/v1
kind: Website
metadata:
  name: site-b
  namespace: default
spec:
  domain: shared.example.com
---
apiVersion: example.com/v1
kind: Website
metadata:
  name: site-a
  namespace: default
spec:
  domain: shared.example.com
---
apiVersion: example.com/v1
kind: Certificate
metadata:
  name: cert-a
  namespace: default
spec:
  domain: a.example.com
`

// mustParse parses the fixture and fails the test on error. The good-doc
// tests are about grouping, not about parse failures.
func mustParse(t *testing.T, data string) map[string][]*unstructured.Unstructured {
	t.Helper()
	crs, err := cmdutil.ParseMultiDocCRs([]byte(data))
	if err != nil {
		t.Fatalf("ParseMultiDocCRs: %v", err)
	}
	return crs
}

func TestParseMultiDocCRs_GroupsBySameKind(t *testing.T) {
	crs := mustParse(t, twoWebsitesOneCert)

	if len(crs["website"]) != 2 {
		t.Fatalf("expected 2 Website docs, got %d", len(crs["website"]))
	}
	if len(crs["certificate"]) != 1 {
		t.Fatalf("expected 1 Certificate doc, got %d", len(crs["certificate"]))
	}
	// File order preserved — site-b first, site-a second.
	if got := crs["website"][0].GetName(); got != "site-b" {
		t.Errorf("expected first Website doc to be site-b, got %s", got)
	}
	if got := crs["website"][1].GetName(); got != "site-a" {
		t.Errorf("expected second Website doc to be site-a, got %s", got)
	}
}

func TestResolveCRInputs_FirstDocIsReconciled(t *testing.T) {
	crs := mustParse(t, twoWebsitesOneCert)

	in, ok := cmdutil.ResolveCRInputs(crs, "Website")
	if !ok {
		t.Fatal("expected Website inputs to resolve")
	}
	if in.CR.GetName() != "site-b" {
		t.Errorf("expected cr under test to be the first doc (site-b), got %s", in.CR.GetName())
	}
	if len(in.Existing) != 1 || in.Existing[0].GetName() != "site-a" {
		t.Fatalf("expected exactly one existing instance (site-a), got %+v", in.Existing)
	}
	if len(in.Peers) != 1 || in.Peers["certificate"] == nil {
		t.Fatalf("expected certificate as the only peer, got %+v", in.Peers)
	}
	if _, hasSelfPeer := in.Peers["website"]; hasSelfPeer {
		t.Error("website should not appear in its own peers map")
	}
}

func TestResolveCRInputs_KindNotPresent(t *testing.T) {
	crs := mustParse(t, twoWebsitesOneCert)

	if _, ok := cmdutil.ResolveCRInputs(crs, "Deployment"); ok {
		t.Error("expected resolveCRInputs to report not-found for a kind absent from the file")
	}
}

func TestResolveCRInputs_SingleDoc_NoExistingInstances(t *testing.T) {
	crs := mustParse(t, twoWebsitesOneCert)

	in, ok := cmdutil.ResolveCRInputs(crs, "Certificate")
	if !ok {
		t.Fatal("expected Certificate inputs to resolve")
	}
	if len(in.Existing) != 0 {
		t.Errorf("single-doc kind should have no existing instances, got %+v", in.Existing)
	}
	if _, hasSelfPeer := in.Peers["certificate"]; hasSelfPeer {
		t.Error("certificate should not appear in its own peers map")
	}
	if in.Peers["website"] == nil {
		t.Error("expected website as a peer (first Website doc)")
	}
}

// TestParseMultiDocCRs_BadDocIsAnError proves a malformed document fails the
// whole parse with its position, instead of being silently dropped.
func TestParseMultiDocCRs_BadDocIsAnError(t *testing.T) {
	const bad = `
apiVersion: example.com/v1
kind: Website
metadata:
  name: good
---
apiVersion: example.com/v1
kind: Website
metadata:
  name: bad
  namespace: [unclosed
`
	_, err := cmdutil.ParseMultiDocCRs([]byte(bad))
	if err == nil {
		t.Fatal("expected a parse error for the malformed document, got nil")
	}
	if !strings.Contains(err.Error(), "document 2") {
		t.Errorf("error should name the failing document, got: %v", err)
	}
}

// TestParseMultiDocCRs_GoodDocsBeforeBadDoc proves the error is returned even
// though the first document parsed, so a caller cannot silently use a partial
// map.
func TestParseMultiDocCRs_GoodDocsBeforeBadDoc(t *testing.T) {
	const mixed = `
apiVersion: example.com/v1
kind: Website
metadata:
  name: good
---
apiVersion: example.com/v1
kind: Certificate
metadata:
  name: ok
---
  : not a mapping
`
	crs, err := cmdutil.ParseMultiDocCRs([]byte(mixed))
	if err == nil {
		t.Fatalf("expected error, got a %d-kind map", len(crs))
	}
	if crs != nil {
		t.Fatalf("expected nil map on parse failure, got %d kind(s)", len(crs))
	}
	if !strings.Contains(err.Error(), "document 3") {
		t.Errorf("error should name the failing document, got: %v", err)
	}
}
