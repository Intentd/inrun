package prepare

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"github.com/intentd/inrun/pkg/types"
)

// Input holds everything Prepare needs from the Controller.
type Input struct {
	Entry    contract.RegistryEntry
	Key      string
	Cat      *catalog.Catalog
	Kube     kubeclient.Interface
	Registry CatalogRegistry
}

// Prepare builds the PreparedRequest for a single reconcile cycle.
//
// Steps:
//  1. Object from cache
//  2. GVK fix
//  3. Namespace guard — returns (nil, nil, nil) for restricted/non-allowed namespaces
//  4. Effective box + target
//  5. Normalize + base resolver
//  6. Enrichment chain (profiles, notes, intent, uniqueness), then
//     reconcile.when/or — returns (nil, nil, nil) when they do not hold —
//     then reconcile.external
//  7. Cross-CRD observation
//  8. Mutation (if mutateFirst)
//  9. Validation / admission
//  10. Mutation (if !mutateFirst)
//
// Returns (nil, nil, nil) when the object is not in cache (deleted between dequeue
// and preparation), when the namespace guard blocks it, or when reconcile.when/or
// does not hold.
// Returns (nil, nil, err) on failure — caller should requeue.
func Prepare(ctx context.Context, in Input) (*domain.PreparedRequest, *ValidationResult, error) {
	if in.Entry.Informer == nil {
		return nil, nil, nil
	}

	// Step 1: object from cache
	raw, exists, err := in.Entry.Informer.GetIndexer().GetByKey(in.Key)
	if err != nil || !exists || raw == nil {
		return nil, nil, nil
	}
	obj, ok := domain.ToUnstructured(raw)
	if !ok {
		return nil, nil, fmt.Errorf("prepare: unexpected type %T in informer cache for %q", raw, in.Key)
	}

	// Step 2: GVK fix — watch events sometimes omit TypeMeta on first reconcile
	if obj.GetObjectKind().GroupVersionKind().Empty() {
		obj.GetObjectKind().SetGroupVersionKind(in.Entry.CRD.GVK())
	}

	crd := in.Entry.CRD

	// Step 3: namespace guard — skip reconcile for restricted/non-allowed namespaces.
	// Deletion is always permitted so finalizers can be removed;
	// this guard runs only for non-deleting CRs.
	if crd.HasNamespaceRules() && obj.GetDeletionTimestamp() == nil {
		result := CheckNamespace(ctx, obj, obj.GetNamespace(), crd.AllRestrictedNamespaces(), crd.AllAllowedNamespaces(), crd.APITypes.Kind)
		if !result.Allowed {
			return nil, nil, nil
		}
	}

	// Step 4: effective box and target
	box, target := effectiveBoxAndTarget(crd, obj)

	// Step 5: normalize + base resolver
	normalized, resolver, normalizeChanges, err := applyNormalize(ctx, crd, obj)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare: %w", err)
	}
	if len(normalizeChanges) > 0 {
		resolver = resolver.WithNormalizeChanges(normalizeChanges)
	}

	// Step 6: enrichment chain
	if in.Cat != nil {
		if !in.Cat.Profiles.Empty() {
			resolver = resolver.WithProfiles(&in.Cat.Profiles)
		}
		if !in.Cat.Notes.Empty() {
			resolver = resolver.WithUserNotes(in.Cat.UserNotes())
		}
	}
	if req := intent.FromObject(resolver.Data()); req != nil {
		resolver = resolver.WithIntent(req)
	}
	if in.Kube != nil {
		resolver = resolver.WithUniquenessChecker(
			newUniquenessChecker(ctx, in.Kube, crd.GVR(), crd.IsNamespaced()),
		)
	}

	// reconcile.when/or: should this reconcile happen at all? Evaluated before
	// the reconcile's calls, over the CR, notes, profiles, intent and .args
	// (validation keeps .external, and args that read it, out of them). A CR
	// being deleted is always reconciled.
	if box.Reconcile.HasConditions() && obj.GetDeletionTimestamp() == nil {
		met, reason := resolver.WithArgs(renderArgs(box, resolver)).EvaluateConditions(box.Reconcile.When, box.Reconcile.Or)
		if !met {
			logger.FromContext(ctx).Info().Str("key", in.Key).Str("reason", reason).
				Msg("reconcile.when not met: reconcile skipped")
			return nil, nil, nil
		}
	}

	// reconcile.external calls — run here, before any reconciler, so results
	// are available as .external.<name>.* when reconcile.args are rendered
	// (see Args). The effective box carries per-target overrides.
	if box.Reconcile != nil && len(box.Reconcile.External) > 0 {
		var extErr error
		resolver, extErr = external.Run(ctx, crd.GVKString(), resolver, box.Reconcile.External, in.Kube.Clientset())
		if extErr != nil {
			return nil, nil, fmt.Errorf("prepare: reconcile.external: %w", extErr)
		}
	}

	// Step 7: cross-CRD observation
	if len(box.EffectiveCross()) > 0 && in.Registry != nil {
		cs := in.Kube.Clientset()
		crossData := readCross(ctx, obj, box.EffectiveCross(), resolver, in.Registry, cs)
		if len(crossData) > 0 {
			resolver = resolver.WithCross(crossData)
		}
	}

	// Steps 8–10: mutation / validation / admission
	// Order respects MutationConfig.MutateFirst:
	//   true (default) — mutate → validate → reconcile
	//   false          — validate → mutate → reconcile
	if crd.HasMutationRules() && crd.ShouldMutateFirst() {
		if resolver, err = applyMutation(ctx, in.Kube, normalized, resolver, crd); err != nil {
			// Mutation failures are non-fatal — log and continue
			_ = err
		}
	}

	var valResult *ValidationResult
	if crd.HasValidationRules() {
		var valErr error
		resolver, valResult, valErr = applyValidation(ctx, in.Kube, obj, resolver, crd)
		if valErr != nil {
			return nil, valResult, valErr // denial carries valResult for status write
		}
	}

	if crd.HasMutationRules() && !crd.ShouldMutateFirst() {
		if resolver, err = applyMutation(ctx, in.Kube, normalized, resolver, crd); err != nil {
			_ = err
		}
	}

	return &domain.PreparedRequest{
		Object:     normalized,
		Context:    resolver,
		Target:     target,
		Box:        box,
		Finalizing: obj.GetDeletionTimestamp() != nil,
	}, valResult, nil
}

// BoxFrom extracts the OperatorBoxConfig from a PreparedRequest.
// Only Coordinator constructs PreparedRequest — any other Box type is a programming error.
func BoxFrom(req *domain.PreparedRequest) types.OperatorBoxConfig {
	if req == nil {
		return types.OperatorBoxConfig{}
	}
	box, ok := req.Box.(types.OperatorBoxConfig)
	if !ok {
		panic(fmt.Sprintf("prepare.BoxFrom: unexpected Box type %T", req.Box))
	}
	return box
}
