package apply

import (
	"context"
	"fmt"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/kubeclient"
	"github.com/inrundev/inrun/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Metadata sets the labels and annotations a Result returns on the CR itself,
// with server-side apply under FieldManager. The full set is applied every
// time, so a key dropped from the Result is removed and keys owned by anyone
// else are left alone.
//
// Nothing is sent when the Result sets none and the field manager owns
// nothing on the object yet, so CRs whose reconciler never uses this cost no
// extra call.
func Metadata(ctx context.Context, kube kubeclient.Interface, obj domain.Object, lbls, annotations map[string]string) error {
	if len(lbls) == 0 && len(annotations) == 0 && !managesMetadata(obj) {
		return nil
	}
	if err := labels.CheckReserved("labels", lbls); err != nil {
		return err
	}
	if err := labels.CheckReserved("annotations", annotations); err != nil {
		return err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		return fmt.Errorf("object %s/%s has no apiVersion/kind", obj.GetNamespace(), obj.GetName())
	}

	u := enrichedObjFor(gvk, obj, metadata{Labels: lbls, Annotations: annotations})
	res, err := kubeclient.ResourceFor(kube, u)
	if err != nil {
		return fmt.Errorf("resolve resource: %w", err)
	}
	// The reconciler's value wins over another manager's for the same key.
	if _, err := res.Apply(ctx, u.GetName(), u, metav1.ApplyOptions{
		FieldManager: FieldManager,
		Force:        true,
	}); err != nil {
		return fmt.Errorf("apply labels and annotations: %w", err)
	}
	return nil
}
