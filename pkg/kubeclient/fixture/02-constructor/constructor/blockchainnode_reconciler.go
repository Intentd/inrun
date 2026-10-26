package constructor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/kubewright/core/deployments"
)

// BlockchainNodeReconciler implements domain.Reconciler for the BlockchainNode
// CRD from its args alone: the Blueprint maps the CR's spec into args, so the
// reconciler never reads the spec, the way a remote reconciler works.
type BlockchainNodeReconciler struct {
	kube kubeclient.Interface
}

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

// NewBlockchainNodeReconciler is the constructor function registered in the Blueprint.
func NewBlockchainNodeReconciler(kube kubeclient.Interface) domain.Reconciler {
	return &BlockchainNodeReconciler{kube: kube}
}

func (r *BlockchainNodeReconciler) Reconcile(ctx context.Context, req domain.Request) (domain.Result, error) {
	// Nothing to clean up when finalizing: the Deployment is owned by the CR.
	if req.Finalizing() {
		return domain.Result{}, nil
	}

	// The CR is the Deployment's owner and the status patch's target only.
	node := req.Object()
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

	spec := deployments.Spec{
		Name:      node.GetName(),
		Namespace: node.GetNamespace(),
		Image:     args.Image,
		Replicas:  replicas,
		Annotations: map[string]string{
			"feature.demo/v2-enabled": annotation,
		},
	}
	if _, err := deployments.Apply(ctx, r.kube.Clientset(), node, spec, kubeclient.ApplyOptions(r.kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("blockchainnode deployment: %w", err)
	}

	return domain.Result{}, r.kube.PatchStatus(ctx, node, map[string]any{
		"phase":           "Running",
		"network":         args.Network,
		"featureEnabled":  annotation,
		"inBusinessHours": bizHours,
	}, metav1.PatchOptions{})
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
