package prepare

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type dynamicClientProvider interface {
	DynamicClient() dynamic.Interface
}

type liveUniquenessChecker struct {
	ctx        context.Context
	kube       dynamicClientProvider
	gvr        schema.GroupVersionResource
	namespaced bool
}

func newUniquenessChecker(ctx context.Context, kube dynamicClientProvider, gvr schema.GroupVersionResource, namespaced bool) types.UniquenessChecker {
	return &liveUniquenessChecker{ctx: ctx, kube: kube, gvr: gvr, namespaced: namespaced}
}

func (u *liveUniquenessChecker) IsUnique(field, value, selfNamespace, selfName string) (bool, error) {
	namespaceable := u.kube.DynamicClient().Resource(u.gvr)

	var resource dynamic.ResourceInterface = namespaceable
	if u.namespaced {
		resource = namespaceable.Namespace(metav1.NamespaceAll)
	}

	list, err := resource.List(u.ctx, metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("listing %s for uniqueness check: %w", u.gvr.Resource, err)
	}

	for _, item := range list.Items {
		if item.GetNamespace() == selfNamespace && item.GetName() == selfName {
			continue
		}
		val, found := types.ResolveScalarField(item.Object, field)
		if found && val == value {
			return false, nil
		}
	}
	return true, nil
}
