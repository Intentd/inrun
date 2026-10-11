package validate

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/labels"
)

// validateCRDEntryMetadata validates the labels: and annotations: blocks
// on each CRD entry.
func (e *executor) validateCRDEntryMetadata() error {
	if err := e.validateCRDEntryLabels(); err != nil {
		return err
	}
	if err := e.validateCRDEntryAnnotations(); err != nil {
		return err
	}
	return nil
}

// validateCRDEntryLabels validates the labels: block on each CRD entry.
//
// Enforces:
//  1. Label keys must be valid Kubernetes qualified names (static — no templates).
//  2. Label values must be valid Go templates (they are resolved at reconcile time
//     against the CR object, so {{ .metadata.name }} etc. are allowed).
func (e *executor) validateCRDEntryLabels() error {
	funcMap := buildFuncMapForValidation(e.k.Notes)
	for crdName, crd := range e.k.Enabled() {
		if !crd.HasUserLabels() {
			continue
		}

		lbls := crd.Labels
		if err := labels.CheckReserved("labels", lbls); err != nil {
			return err
		}
		for key, value := range lbls {
			if isTemplate(key) {
				return fmt.Errorf("%s CRD %q: labels: key %q must be a static label key, not a template", failureMark(), crdName, key)
			}
			if errs := isValidLabelKey(key); len(errs) > 0 {
				return fmt.Errorf("%s CRD %q: labels: key %q is not a valid Kubernetes label key: %s", failureMark(), crdName, key, strings.Join(errs, "; "))
			}
			if isTemplate(value) {
				if err := validateTemplate("labels", crdName, key, "value", value, funcMap); err != nil {
					return err
				}
			} else {
				if errs := isValidLabelValue(value); len(errs) > 0 {
					return fmt.Errorf("%s CRD %q: labels: value %q is not a valid Kubernetes label value: %s", failureMark(), crdName, value, strings.Join(errs, "; "))
				}
			}
		}
	}
	return nil
}

// validateCRDEntryAnnotations validates the annotations: block on each CRD entry.
//
// Enforces:
//  1. Annotation keys must be valid Kubernetes qualified names (static — no templates).
//  2. Annotation values must be valid Go templates (they are resolved at reconcile time
//     against the CR object, so {{ .metadata.name }} etc. are allowed).
func (e *executor) validateCRDEntryAnnotations() error {
	funcMap := buildFuncMapForValidation(e.k.Notes)

	for crdName, crd := range e.k.Enabled() {
		if !crd.HasUserAnnotations() {
			continue
		}

		ann := crd.Annotations
		if err := labels.CheckReserved("annotations", ann); err != nil {
			return err
		}
		for key, value := range ann {
			if isTemplate(key) {
				return fmt.Errorf("%s CRD %q: annotations: key %q must be a static annotation key, not a template",
					failureMark(), crdName, key,
				)
			}

			if errs := isValidLabelKey(key); len(errs) > 0 {
				return fmt.Errorf("%s CRD %q: annotations: key %q is not a valid Kubernetes annotation key: %s",
					failureMark(), crdName, key, strings.Join(errs, "; "),
				)
			}

			if err := validateTemplate("annotations", crdName, key, "value", value, funcMap); err != nil {
				return err
			}
		}
	}

	return nil
}
