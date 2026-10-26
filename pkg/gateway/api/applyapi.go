package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// applyHandler returns the http.HandlerFunc for POST /api/v1/apply.
// The auth middleware must wrap this handler before registration.
func applyHandler(kube kubeclient.Interface, clusters *ClusterRegistry, bp *blueprint.Blueprint) http.HandlerFunc {
	var notes types.KELFunctionRegistry
	if !bp.Empty() {
		notes = bp.UserFunctions()
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed", "only POST requests are supported")
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApplyResponse{
				Message: "failed to read request body",
			})
			return
		}

		raw, err := intent.Decode(body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApplyResponse{
				Message: err.Error(),
			})
			return
		}

		req := applyRequest{
			dryRun:    r.URL.Query().Get("dryRun") == "true",
			overwrite: r.URL.Query().Get("overwrite") == "true", // SSA forceConflict
			override:  r.URL.Query().Get("override") == "true",  // TargetConflict
		}

		prepared, ok := prepareApply(w, r, raw, kube, bp, notes)
		if !ok {
			return
		}

		executeApply(w, r, req, prepared, kube, clusters, notes)
	}
}

type applyRequest struct {
	dryRun    bool
	overwrite bool
	override  bool
}

type preparedApply struct {
	obj       *unstructured.Unstructured
	crd       *types.CRDEntry
	alias     string
	gvr       schema.GroupVersionResource
	patchBody []byte
	raw       map[string]interface{}
}

func prepareApply(
	w http.ResponseWriter,
	r *http.Request,
	raw map[string]interface{},
	kube kubeclient.Interface,
	bp *blueprint.Blueprint,
	notes types.KELFunctionRegistry,
) (*preparedApply, bool) {
	p := &preparedApply{raw: raw}

	if isTargetRequest(raw) {
		target, _ := raw["target"].(string)
		if strings.TrimSpace(target) == "" {
			writeJSON(w, http.StatusBadRequest, ApplyResponse{
				Message: `"target" must be a non-empty string`,
			})
			return nil, false
		}

		resolution := bp.LookupByTargetOrAlias(target)
		if resolution == nil {
			writeJSON(w, http.StatusBadRequest, ApplyResponse{
				Message: fmt.Sprintf(
					"unknown target %q — available: %s",
					target,
					strings.Join(bp.AvailableTargets(), ", "),
				),
			})
			return nil, false
		}

		p.crd = resolution.CRD
		p.alias = resolution.Alias
		gvk := p.crd.GVK()

		// Preserve the existing response/continuation behavior.
		if _, err := crdExists(kube.RestConfig(), &gvk); err != nil {
			writeJSONError(w, http.StatusBadRequest, "CRD not installed",
				fmt.Sprintf("CRD not installed for '%q'", target),
			)
		}

		if !p.crd.TargetModeEnabledFor(p.alias) {
			writeJSONError(w, http.StatusBadRequest, "target mode not enabled",
				fmt.Sprintf("Target mode is not enabled for '%q'", target),
			)
			return nil, false
		}

		built, err := intent.BuildCR(raw, p.crd, notes)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApplyResponse{
				Message: err.Error(),
			})
			return nil, false
		}

		p.obj = built
		InjectProvenanceAnnotations(p.obj, p.crd.ServeTarget(), p.alias)
		InjectServeIntentAnnotation(p.obj, raw)

		// Preserve the original target-mode marshal and error response.
		p.patchBody, err = json.Marshal(p.obj.Object)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApplyResponse{
				Message: "failed to marshal built CR",
			})
			return nil, false
		}
	} else {
		full := unstructured.Unstructured{Object: raw}

		if full.GetKind() == "" || full.GetAPIVersion() == "" {
			writeJSON(w, http.StatusBadRequest, ApplyResponse{
				Message: `request must include either "target" (target mode) or "apiVersion" and "kind" (full CR mode)`,
			})
			return nil, false
		}

		p.crd = bp.LookupByKind(full.GetKind()).Entry()
		if p.crd == nil {
			writeJSON(w, http.StatusBadRequest, ApplyResponse{
				Message: fmt.Sprintf("unknown kind %q", full.GetKind()),
			})
			return nil, false
		}

		gvk := p.crd.GVK()
		// Preserve the existing response/continuation behavior.
		if _, err := crdExists(kube.RestConfig(), &gvk); err != nil {
			writeJSONError(w, http.StatusBadRequest, "CRD not installed",
				fmt.Sprintf("CRD not installed for '%q'", p.crd.Kind()),
			)
		}

		if p.crd.HasMigrate() {
			dstGVK := p.crd.Migrate.GVK()
			if gvk != dstGVK {
				writeJSONError(w, http.StatusBadRequest, "source CRD under migration",
					fmt.Sprintf(
						"kind %q is being migrated; submit the destination kind %q with apiVersion %q",
						full.GetKind(), dstGVK.Kind, dstGVK.GroupVersion().String(),
					),
				)
				return nil, false
			}
		}

		if !p.crd.ServeEnabled() {
			writeJSONError(w, http.StatusBadRequest, "serve not enabled",
				fmt.Sprintf("Serve is not enabled for kind %q", full.GetKind()),
			)
			return nil, false
		}

		matchedTarget := ""
		if target := p.crd.EffectiveServeTargetForMap(full.Object); target != "" {
			matchedTarget = target
		}
		if matchedTarget != "" {
			logger.FromContext(r.Context()).Debug().
				Str("kind", full.GetKind()).
				Str("name", full.GetName()).
				Str("namespace", full.GetNamespace()).
				Str("target", matchedTarget).
				Msg("matched target")
		}

		effectiveTarget := matchedTarget
		if effectiveTarget == "" {
			effectiveTarget = p.crd.ServeTarget()
		}

		if !p.crd.FullCRModeEnabledFor(effectiveTarget) {
			writeJSONError(w, http.StatusBadRequest, "CR mode not enabled",
				fmt.Sprintf("Full CR mode is not enabled for %q", effectiveTarget),
			)
			return nil, false
		}

		p.obj = &full

		InjectProvenanceAnnotations(p.obj, p.crd.ServeTarget(), matchedTarget)
		if matchedTarget != "" {
			InjectFieldSelectorAnnotations(p.obj, matchedTarget, p.crd.FieldSelectorForTarget(matchedTarget))
		}

		if err := resolveServeMeta(p.obj, p.crd, notes); err != nil {
			writeJSON(w, http.StatusBadRequest, ApplyResponse{
				Message: err.Error(),
			})
			return nil, false
		}
	}

	patchBody, err := json.Marshal(p.obj.Object)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ApplyResponse{
			Message: fmt.Sprintf("failed to marshal CR with annotations: %v", err),
		})
		return nil, false
	}
	p.patchBody = patchBody

	p.gvr = p.crd.GVR()
	if p.crd.HasMigrate() {
		p.gvr = p.crd.Migrate.GVR()
	}

	// Only override alias when a fieldSelector explicitly matched.
	if effectiveTarget := p.crd.ServeTargetForFieldSelector(p.obj.Object); effectiveTarget != "" {
		p.alias = effectiveTarget
	}

	return p, true
}

func executeApply(
	w http.ResponseWriter,
	r *http.Request,
	req applyRequest,
	p *preparedApply,
	kube kubeclient.Interface,
	clusters *ClusterRegistry,
	notes types.KELFunctionRegistry,
) {
	targets, err := resolveClusterTargets(p.crd, p.alias, p.raw, notes, clusters, kube)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ApplyResponse{Message: err.Error()})
		return
	}

	forceConflictEnabled := p.crd.ServeForceConflictEnabledFor(p.alias)
	if req.overwrite && !forceConflictEnabled {
		writeJSONError(w, http.StatusBadRequest, "forceConflict not enabled",
			fmt.Sprintf("Force conflict is not enabled for %q", p.alias),
		)
		return
	}
	if !req.overwrite && forceConflictEnabled {
		req.overwrite = true
	}

	if p.obj.GetName() == "" {
		writeJSON(w, http.StatusUnprocessableEntity, ApplyResponse{
			DryRun:  req.dryRun,
			Message: "name is required",
			Violations: []ApplyViolation{{
				Field:    "metadata.name",
				Message:  "name is required",
				Severity: "error",
			}},
		})
		return
	}

	if p.obj.GetNamespace() == "" {
		writeJSON(w, http.StatusUnprocessableEntity, ApplyResponse{
			DryRun:  req.dryRun,
			Message: "namespace is required",
			Violations: []ApplyViolation{{
				Field:    "metadata.namespace",
				Message:  "namespace is required",
				Severity: "error",
			}},
		})
		return
	}

	patchOpts := metav1.PatchOptions{
		FieldManager: config.FieldManagerGateway,
		Force:        ptr.To(req.overwrite),
	}
	if req.dryRun {
		patchOpts.DryRun = []string{metav1.DryRunAll}
	}

	tokenName := TokenNameFromContext(r.Context())
	payload := EvaluatePayload(p.obj.Object, p.crd, p.alias, notes)

	if len(targets) == 1 && targets[0].name == "" {
		applySingleCluster(w, r, req, p, targets[0], patchOpts, tokenName, payload, notes)
		return
	}

	applyFanout(w, r, req, p, targets, patchOpts, tokenName, payload, notes)
}

func applySingleCluster(
	w http.ResponseWriter,
	r *http.Request,
	req applyRequest,
	p *preparedApply,
	target clusterTarget,
	patchOpts metav1.PatchOptions,
	tokenName string,
	payload map[string]interface{},
	notes types.KELFunctionRegistry,
) {
	effectiveKube := target.kube

	if p.crd != nil {
		op := types.ServeOpCreate
		existing, probeErr := effectiveKube.DynamicClient().
			Resource(p.gvr).
			Namespace(p.obj.GetNamespace()).
			Get(r.Context(), p.obj.GetName(), metav1.GetOptions{})
		if probeErr == nil {
			op = types.ServeOpUpdate
			ann := existing.GetAnnotations()
			storedSurface := ann[labels.AnnotationServeAlias]
			if storedSurface == "" {
				storedSurface = ann[labels.AnnotationServeTarget]
			}
			incomingSurface := p.alias
			if incomingSurface == "" {
				incomingSurface = p.crd.ServeTarget()
			}
			if storedSurface != "" && storedSurface != incomingSurface {
				if !req.override {
					msg := fmt.Sprintf(
						"routing surface conflict: resource was created via %q, cannot update via %q without ?override=true",
						storedSurface, incomingSurface,
					)
					writeJSON(w, http.StatusConflict, ApplyResponse{
						Message: msg,
						Violations: []ApplyViolation{{
							Field:    "metadata.annotations",
							Message:  msg,
							Severity: "error",
						}},
					})
					return
				}

				if req.override {
					if !p.crd.ServeTargetOverrideEnabledFor(storedSurface) {
						writeJSONError(w, http.StatusBadRequest, "target override not enabled",
							fmt.Sprintf("Target override is not enabled for %q", storedSurface),
						)
						return
					}
				}

				logger.FromContext(r.Context()).Warn().
					Str("from", storedSurface).
					Str("to", incomingSurface).
					Str("name", p.obj.GetName()).
					Str("namespace", p.obj.GetNamespace()).
					Msg("routing surface override")
			}
		}

		allowed, reason := p.crd.TokenAllowedFor(
			p.alias, tokenName, op, p.obj.GetNamespace(), types.ServeClassResources,
		)
		if !allowed {
			msg := reason.Message(tokenName, op, p.obj.GetKind(), p.obj.GetNamespace())
			writeJSON(w, http.StatusForbidden, ApplyResponse{
				Message: msg,
				Violations: []ApplyViolation{{
					Field:    "metadata",
					Message:  msg,
					Severity: "error",
				}},
			})
			return
		}
	}

	ns := p.obj.GetNamespace()
	capture := &warningCapture{}
	result, err := scopedDynamic(effectiveKube, capture).
		Resource(p.gvr).
		Namespace(ns).
		Patch(r.Context(), p.obj.GetName(), k8stypes.ApplyPatchType, p.patchBody, patchOpts)
	if err != nil {
		violations := extractViolations(err)
		message := err.Error()
		if len(violations) > 0 && violations[0].Message != "" {
			message = violations[0].Message
		}
		logger.FromContext(r.Context()).Warn().
			Str("kind", p.obj.GetKind()).
			Str("name", p.obj.GetName()).
			Bool("dryRun", req.dryRun).
			Err(err).
			Msg("gateway API: SSA rejected")
		writeJSON(w, http.StatusUnprocessableEntity, ApplyResponse{
			DryRun:     req.dryRun,
			Message:    message,
			Warnings:   capture.collect(),
			Violations: violations,
		})
		return
	}

	resolver := template.NewResolverFromMap(p.raw).WithUserFunctions(notes)
	pollURL := resolvePollURL(
		result.GetKind(), result.GetNamespace(), result.GetName(),
		p.crd.GetServePollingConfig(), resolver,
	)
	writeJSON(w, http.StatusOK, ApplyResponse{
		Accepted:   true,
		DryRun:     req.dryRun,
		Name:       result.GetName(),
		Namespace:  result.GetNamespace(),
		Kind:       result.GetKind(),
		APIVersion: result.GetAPIVersion(),
		PollURL:    pollURL,
		Warnings:   capture.collect(),
		Payload:    payload,
	})
}

func applyFanout(
	w http.ResponseWriter,
	r *http.Request,
	req applyRequest,
	p *preparedApply,
	targets []clusterTarget,
	patchOpts metav1.PatchOptions,
	tokenName string,
	payload map[string]interface{},
	notes types.KELFunctionRegistry,
) {
	obj := p.obj
	crd := p.crd
	gvr := p.gvr
	patchBody := p.patchBody
	alias := p.alias

	incomingSurface := alias
	if incomingSurface == "" && crd != nil {
		incomingSurface = crd.ServeTarget()
	}
	resolver := template.NewResolverFromMap(p.raw).WithUserFunctions(notes)

	var clusterResults []ClusterApplyResult
	for _, t := range targets {
		cr := ClusterApplyResult{Cluster: t.name}

		if crd != nil {
			op := types.ServeOpCreate
			existing, probeErr := t.kube.DynamicClient().
				Resource(gvr).
				Namespace(obj.GetNamespace()).
				Get(r.Context(), obj.GetName(), metav1.GetOptions{})
			if probeErr == nil {
				op = types.ServeOpUpdate
				ann := existing.GetAnnotations()
				storedSurface := ann[labels.AnnotationServeAlias]
				if storedSurface == "" {
					storedSurface = ann[labels.AnnotationServeTarget]
				}
				if storedSurface != "" && storedSurface != incomingSurface && !req.override {
					cr.Message = fmt.Sprintf(
						"routing surface conflict: resource was created via %q, cannot update via %q without ?override=true",
						storedSurface, incomingSurface,
					)
					clusterResults = append(clusterResults, cr)
					continue
				}
			}

			allowed, reason := crd.TokenAllowedFor(alias, tokenName, op, obj.GetNamespace(), types.ServeClassResources)
			if !allowed {
				cr.Message = reason.Message(tokenName, op, obj.GetKind(), obj.GetNamespace())
				clusterResults = append(clusterResults, cr)
				continue
			}
		}

		capture := &warningCapture{}
		result, err := scopedDynamic(t.kube, capture).
			Resource(gvr).
			Namespace(obj.GetNamespace()).
			Patch(r.Context(), obj.GetName(), k8stypes.ApplyPatchType, patchBody, patchOpts)
		if err != nil {
			violations := extractViolations(err)
			message := err.Error()
			if len(violations) > 0 && violations[0].Message != "" {
				message = violations[0].Message
			}
			logger.FromContext(r.Context()).Warn().
				Str("cluster", t.name).
				Str("kind", obj.GetKind()).
				Str("name", obj.GetName()).
				Err(err).
				Msg("gateway API: SSA rejected (fan-out)")
			cr.Message = message
			cr.Warnings = capture.collect()
			cr.Violations = violations
			clusterResults = append(clusterResults, cr)
			continue
		}

		cr.Accepted = true
		cr.Name = result.GetName()
		cr.Namespace = result.GetNamespace()
		cr.Warnings = capture.collect()
		cr.PollURL = resolvePollURL(
			result.GetKind(),
			result.GetNamespace(),
			result.GetName(),
			crd.GetServePollingConfig(),
			resolver,
		)
		clusterResults = append(clusterResults, cr)
	}

	var anyAccepted, allAccepted bool
	allAccepted = true
	for _, cr := range clusterResults {
		if cr.Accepted {
			anyAccepted = true
		} else {
			allAccepted = false
		}
	}

	writeJSON(w, http.StatusOK, ApplyResponse{
		Accepted:       anyAccepted,
		DryRun:         req.dryRun,
		Kind:           obj.GetKind(),
		APIVersion:     obj.GetAPIVersion(),
		PartialSuccess: anyAccepted && !allAccepted,
		Clusters:       clusterResults,
		Payload:        payload,
	})
}
