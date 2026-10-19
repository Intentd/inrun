package reconcile

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// validatePortProtocols checks the protocol: field on all resource templates that
// declare a container port — Deployments, ReplicaSets, StatefulSets, and Pods.
//
// Services are validated separately in validateService; this covers workload types.
// Empty protocol values are skipped — they default to TCP at resolve time.
func (c *checker) validatePortProtocols() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectPortProtocolEntries() {
			if isTemplate(entry.Protocol) {
				continue // dynamic value — cannot validate at load time
			}
			if !types.IsValidProtocol(entry.Protocol) {
				return errInvalidProtocolForResource(crdName, entry.ResourceName, entry.Phase, entry.Protocol)
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
