package maintain

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
)

// applyFinalizers adds or removes finalizers depending on CRDEntry.RemoveFinalizers.
//
// Normal operation: ensure all effective finalizers are present.
// Force-cleanup mode (RemoveFinalizers=true): strip all effective finalizers from the object.
func applyFinalizers(
	ctx context.Context,
	in Input,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	removeFinalizers := in.CRD.OperatorBox.EffectiveRemoveFinalizers()
	if r := box.Runtime; r != nil && r.RemoveFinalizers {
		removeFinalizers = true
	}
	if removeFinalizers {
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
	if len(obj.GetFinalizers()) == 0 {
		return nil
	}

	if !labels.StripFinalizers(obj, box.EffectiveFinalizers()) {
		return nil
	}

	logger.Debug().
		Str("name", obj.GetName()).
		Msgf("removing finalizers → %v", obj.GetFinalizers())

	return in.Kube.PatchFinalizers(ctx, obj, obj.GetFinalizers(), metav1.PatchOptions{})
}
