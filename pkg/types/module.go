// pkg/types/module.go
package types

// Module is the smallest reusable primitive in Inrun's composition model.
// A Module declares named inputs and resource blocks. It cannot run alone —
// it must be imported by a Catalog that provides its inputs via with:.
//
// YAML:
//
//	apiVersion: inrun.dev/v1
//	kind: Module
//	metadata:
//	  name: postgres
//	inputs:
//	  - name: image
//	  - name: volumeSize
//	resources: ...
//	status: ...
//	admission:
//	  validation:
//	    rules:
//	      - field: spec.image
//	        prefix: "myregistry.com/"
//	        action: deny
//	  mutation:
//	    rules:
//	      - field: spec.replicas
//	        default: "2"
type Module struct {
	APIVersion string           `yaml:"apiVersion" json:"apiVersion"`
	Kind       string           `yaml:"kind" json:"kind"`
	Metadata   ModuleMeta       `yaml:"metadata" json:"metadata"`
	Inputs     []ModuleInput    `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Notes      NoteRegistry     `yaml:"notes,omitempty" json:"notes,omitempty"`
	Profiles   ProfileRegistry  `yaml:"profiles,omitempty" json:"profiles,omitempty"`
	Resources  *ModuleResources `yaml:"resources,omitempty" json:"resources,omitempty"`
	Status     *StatusConfig    `yaml:"status,omitempty" json:"status,omitempty"`
	Admission  *Admission       `yaml:"admission,omitempty" json:"admission,omitempty"`
}

// ModuleResources groups the resources a Module contributes to a CRD entry.
// Resources declared directly under resources: are merged into onReconcile.
// Resources declared under resources.onCreate: are merged into onCreate,
// making them immune to the update=true path (correct for once: true secrets).
type ModuleResources struct {
	// OnCreate groups resources that must only be processed during creation —
	// never updated on subsequent reconciles. Secrets with once: true belong here.
	OnCreate *HookTemplates `yaml:"onCreate,omitempty" json:"onCreate,omitempty"`

	// All remaining HookTemplates fields are promoted to the resources: level
	// and merged into the CRD's onReconcile phase.
	HookTemplates `yaml:",inline"`
}

// ModuleMeta holds Module identity fields.
type ModuleMeta struct {
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version,omitempty" json:"version,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Author      string   `yaml:"author,omitempty" json:"author,omitempty"`
	License     string   `yaml:"license,omitempty" json:"license,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// ModuleInput declares one input parameter for a Module.
type ModuleInput struct {
	// Name is the input identifier referenced in templates as inputs.Name.
	Name string `yaml:"name" json:"name"`

	// Description explains what this input controls.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Required — when true, the importing Catalog must provide this input
	// in its with: block. Validation fails if required inputs are missing.
	Required bool `yaml:"required,omitempty" json:"required,omitempty"`

	// Type hints at the expected type of the input value. Not currently enforced,
	// but reserved for future type checking or schema generation.
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Default is the value used when the input is not provided in with:.
	// Only valid when Required is false.
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
}

// ModuleImport declares one Module import inside an operatorBox.
// Follows the same resolution semantics as RegistrySource in a Stack —
// if you know how to pull a pattern, you already know how to pull a Module.
//
// YAML inside operatorBox:
//
// File (developer path):
//
//	imports:
//	  - module: ./modules/postgres/module.yaml
//	    with:
//	      image: "postgres:16"
//
// OCI registry (the Inrun registry houses both patterns and modules):
//
//	imports:
//	  - module: ghcr.io/intentd/registry/postgres@v16
//	    oci: true
//	    with:
//	      image: "{{ .spec.postgresImage }}"
//
// Git registry:
//
//	imports:
//	  - module: https://github.com/myorg/postgres-module@main
//	    with:
//	      image: "{{ .spec.postgresImage }}"
type ModuleImport struct {
	// Module is the registry URL, file path, or short name.
	// Same formats as RegistrySource.URL:
	//   File:  ./postgres/module.yaml
	//   OCI:   ghcr.io/intentd/registry/postgres@v16
	//   Git:   https://github.com/myorg/postgres-module@main
	// @ shorthand encodes the version inline: url@version
	Module string `yaml:"module" json:"module,omitempty"`

	// Version — explicit version (tag, branch, or SHA).
	// Ignored when @ shorthand is used in Module.
	// Defaults to "latest" for OCI, "main" for Git.
	Version string `yaml:"version,omitempty" json:"version,omitempty"`

	// OCI — when true, pull the Module artifact via OCI/ORAS protocol.
	// When false (default), pull via Git (GitHub raw URL, GitLab, or git clone).
	OCI bool `yaml:"oci,omitempty" json:"oci,omitempty"`

	// Auth — optional credentials for the registry.
	// Same auth model as RegistrySource.Auth — resolved from environment variables.
	Auth *FileSourceAuth `yaml:"auth,omitempty" json:"auth,omitempty"`

	// With binds the Module's declared inputs to values.
	// Values are template expressions evaluated in the CRD's reconcile context.
	// Required inputs not provided here are a validation error.
	// Optional inputs not provided use their Module-declared defaults.
	With map[string]string `yaml:"with,omitempty" json:"with,omitempty"`
}
