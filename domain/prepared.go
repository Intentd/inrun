package domain

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// ToTyped converts the prepared Object into a typed CR.
// T must be a struct type — pass the kind, not a pointer.
//
//	node, err := domain.ToTyped[apiv1.BlockchainNode](req.Prepared)
func ToTyped[T any](p *PreparedRequest) (*T, error) {
	out := new(T)
	if err := fromUnstructured(p.Object, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ToTypedWith converts a domain.Object to the type newObj returns. PTR may be
// the interface domain.Object (the generic.Reconciler case), so the target is
// decided by what newObj builds, not by PTR: an unstructured object is
// converted whenever newObj builds a typed one.
//
//	obj, err := domain.ToTypedWith[PTR](prepared.Object, r.newObj)
func ToTypedWith[PTR Object](obj Object, newObj func() PTR) (PTR, error) {
	out := newObj()
	_, isU := obj.(*unstructured.Unstructured)
	_, wantU := any(out).(*unstructured.Unstructured)
	if typed, ok := obj.(PTR); ok && (!isU || wantU) {
		return typed, nil
	}
	if err := fromUnstructured(obj, out); err != nil {
		return out, err
	}
	return out, nil
}

// fromUnstructured converts a domain.Object (expected to be *unstructured.Unstructured)
// into out using the default unstructured converter.
func fromUnstructured(obj Object, out any) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("domain: expected *unstructured.Unstructured, got %T", obj)
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, out)
}

// PreparedContext is the runtime-enriched resolver context handed to a reconciler.
// It is an interface so domain does not import the concrete template resolver package.
// *template.Resolver satisfies this interface.
type PreparedContext interface {
	// Resolve evaluates a Go template expression against the enriched CR context.
	Resolve(expr string) (string, error)
	// Data returns the full map available to template expressions.
	Data() map[string]interface{}
}

// PreparedRequest holds the fully prepared reconciliation context built by Coordinator
// before the reconciler is invoked. Both typed and declarative reconcilers receive the
// same PreparedRequest — the difference is how they reconcile, not what they receive.
type PreparedRequest struct {
	// Object is the CR as *unstructured.Unstructured, normalized and enriched.
	// Always non-nil when Prepared is non-nil.
	Object Object

	// Context is the enriched resolver: profiles, functions, cross, intent.
	Context PreparedContext

	// Target is the resolved serve target name. Empty for the primary surface.
	Target string

	// Box is the effective OperatorBoxConfig for this reconciliation surface.
	// Use prepare.BoxFrom(req) to extract it with a type assertion.
	Box interface{}

	// Args are the Blueprint's reconcile.args, rendered against the CR for this
	// reconcile. Read them with Request.Args().
	Args Args

	// Finalizing is true when the CR is being deleted. Inrun's finalizer holds
	// it until the reconciler returns without a requeue. Read it with
	// Request.Finalizing().
	Finalizing bool
}
