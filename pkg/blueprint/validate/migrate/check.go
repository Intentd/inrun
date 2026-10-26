// Package migrate checks a CRDs configuration.
package migrate

import (
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/validate/internal/check"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/util/kube"
)

type checker struct{ check.Checker }

// Check runs this block's checks in order and returns the first error.
func Check(bp *blueprint.Blueprint, cfg *config.Config) error {
	c := &checker{check.New(bp, cfg)}
	return check.First(
		c.validateMigrate,
	)
}

var (
	buildFuncMapForValidation = check.FuncMap
	checkReserved             = check.CheckReserved
	failureMark               = check.FailureMark
	isTemplate                = check.IsTemplate
	readCRDSchema             = kube.ReadSchemaFromCRDFile
	compareSchemas            = kube.CompareSchemas
	validateTemplate          = check.Template
	isValidK8sName            = check.IsValidK8sName
	validateAnnotationKeys    = check.ValidateAnnotationKeys
	validateLabelMap          = check.ValidateLabelMap
	validateLabelSelector     = check.ValidateLabelSelector
)

// readCRDNamespaced reports whether the CRD file at path is namespaced.
func readCRDNamespaced(path string) (bool, error) {
	h, err := kube.ReadCRDFileHeader(path)
	if err != nil {
		return false, err
	}
	return h.Namespaced()
}
