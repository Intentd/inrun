package module

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

func expandIncludes(m *types.Module, dir string) error {
	if err := types.ExpandFunctionsInclude(&m.Functions, dir); err != nil {
		return err
	}
	if err := types.ExpandPresetInclude(&m.Presets, dir); err != nil {
		return err
	}
	if err := types.ExpandStatusInclude(m.Status, dir); err != nil {
		return err
	}
	if m.Admission != nil {
		if err := types.ExpandValidationInclude(m.Admission.Validation, dir); err != nil {
			return err
		}
		if err := types.ExpandMutationInclude(m.Admission.Mutation, dir); err != nil {
			return err
		}
		if m.Admission.HasValidationExternal() {
			var err error
			m.Admission.Validation.External, err = types.ExpandExternalCalls(m.Admission.Validation.External, dir)
			if err != nil {
				return fmt.Errorf("admission.validation.external: %w", err)
			}
		}
		if m.Admission.HasMutationExternal() {
			var err error
			m.Admission.Mutation.External, err = types.ExpandExternalCalls(m.Admission.Mutation.External, dir)
			if err != nil {
				return fmt.Errorf("admission.mutation.external: %w", err)
			}
		}
	}
	return nil
}
