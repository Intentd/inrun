package kind

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

// writeStatus sets obj's status. With a status subresource, writes to the
// main object ignore status, so it goes through UpdateStatus; without one,
// UpdateStatus is NotFound and the main object is updated. Nothing is written
// when status is empty, already in place, or the run is a dry-run.
func (m *migrator) writeStatus(ctx context.Context, ri dynamic.ResourceInterface, obj *unstructured.Unstructured, status map[string]interface{}) error {
	if len(status) == 0 || m.DryRun || obj == nil {
		return nil
	}
	if reflect.DeepEqual(jsonNormalize(obj.Object["status"]), jsonNormalize(status)) {
		return nil
	}
	u := obj.DeepCopy()
	u.Object["status"] = runtime.DeepCopyJSONValue(status)
	_, err := ri.UpdateStatus(ctx, u, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		_, err = ri.Update(ctx, u, metav1.UpdateOptions{})
	}
	return err
}
