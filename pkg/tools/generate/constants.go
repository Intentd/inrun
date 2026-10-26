package generate

const (
	// TypeRegistryPackage is the fixed output directory for all generated typeregistry files.
	// Both the registry and declarative hook implementations are written here.
	// The Inrun runtime imports this package directly.
	TypeRegistryPackage = "pkg/typeregistry"

	// RegistryFile is the generated file containing:
	//   - ObjectRegistry
	//   - ListRegistry
	//   - HookRegistry
	//   - ReconcilerRegistry
	//   - SchemeAdderFns entries
	//
	// It is regenerated on every `inrun generate registry` invocation.
	RegistryFile = "zz_generated_typeregistry.go"
)
