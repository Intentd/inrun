// pkg/catalog/module_validate.go
package catalog

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/registry/module"
	"github.com/intentd/inrun/pkg/types"
)

// ModuleValidationError represents a single validation failure for a Module.
type ModuleValidationError struct {
	Path    string
	Message string
}

func (e ModuleValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return e.Message
}

// ValidateModule validates a Module YAML file at the given path.
// Returns a slice of errors — empty means valid.
func ValidateModule(path string) []ModuleValidationError {
	var errs []ModuleValidationError

	m, err := module.Load(path)
	if err != nil {
		return []ModuleValidationError{{Path: path, Message: err.Error()}}
	}

	if m.Metadata.Name == "" {
		errs = append(errs, ModuleValidationError{Path: "metadata.name", Message: "name is required"})
	}

	seen := make(map[string]bool)
	for i, input := range m.Inputs {
		if input.Name == "" {
			errs = append(errs, ModuleValidationError{
				Path:    fmt.Sprintf("inputs[%d].name", i),
				Message: "input name is required",
			})
			continue
		}
		if seen[input.Name] {
			errs = append(errs, ModuleValidationError{
				Path:    fmt.Sprintf("inputs[%d].name", i),
				Message: fmt.Sprintf("duplicate input name: %s", input.Name),
			})
		}
		seen[input.Name] = true

		if input.Required && input.Default != "" {
			errs = append(errs, ModuleValidationError{
				Path: fmt.Sprintf("inputs[%d]", i),
				Message: fmt.Sprintf(
					"input %q is required but also has a default — required inputs must not have defaults",
					input.Name,
				),
			})
		}
	}

	if m.Resources == nil {
		errs = append(errs, ModuleValidationError{
			Path:    "resources",
			Message: "resources block is required in a Module",
		})
	}

	if m.Resources != nil {
		for _, msg := range module.ValidateModuleTemplates(m) {
			errs = append(errs, ModuleValidationError{Path: "resources", Message: msg})
		}
	}

	return errs
}

// ValidateModuleImports validates that all imports in an operatorBox have
// required inputs provided in their with: block.
func ValidateModuleImports(crdName string, imports []types.ModuleImport) []ModuleValidationError {
	var errs []ModuleValidationError

	for i, imp := range imports {
		m, err := module.LoadImport(&imp)
		if err != nil {
			errs = append(errs, ModuleValidationError{
				Path:    fmt.Sprintf("spec.crds.%s.operatorBox.imports[%d]", crdName, i),
				Message: fmt.Sprintf("loading module %q: %s", imp.Module, err),
			})
			continue
		}

		for _, input := range m.Inputs {
			if input.Required {
				if _, ok := imp.With[input.Name]; !ok {
					errs = append(errs, ModuleValidationError{
						Path: fmt.Sprintf("spec.crds.%s.operatorBox.imports[%d]", crdName, i),
						Message: fmt.Sprintf(
							"import %q is missing required input %q\n"+
								"  Module requires: %s\n"+
								"  Provided: %s\n"+
								"  Missing: %s",
							imp.Module, input.Name,
							moduleRequiredInputList(m.Inputs),
							moduleProvidedInputList(imp.With),
							input.Name,
						),
					})
				}
			}
		}

		declared := make(map[string]bool)
		for _, input := range m.Inputs {
			declared[input.Name] = true
		}
		for name := range imp.With {
			if !declared[name] {
				errs = append(errs, ModuleValidationError{
					Path: fmt.Sprintf("spec.crds.%s.operatorBox.imports[%d]", crdName, i),
					Message: fmt.Sprintf(
						"unknown input %q supplied in with: — module %q does not declare it",
						name, imp.Module,
					),
				})
			}
		}
	}

	return errs
}

func moduleRequiredInputList(inputs []types.ModuleInput) string {
	var names []string
	for _, input := range inputs {
		if input.Required {
			names = append(names, input.Name)
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

func moduleProvidedInputList(with map[string]string) string {
	if len(with) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(with))
	for k := range with {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}
