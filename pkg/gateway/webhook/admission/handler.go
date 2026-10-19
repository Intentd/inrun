package admission

import (
	"fmt"

	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/kube"
	"github.com/intentd/inrun/pkg/util/structured"
)

var (
	setFieldPath   = structured.SetNestedPath
	convertToType  = types.ConvertToType
	scalarToString = types.ScalarToString
	resolveScalar  = types.ResolveScalarField
)

// Handler serves /validate and /mutate and counts their decisions per CRD.
type Handler struct {
	bp       *blueprint.Blueprint
	cfg      *config.Config
	client   kubernetes.Interface // for the rules' external calls
	registry blueprint.AdmissionRegistry
	stats    *health.StatsMap[health.AdmissionStats]
}

// New builds the Handler for the Blueprint's admission rules. window is how
// many recent requests the latency stats cover.
func New(client kubernetes.Interface, bp *blueprint.Blueprint, cfg *config.Config, window int) *Handler {
	h := &Handler{
		bp:       bp,
		cfg:      cfg,
		client:   client,
		registry: bp.AdmissionRegistry(),
		stats: health.NewStatsMap(func() *health.AdmissionStats {
			return health.NewAdmissionStats(window)
		}),
	}
	// Enabled, not Spec.CRDs: only enabled entries have their GVR set.
	for _, crd := range bp.Enabled() {
		gvr := crd.GVR()
		h.stats.For(kube.GVRKey(gvr.Group, gvr.Version, gvr.Resource))
	}
	return h
}

// StatsFor returns the admission stats for gvrKey, or nil.
func (h *Handler) StatsFor(gvrKey string) *health.AdmissionStats {
	return h.stats.Get(gvrKey)
}

// runtimeEndpoint is the in-cluster URL of the runtime deployed next to this
// gateway, which serves the rules' health and metrics lookups. Both read the
// same port setting.
func (h *Handler) runtimeEndpoint() string {
	svc := h.bp.RuntimeServiceName()
	ns := h.cfg.Cluster().Namespace()
	port := h.cfg.Health().Port()
	return fmt.Sprintf("http://%s.%s.svc:%s", svc, ns, port)
}
