// pkg/types/conversion.go
package types

// CRDConversion declares declarative version conversion rules for a CRD.
// When enabled, Inrun serves the /convert endpoint and translates
// ConversionReview requests using these rules and the template resolver.
//
// Example Catalog declaration:
//
//	conversion:
//	  storageVersion: v1
//	  paths:
//	    - from: v1alpha1
//	      to: v1
//	      spec:
//	        image: "{{ .spec.image }}"
//	        replicas: "{{ .spec.replicas }}"
//	        seo:
//	          enabled: false   # default — v1alpha1 has no SEO field
//	    - from: v1
//	      to: v1alpha1
//	      spec:
//	        image: "{{ .spec.image }}"
//	        replicas: "{{ .spec.replicas }}"
//	        theme: "default"   # default — v1 has no theme field
type CRDConversion struct {
	// Include is a path (relative to the catalog file) to a YAML file whose
	// top-level value is a list of ConversionPath entries under a "paths:" key.
	// Expanded at load time — included paths come first, inline paths append after.
	Include string `yaml:"include,omitempty" json:"include,omitempty"`

	// Participant marks this CRD as a conversion participant without declaring
	// conversion paths. Use this on the stable/storage-version CRD when the
	// conversion paths are declared on the other version. Both CRDs need to
	// be marked (one with paths, one with participant: true) for conversion
	// stats to appear for each in the Console.
	Participant bool `yaml:"participant,omitempty" json:"participant,omitempty"`

	// StorageVersion — the version all objects are stored as internally.
	// All conversion paths route through this version.
	StorageVersion string `yaml:"storageVersion,omitempty" json:"storageVersion,omitempty"`

	// Paths — one entry per (from, to) pair.
	// You need at least two paths for a two-version CRD:
	//   - old → storage  (up-conversion)
	//   - storage → old  (down-conversion)
	Paths []ConversionPath `yaml:"paths,omitempty" json:"paths,omitempty"`

	// ManageCRD — when true and Inrun generates the webhook certificate, Inrun
	// owns the CRD's spec.conversion: it points the conversion webhook at the
	// gateway with the generated CA, at startup and again whenever it drifts.
	// With a certificate configured in security.webhooks you configure
	// spec.conversion yourself, and this field has no effect.
	// Default: true on the entry that declares paths, false on a participant.
	ManageCRD *bool `yaml:"manageCRD,omitempty" json:"manageCRD,omitempty"`
}

// ConversionPath declares one explicit conversion mapping.
// Both From and To are bare version strings — not full apiVersion strings.
//
//	from: v1alpha1   ← bare version, not "demo.inrun.dev/v1alpha1"
//	to: v1
//	spec:
//	  image: "{{ .spec.image }}"
//	  seo:
//	    enabled: false
type ConversionPath struct {
	// From — the source version (bare, e.g. "v1alpha1")
	From string `yaml:"from" validate:"required" json:"from"`

	// To — the target version (bare, e.g. "v1")
	To string `yaml:"to" validate:"required" json:"to"`

	// Spec — the output spec in the target version's format.
	// Values support Go template expressions evaluated against the source object.
	// Static values are used as-is.
	Spec map[string]interface{} `yaml:"spec" validate:"required" json:"spec"`
}

// ConversionRules is the runtime form of CRDConversion, keyed by Kind.
// Registered in the InMemoryConversionRegistry at Catalog load time.
type ConversionRules struct {
	Kind           string           `json:"kind" yaml:"kind"`
	StorageVersion string           `json:"storageVersion" yaml:"storageVersion"`
	Paths          []ConversionPath `json:"paths" yaml:"paths"`
}

// FindPath returns the conversion path for a given (from, to) pair.
// Both fromVersion and toVersion must be bare version strings.
// Returns nil when no path matches.
func (r *ConversionRules) FindPath(fromVersion, toVersion string) *ConversionPath {
	for i := range r.Paths {
		p := &r.Paths[i]
		if p.From == fromVersion && p.To == toVersion {
			return p
		}
	}
	return nil
}
