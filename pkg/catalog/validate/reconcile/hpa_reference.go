package reconcile

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// validateHPAReference ensures that every HPA declaration has a valid ScaleTargetRef.
func (c *checker) validateHPAReference() error {
	for crdName, crd := range c.Cat.EnabledCRDs() {
		if !crd.HasAnyHookTemplates() || !crd.HasAnyHPA() {
			continue
		}

		if crd.HasOnCreate() {
			for _, h := range crd.Box().EffectiveOnCreate().HorizontalPodAutoscalers {
				if err := validateOneHPARef(crdName, h.Name, h.ScaleTargetRef); err != nil {
					return fmt.Errorf(failureMark(), err)
				}
			}
		}

		if crd.HasOnReconcile() {
			for _, h := range crd.Box().EffectiveOnReconcile().HorizontalPodAutoscalers {
				if err := validateOneHPARef(crdName, h.Name, h.ScaleTargetRef); err != nil {
					return fmt.Errorf(failureMark(), err)
				}
			}
		}
	}
	return nil
}

func validateOneHPARef(crdName, hpaName string, ref types.ScaleTargetRef) error {
	appsV1Kinds := map[string]bool{"Deployment": true, "StatefulSet": true, "ReplicaSet": true}
	if ref.APIVersion == "" && appsV1Kinds[ref.Kind] {
		ref.APIVersion = "apps/v1"
	}
	if ref.APIVersion == "" {
		return fmt.Errorf(
			"%s invalid HPA ScaleTargetRef in CRD %q (hpa %q): missing apiVersion\n\n"+
				"Example:\n"+
				"  ScaleTargetRef:\n"+
				"    apiVersion: apps/v1\n"+
				"    kind: Deployment\n"+
				"    name: my-app",
			failureMark(), crdName, hpaName,
		)
	}
	if ref.Kind == "" {
		return fmt.Errorf(
			"%s invalid HPA ScaleTargetRef in CRD %q (hpa %q): missing kind\n\n"+
				"Example:\n"+
				"  ScaleTargetRef:\n"+
				"    apiVersion: apps/v1\n"+
				"    kind: ReplicaSet\n"+
				"    name: my-app",
			failureMark(), crdName, hpaName,
		)
	}
	if ref.Name == "" {
		return fmt.Errorf(
			"invalid HPA ScaleTargetRef in CRD %q (hpa %q): missing name\n\n"+
				"Example:\n"+
				"  ScaleTargetRef:\n"+
				"    apiVersion: apps/v1\n"+
				"    kind: StatefulSet\n"+
				"    name: my-app",
			crdName, hpaName,
		)
	}
	return nil
}
