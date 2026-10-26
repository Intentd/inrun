package constructor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
)

// BlockchainNodeReconciler implements domain.Reconciler for the BlockchainNode
// CRD the way a remote reconciler does, in Go: args in, the resources and
// status it wants out. It never reads the CR's spec, calls no Kubernetes API
// and needs no client — Inrun builds the resources from their intent form,
// applies them with the CR as owner, and merges the status.
type BlockchainNodeReconciler struct{}

// nodeArgs is what the Blueprint's args give this reconciler: its own input
// type, not the CRD's. Rendered args arrive as native values, so replicas
// binds to a number.
type nodeArgs struct {
	Image              string `json:"image"`
	Replicas           int32  `json:"replicas"`
	Network            string `json:"network"`
	FlagURL            string `json:"flagUrl"`
	BusinessHoursStart string `json:"businessHoursStart"`
	BusinessHoursEnd   string `json:"businessHoursEnd"`
}

// NewBlockchainNodeReconciler is the constructor function registered in the
// Blueprint. The client is offered to every constructor; this one has no use
// for it.
func NewBlockchainNodeReconciler(_ kubeclient.Interface) domain.Reconciler {
	return &BlockchainNodeReconciler{}
}

func (r *BlockchainNodeReconciler) Reconcile(ctx context.Context, req domain.Request) (domain.Result, error) {
	// Nothing to clean up when finalizing: the Deployment is owned by the CR.
	if req.Finalizing() {
		return domain.Result{}, nil
	}

	var args nodeArgs
	if err := req.Args().Bind(&args); err != nil {
		return domain.Result{}, fmt.Errorf("blockchainnode args: %w", err)
	}

	// The Blueprint declares the flag URL pattern and the business-hours window.
	// The constructor owns the time check and the HTTP call — the Blueprint owns
	// the configuration. The binary is identical across environments and schedules.
	bizHours := r.inBusinessHours(args.BusinessHoursStart, args.BusinessHoursEnd)
	featureEnabled := bizHours && r.checkFlag(ctx, args.FlagURL)

	annotation := "false"
	if featureEnabled {
		annotation = "true"
	}

	replicas := args.Replicas
	if replicas == 0 {
		replicas = 1
	}

	// The Deployment in intent form, as a remote reconciler would answer:
	// Inrun builds it, applies it in the CR's namespace and owns it by the CR.
	return domain.Result{
		Resources: []map[string]interface{}{{
			"type": "deployment",
			"fields": map[string]interface{}{
				"name":     req.Name(),
				"image":    args.Image,
				"replicas": replicas,
				"annotations": map[string]interface{}{
					"feature.demo/v2-enabled": annotation,
				},
			},
		}},
		Status: map[string]interface{}{
			"phase":           "Running",
			"network":         args.Network,
			"featureEnabled":  annotation,
			"inBusinessHours": bizHours,
		},
	}, nil
}

// inBusinessHours returns true when the current UTC time is a weekday within
// the declared window. start and end are "HH:MM" strings from blueprint args.
func (r *BlockchainNodeReconciler) inBusinessHours(start, end string) bool {
	now := time.Now().UTC()
	if wd := now.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	cur := now.Format("15:04")
	return cur >= start && cur < end
}

// checkFlag calls the feature-flag endpoint and returns true when the body is "true".
func (r *BlockchainNodeReconciler) checkFlag(ctx context.Context, url string) bool {
	if url == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	return strings.TrimSpace(string(body)) == "true"
}
