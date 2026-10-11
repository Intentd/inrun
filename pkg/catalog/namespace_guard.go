package catalog

import (
	"fmt"

	"github.com/intentd/inrun/pkg/utils"
)

// NamespaceRuleEntry holds the raw allowed/restricted namespace lists for one CRD.
// Returned by NamespaceProtectionRuleMap so callers can build their own lookup structures.
type NamespaceRuleEntry struct {
	Allowed    []string
	Restricted []string
}

// NamespaceProtectionRuleMap returns the allowed/restricted namespace lists for every
// CRD that declares namespace rules. Key format is "plural.group" — matching the
// lookup key used by the /namespace-protection handler.
//
// Returns nil when namespace protection is disabled.
func (k *Catalog) NamespaceProtectionRuleMap() map[string]NamespaceRuleEntry {
	if !k.IsNamespaceProtectionEnabled() {
		return nil
	}

	out := make(map[string]NamespaceRuleEntry)
	for _, crd := range k.enabledCRDs {
		if crd.IsBuiltIn || !crd.HasNamespaceRules() {
			continue
		}
		key := crd.APITypes.Plural + "." + crd.APITypes.Group
		out[key] = NamespaceRuleEntry{
			Allowed:    []string(crd.AllAllowedNamespaces()),
			Restricted: []string(crd.AllRestrictedNamespaces()),
		}
	}

	return out
}

// NamespaceProtectionGVRs returns the list of GVRs for CRDs that declare
// allowedNamespaces or restrictedNamespaces. Only these CRDs are intercepted
// by the namespace-protection webhook.
//
// When running outside the cluster (e.g. `inrun`), the webhook cannot be
// reached, so no rules are returned.
func (k *Catalog) NamespaceProtectionGVRs() []GVREntry {
	if !k.IsNamespaceProtectionEnabled() {
		return nil
	}
	if !utils.IsRunningInPod() {
		return nil
	}

	var out []GVREntry

	for _, crd := range k.enabledCRDs {
		// Skip built-ins — they are not CRDs and do not have namespace rules.
		if crd.IsBuiltIn {
			continue
		}

		// Skip CRDs with no namespace restrictions declared.
		if !crd.HasNamespaceRules() {
			continue
		}

		// Compose the GVR from the CRD's API types.
		out = append(out, GVREntry{
			Key:        fmt.Sprintf("%s/%s/%s", crd.APITypes.Group, crd.APITypes.Version, crd.APITypes.Plural),
			Group:      crd.APITypes.Group,
			Version:    crd.APITypes.Version,
			Resource:   crd.APITypes.Plural,
			Operations: []string{"CREATE", "UPDATE"},
		})
	}

	return out
}
