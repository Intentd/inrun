package runtimeapi

// The runtime's HTTP routes for one CRD, by its catalog key (spec.crds.<key>).
// The runtime registers them and every client builds its URLs from them, so
// both sides change together.

// CatalogPath is the root of the runtime's catalog API.
const CatalogPath = "/catalog"

// CRDPath is a CRD's summary: health, metrics and configuration.
func CRDPath(crd string) string { return CatalogPath + "/" + crd }

// HealthPath is a CRD's health.
func HealthPath(crd string) string { return CRDPath(crd) + "/health" }

// CRListPath lists a CRD's CRs from the informer cache.
func CRListPath(crd string) string { return CRDPath(crd) + "/cr" }

// CRPath is one CR's detail; register CRListPath(crd)+"/" to serve it.
func CRPath(crd, namespace, name string) string {
	return CRListPath(crd) + "/" + namespace + "/" + name
}

// RawPath is a CRD's raw Catalog entry.
func RawPath(crd string) string { return CRDPath(crd) + "/raw" }

// EnrichedPath is a CRD's Catalog entry with defaults applied.
func EnrichedPath(crd string) string { return CRDPath(crd) + "/enriched" }
