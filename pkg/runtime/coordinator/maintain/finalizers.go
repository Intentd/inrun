package maintain

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
)

// applyFinalizers adds or removes finalizers depending on runtime.finalizers.remove.
//
// Normal operation: ensure all effective finalizers are present.
// Force-cleanup mode (remove: true): strip all effective finalizers from the object.
func applyFinalizers(
	ctx context.Context,
	in Input,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	if box.EffectiveRemoveFinalizers() {
		return stripFinalizers(ctx, in, obj, box)
	}
	return ensureFinalizers(ctx, in, obj, box)
}

func ensureFinalizers(
	ctx context.Context,
	in Input,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	finalizers := box.EffectiveFinalizers()
	if len(finalizers) == 0 {
		return nil
	}
	// The API server refuses new finalizers on an object being deleted.
	if obj.GetDeletionTimestamp() != nil {
		return nil
	}

	logger.Debug().
		Str("name", obj.GetName()).
		Any("crd finalizers", finalizers).
		Msgf("checking finalizers: %v", obj.GetFinalizers())

	if !labels.EnsureFinalizers(obj, finalizers) {
		return nil
	}

	logger.Debug().
		Str("name", obj.GetName()).
		Msgf("adding finalizers → %v", obj.GetFinalizers())

	if in.Recorder != nil {
		in.Recorder.Eventf(obj, corev1.EventTypeNormal, in.CRD.APITypes.Kind+"FinalizerAdded",
			fmt.Sprintf("Added finalizers to %s/%s", obj.GetNamespace(), obj.GetName()))
	}

	return in.Kube.PatchFinalizers(ctx, obj, obj.GetFinalizers(), metav1.PatchOptions{})
}

func stripFinalizers(
	ctx context.Context,
	in Input,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	return Finalize(ctx, in.Kube, obj, box)
}

// Finalize removes the box's finalizers (Inrun's cleanup finalizer and any the
// Blueprint declares) from obj, letting the API server finish deleting it. The
// coordinator calls it once a finalizing reconcile returns without a requeue.
func Finalize(ctx context.Context, kube kubeclient.Interface, obj domain.Object, box types.OperatorBoxConfig) error {
	if len(obj.GetFinalizers()) == 0 {
		return nil
	}
	if !labels.StripFinalizers(obj, box.EffectiveFinalizers()) {
		return nil
	}

	logger.Debug().
		Str("name", obj.GetName()).
		Msgf("removing finalizers → %v", strings.Join(obj.GetFinalizers(), ", "))

	// Already gone: an earlier pass released it, which is what we wanted.
	if err := kube.PatchFinalizers(ctx, obj, obj.GetFinalizers(), metav1.PatchOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
