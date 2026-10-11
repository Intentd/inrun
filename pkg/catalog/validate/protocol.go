package validate

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// validatePortProtocols checks the protocol: field on all resource templates that
// declare a container port — Deployments, ReplicaSets, StatefulSets, and Pods.
//
// Services are validated separately in validateService; this covers workload types.
// Empty protocol values are skipped — they default to TCP at resolve time.
func (e *executor) validatePortProtocols() error {
	for crdName, crd := range e.cat.EnabledCRDs() {
		for _, e := range crd.CollectPortProtocolEntries() {
			if types.IsTemplate(e.Protocol) {
				continue // dynamic value — cannot validate at load time
			}
			if !types.IsValidProtocol(e.Protocol) {
				return errInvalidProtocolForResource(crdName, e.ResourceName, e.Phase, e.Protocol)
			}
		}
	}
	return nil
}

func errInvalidProtocolForResource(crd, resource, phase, protocol string) error {
	return fmt.Errorf(`
──────────────────────────────────────────────
%s Invalid protocol: %q
   CRD:      %s
   Resource: %s
   Phase:    %s

Allowed values:
  • TCP  (default — omit the field for TCP)
  • UDP
  • SCTP
──────────────────────────────────────────────`, failureMark(), protocol, crd, resource, phase)
}
