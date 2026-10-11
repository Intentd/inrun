package catalog

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

func populateAllServeFieldsFromInclude(entry *types.CRDEntry, catalogDir string) error {
	if err := types.ExpandServeInclude(entry.Serve, catalogDir); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	if err := types.ExpandServeTargetShorthand(entry.Serve); err != nil {
		return fmt.Errorf("serve.target: %w", err)
	}
	if err := types.ExpandServeTargetIncludes(entry.Serve, catalogDir); err != nil {
		return fmt.Errorf("serve.target: %w", err)
	}
	return nil
}

func populateStatusFieldsFromInclude(entry *types.CRDEntry, catalogDir string) error {
	if err := types.ExpandStatusInclude(entry.OperatorBox.EffectiveStatus(), catalogDir); err != nil {
		return fmt.Errorf("status: %w", err)
	}
	return nil
}

func populateValidationRulesFromInclude(entry *types.CRDEntry, catalogDir string) error {
	if err := types.ExpandValidationInclude(entry.EffectiveValidation(), catalogDir); err != nil {
		return fmt.Errorf("validation: %w", err)
	}
	return nil
}

func populateMutationRulesFromInclude(entry *types.CRDEntry, catalogDir string) error {
	if err := types.ExpandMutationInclude(entry.EffectiveMutation(), catalogDir); err != nil {
		return fmt.Errorf("mutation: %w", err)
	}
	return nil
}

func populateConversionPathsFromInclude(entry *types.CRDEntry, catalogDir string) error {
	if err := types.ExpandConversionInclude(entry.EffectiveConversion(), catalogDir); err != nil {
		return fmt.Errorf("conversion: %w", err)
	}
	return nil
}

func populateObserveInclude(entry *types.CRDEntry, catalogDir string) error {
	if entry.OperatorBox.Observe != nil {
		if err := types.ExpandObserveInclude(entry.OperatorBox.Observe, catalogDir); err != nil {
			return fmt.Errorf("operatorBox.observe: %w", err)
		}
	}

	if entry.Serve == nil {
		return nil
	}

	for name, cfg := range entry.Serve.Target.Entries {
		if cfg == nil || cfg.OperatorBox == nil || cfg.OperatorBox.Observe == nil {
			continue
		}

		if err := types.ExpandObserveInclude(cfg.OperatorBox.Observe, catalogDir); err != nil {
			return fmt.Errorf("serve.target[%q].operatorBox.observe: %w", name, err)
		}
	}

	return nil
}

func populateReconcilerFromInclude(entry *types.CRDEntry, catalogDir string) error {
	if err := types.ExpandReconcileInclude(entry.OperatorBox.Reconcile, catalogDir); err != nil {
		return fmt.Errorf("operatorBox.reconcile: %w", err)
	}
	if entry.Serve == nil {
		return nil
	}
	for name, cfg := range entry.Serve.Target.Entries {
		if cfg == nil || cfg.OperatorBox == nil {
			continue
		}
		if err := types.ExpandReconcileInclude(cfg.OperatorBox.Reconcile, catalogDir); err != nil {
			return fmt.Errorf("serve.target[%q].operatorBox.reconcile: %w", name, err)
		}
	}
	return nil
}
