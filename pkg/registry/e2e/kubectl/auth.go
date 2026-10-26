package kubectl

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/types"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func kubectlAuth(ctx context.Context, cs kubernetes.Interface, e types.E2EKubectlAuth) error {
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
			return fmt.Errorf("auth can-i %s %s: %w", e.Verb, e.Resource, err)
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
			return fmt.Errorf("auth can-i %s %s --as %s: %w", e.Verb, e.Resource, e.As, err)
		}
		allowed = result.Status.Allowed
	}

	out := "no"
	if allowed {
		out = "yes"
	}
	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("auth can-i %s %s: %w\nresult: %s", e.Verb, e.Resource, err, out)
	}
	return nil
}
