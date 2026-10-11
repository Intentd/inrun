package web

import (
	"net/http"
	"sort"
	"strings"

	ccversion "github.com/inrundev/inrun/console/version"
)

// ChildResourceEntry describes one Kubernetes resource kind managed by the operator.
type ChildResourceEntry struct {
	Kind   string   // "Deployment", "ConfigMap", etc.
	Count  int      // total declared across all phases
	Phases []string // which lifecycle phases declare it: "onCreate", "onReconcile", "onDelete"
}

// DocsCatalogEntry is a single catalog shown on the docs landing page.
type DocsCatalogEntry struct {
	Name        string
	Description string
	Version     string
	Author      string
	CRDs        []DocsCRDEntry
}

// DocsCRDEntry is a CRD link on the docs landing page.
type DocsCRDEntry struct {
	Name        string
	CatalogName string
	State       string
}

// DocsLandingData is the view-model for the docs landing page.
type DocsLandingData struct {
	Catalogs       []DocsCatalogEntry
	ConsoleVersion string
}

// CRDDocsData is the view-model for a single CRD docs page.
type CRDDocsData struct {
	CatalogName      string
	CatalogVersion   string
	CRD              *CRDDetail
	KubectlKind      string
	KubectlKindLower string
	KubectlGroup     string
	KubectlVersion   string
	HasAdmission     bool
	HasConversion    bool
	HasProtection    bool
	HasRBAC          bool
	HasAutoscaler    bool
	HasChildren      bool
	ChildResources   []ChildResourceEntry
	ConsoleVersion   string
	RuntimeVersion   string
}

// handleDocsLanding renders /console/docs.
func (c *Console) handleDocsLanding(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var entries []DocsCatalogEntry
	for _, inst := range c.instances {
		if inst.Catalog == nil {
			continue
		}
		kat := inst.Catalog
		var crdEntries []DocsCRDEntry
		for _, c := range kat.CRDs {
			crdEntries = append(crdEntries, DocsCRDEntry{
				Name:        c.Name,
				CatalogName: kat.Name,
				State:       c.State,
			})
		}
		sort.Slice(crdEntries, func(i, j int) bool {
			return crdEntries[i].Name < crdEntries[j].Name
		})
		entries = append(entries, DocsCatalogEntry{
			Name:        kat.Name,
			Description: kat.Description,
			Version:     kat.Version,
			Author:      kat.Author,
			CRDs:        crdEntries,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	c.renderTemplate(w, "docs.html", DocsLandingData{
		Catalogs:       entries,
		ConsoleVersion: ccversion.Short(),
	})
}

// handleCatalogDocs serves /console/docs/{catalog} with the docs landing.
func (c *Console) handleCatalogDocs(w http.ResponseWriter, r *http.Request, _ string) {
	c.handleDocsLanding(w, r)
}

// handleCRDDocs renders /console/docs/{catalog}/{crd}.
func (c *Console) handleCRDDocs(w http.ResponseWriter, r *http.Request, catalogName, crdName string) {
	inst, ok := c.instanceByCatalogName(catalogName)
	if !ok {
		c.handleNotFound(w, r)
		return
	}

	crd, err := inst.Client.FetchCRDDetail(crdName, endpointInfoFor(inst.Catalog, crdName), summaryFor(inst.Catalog, crdName))
	if err != nil {
		Warnf("docs fetch CRD %s: %v", crdName, err)
		crd = &CRDDetail{
			Name:        crdName,
			State:       "offline",
			Description: "Unable to connect to Inrun runtime",
			GVK:         "unknown",
			LastError:   err.Error(),
		}
	}

	kubectlKind, kubectlGroup, kubectlVersion := parseGVK(crd.GVK)
	children := extractChildResources(crd.OperatorBox)

	c.renderTemplate(w, "crd_docs.html", CRDDocsData{
		CatalogName:      catalogName,
		CatalogVersion:   inst.Catalog.Version,
		CRD:              crd,
		KubectlKind:      kubectlKind,
		KubectlKindLower: strings.ToLower(kubectlKind),
		KubectlGroup:     kubectlGroup,
		KubectlVersion:   kubectlVersion,
		HasAdmission:     crd.Admission != nil && crd.Admission.WebhooksEnabled,
		HasConversion:    crd.Conversion != nil && crd.Conversion.Enabled,
		HasProtection: (crd.DeletionProtection != nil && crd.DeletionProtection.Enabled) ||
			(crd.NamespaceProtection != nil && crd.NamespaceProtection.Enabled),
		HasRBAC:        crd.RBAC.TotalRules > 0,
		HasAutoscaler:  crd.AutoscalerEnabled && crd.AutoscalerWorkers != nil,
		HasChildren:    len(children) > 0,
		ChildResources: children,
		ConsoleVersion: ccversion.Short(),
		RuntimeVersion: inst.Catalog.RuntimeVersion,
	})
}

// extractChildResources reads the operator box template summary and returns a
// sorted list of child resource kinds the operator manages for this CRD.
// The template summary is produced by the runtime's templateSummary function
// and lives at OperatorBox["templates"]["onCreate"|"onReconcile"|"onDelete"].
func extractChildResources(box map[string]interface{}) []ChildResourceEntry {
	if box == nil {
		return nil
	}
	templates, ok := box["templates"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Friendly display names for resource keys returned by templateSummary.
	kindNames := map[string]string{
		"deployments":            "Deployment",
		"services":               "Service",
		"pods":                   "Pod",
		"jobs":                   "Job",
		"cronJobs":               "CronJob",
		"configMaps":             "ConfigMap",
		"serviceAccounts":        "ServiceAccount",
		"statefulSets":           "StatefulSet",
		"daemonSets":             "DaemonSet",
		"ingresses":              "Ingress",
		"secrets":                "Secret",
		"persistentVolumeClaims": "PersistentVolumeClaim",
	}

	type kindData struct {
		count  int
		phases []string
	}
	byKey := make(map[string]*kindData)

	for _, phase := range []string{"onCreate", "onReconcile", "onDelete"} {
		phaseMap, ok := templates[phase].(map[string]interface{})
		if !ok {
			continue
		}
		for key, val := range phaseMap {
			n, _ := val.(float64) // JSON numbers decode as float64
			if int(n) == 0 {
				continue
			}
			if byKey[key] == nil {
				byKey[key] = &kindData{}
			}
			byKey[key].count += int(n)
			byKey[key].phases = append(byKey[key].phases, phase)
		}
	}

	if len(byKey) == 0 {
		return nil
	}

	result := make([]ChildResourceEntry, 0, len(byKey))
	for key, d := range byKey {
		name := kindNames[key]
		if name == "" {
			name = strings.Title(strings.TrimSuffix(key, "s")) //nolint:staticcheck
		}
		result = append(result, ChildResourceEntry{
			Kind:   name,
			Count:  d.count,
			Phases: d.phases,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Kind < result[j].Kind })
	return result
}

// parseGVK splits "group/version, Kind=Foo" into (kind, group, version).
func parseGVK(gvk string) (kind, group, version string) {
	if gvk == "" || gvk == "unknown" {
		return "Unknown", "", ""
	}
	parts := strings.SplitN(gvk, ", Kind=", 2)
	if len(parts) == 2 {
		kind = parts[1]
		gv := parts[0]
		if idx := strings.LastIndex(gv, "/"); idx >= 0 {
			group = gv[:idx]
			version = gv[idx+1:]
		} else {
			version = gv
		}
	} else {
		kind = gvk
	}
	return
}
