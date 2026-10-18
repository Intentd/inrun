package kube

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// ValidKubernetesName uses IsDNS1123Subdomain tests for a string that conforms to the
// definition of a subdomain in DNS (RFC 1123).
func ValidKubernetesName(name string) error {
	if errs := validation.IsDNS1123Label(name); len(errs) > 0 {
		return fmt.Errorf("invalid name %q: %s", name, strings.Join(errs, "; "))
	}
	return nil
}
