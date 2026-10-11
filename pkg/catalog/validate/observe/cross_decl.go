package observe

import (
	"fmt"
	"strings"
)

// validateCrossDecl validates all cross: declarations in a catalog.
// Rules:
//   - No self referencing
//   - crd: and labels: blocks cannot be both declared in the same cross declaration
//   - Every crd without source block (same catalog) must exist in the catalog
//   - All "as:" fields must be unique across the entire catalog (not just per-CRD)
//   - When label is used, 'as' is required for cross reference. Nothing to default to
//   - Only allow known ONCOP protocols
//   - Only one of the optional token and SecretRef is allowed
//   - If cross.selector is used, only a pair of either name/namespace or matchLabels is used.
//   - If namespace is defined without name in selector, an error.
func (c *checker) validateCrossDecl() error {
	// seen tracks all alias (as:) values across the entire catalog — uniqueness
	// is a catalog-wide constraint, not per-CRD.
	seen := make(map[string]bool)

	for name, crd := range c.Cat.EnabledCRDs() {
		if !crd.HasCrossDecl() {
			continue
		}

		decl := crd.Box().EffectiveCross()

		for i, cross := range decl {
			crsCRD := cross.CRD
			sel := cross.LabelSelector
			alias := cross.As

			// require either crd or labels
			if crsCRD == "" && sel == nil {
				return fmt.Errorf("%s CRD %q: cross[%d]: crd or labels required.", failureMark(), name, i)
			}

			// enforce only one mode is used
			if cross.HasCRDAndLabelDecl() {
				return fmt.Errorf("%s CRD %q: cross[%d]: only one of 'crd:' or 'labels:' is required.", failureMark(), name, i)
			}

			// self-reference
			if crsCRD == name {
				return fmt.Errorf("%s CRD %q: cross: a CRD cannot reference itself: cross[%d].crd: %s",
					failureMark(), name, i, crsCRD)
			}

			// crd not found in catalog
			if cross.IsCRDBased() && !cross.HasSource() {
				if result := c.Cat.LookupByName(crsCRD); result.Entry() == nil {
					return fmt.Errorf("%s CRD %q: cross[%d].crd: %q not found", failureMark(), name, i, crsCRD)
				}
			}

			// label-based: as is required, lookup must resolve
			if cross.IsLabelBased() {
				if alias == "" {
					return fmt.Errorf("%s CRD %q: cross[%d].labels: 'as' is required for cross referencing", failureMark(), name, i)
				}
				if result := c.Cat.LookupByLabel(sel); result.Entry() == nil {
					return fmt.Errorf("%s CRD %q: cross[%d].labels: %s not found in any CRD in the catalog", failureMark(), name, i, sel.String())
				}
			}

			// alias
			if alias != "" {
				if err := isValidResolverName(alias); err != nil {
					return fmt.Errorf("%s CRD %q: invalid 'as' name: cross[%d].as: %s - %w",
						failureMark(), name, i, alias, err)
				}
				// uniqueness is catalog-wide
				if seen[alias] {
					return fmt.Errorf("%s CRD %q: cross: alias (as) %q is already used in another CRD — as: must be unique across the catalog",
						failureMark(), name, alias)
				}
				seen[alias] = true
			}

			// cross.source
			if cross.HasSource() {
				cs := cross.Source
				pr := cs.Protocol
				if pr != "" {
					if !cross.IsValid(pr.String()) {
						return fmt.Errorf("%s CRD %q: invalid ONCOP protocol: cross[%d].source.%s. Choose either of these: %s",
							failureMark(), name, i, pr, strings.Join(cross.ValidONCOProtocols(), ", "))
					}
				}

				if cs.HasAuth() {
					if cs.HasTokenAndSecretRef() {
						return fmt.Errorf("%s CRD %q: declared both token and secretRef. Only one is allowed: cross[%d].source",
							failureMark(), name, i)
					}

					if cs.HasSecretRef() {
						ref := cs.Auth.SecretRef
						if ref.Name == "" {
							return fmt.Errorf("%s CRD %q: name is required for secretRef: cross[%d].source.auth.secretRef",
								failureMark(), name, i)
						}
						if ref.Key == "" {
							return fmt.Errorf("%s CRD %q: key is required for secretRef: cross[%d].source.auth.secretRef",
								failureMark(), name, i)
						}
						if ref.Namespace == "" {
							crd.Warnings.AddWarning(fmt.Sprintf("%s CRD %q: namespace is empty for secretRef: cross[%d].source.auth.secretRef. Defaults to inrun namespace.",
								failureMark(), name, i))
						}
					}
				}
			}
		}
		c.Cat.EnabledCRDs()[name] = crd
	}
	return nil
}
