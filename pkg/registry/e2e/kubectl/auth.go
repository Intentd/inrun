package kubectl

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/types"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func readAuth(ctx context.Context, cs kubernetes.Interface, e types.E2EKubectlAuth) (string, error) {
	attrs := &authorizationv1.ResourceAttributes{
		Verb:      e.Verb,
		Resource:  e.Resource,
		Namespace: e.Namespace,
	}

	var allowed bool
	if e.As == "" {
		// No impersonation: use SelfSubjectAccessReview (what kubectl auth can-i does by default).
		ssar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: attrs,
			},
		}
		result, err := cs.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, ssar, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("auth can-i %s %s: %w", e.Verb, e.Resource, err)
		}
		allowed = result.Status.Allowed
	} else {
		sar := &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				ResourceAttributes: attrs,
				User:               e.As,
			},
		}
		result, err := cs.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("auth can-i %s %s --as %s: %w", e.Verb, e.Resource, e.As, err)
		}
		allowed = result.Status.Allowed
	}

	out := "no"
	if allowed {
		out = "yes"
	}
	return out, assertOutput(out, e.TextAssertion, e.Assert, fmt.Sprintf("auth can-i %s %s", e.Verb, e.Resource))
}
