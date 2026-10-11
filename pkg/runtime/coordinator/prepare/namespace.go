package prepare

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
)

// NamespaceGuardResult holds the outcome of a namespace restriction check.
type NamespaceGuardResult struct {
	Allowed   bool
	Namespace string
	Pattern   string // matched restricted or allowed pattern
	Reason    string // "restricted", "not-allowed", or ""
}

// CheckNamespace determines whether a target namespace is permitted for this CR.
//
// Precedence:
//  1. RestrictedNamespaces — deny-list (always wins)
//  2. AllowedNamespaces    — allow-list (empty = allow all)
func CheckNamespace(
	ctx context.Context,
	obj domain.Object,
	targetNamespace string,
	restricted types.RestrictedNamespaces,
	allowed types.AllowedNamespaces,
	crdName string,
) *NamespaceGuardResult {
	if restricted.IsRestricted(targetNamespace) {
		pattern := matchedRestrictedPattern(targetNamespace, restricted)
		logger.FromContext(ctx).Warn().
			Str("crd", crdName).
			Str("cr", fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())).
			Str("targetNamespace", targetNamespace).
			Str("pattern", pattern).
			Msg("namespace guard: reconcile skipped — namespace is restricted")
		return &NamespaceGuardResult{Allowed: false, Namespace: targetNamespace, Pattern: pattern, Reason: "restricted"}
	}

	if len(allowed) > 0 && !allowed.IsAllowed(targetNamespace) {
		pattern := matchedAllowedPattern(targetNamespace, allowed)
		logger.FromContext(ctx).Warn().
			Str("crd", crdName).
			Str("cr", fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())).
			Str("targetNamespace", targetNamespace).
			Msg("namespace guard: reconcile skipped — namespace not in allowed list")
		return &NamespaceGuardResult{Allowed: false, Namespace: targetNamespace, Pattern: pattern, Reason: "not-allowed"}
	}

	return &NamespaceGuardResult{Allowed: true, Namespace: targetNamespace}
}

// EventMessage returns the Kubernetes event message for a blocked namespace.
func (r *NamespaceGuardResult) EventMessage(resourceKind, resourceName string) string {
	switch r.Reason {
	case "restricted":
		return fmt.Sprintf(
			"Skipped %s %q in namespace %q — namespace is restricted (matched pattern: %q)",
			resourceKind, resourceName, r.Namespace, r.Pattern,
		)
	case "not-allowed":
		return fmt.Sprintf(
			"Skipped %s %q in namespace %q — namespace is not in allowedNamespaces (closest match: %q)",
			resourceKind, resourceName, r.Namespace, r.Pattern,
		)
	default:
		return ""
	}
}

func matchedRestrictedPattern(namespace string, restricted types.RestrictedNamespaces) string {
	for _, pattern := range restricted {
		if types.MatchesPattern(namespace, pattern) {
			return pattern
		}
	}
	return ""
}

func matchedAllowedPattern(namespace string, allowed types.AllowedNamespaces) string {
	for _, pattern := range allowed {
		if types.MatchesPattern(namespace, pattern) {
			return pattern
		}
	}
	return ""
}
