package constructor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/intentd/inrun-args-constructor/api/v1alpha1"
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/resources/core/deployments"
)

// BlockchainNodeReconciler implements domain.Reconciler for the BlockchainNode CRD.
type BlockchainNodeReconciler struct {
	kube kubeclient.Interface
}

// NewBlockchainNodeReconciler is the constructor function registered in the Blueprint.
func NewBlockchainNodeReconciler(kube kubeclient.Interface) domain.Reconciler {
	return &BlockchainNodeReconciler{kube: kube}
}

func (r *BlockchainNodeReconciler) Reconcile(ctx context.Context, req domain.Request) (domain.Result, error) {
	if req.Prepared == nil {
		return domain.Result{}, nil
	}

	node, err := domain.ToTyped[apiv1.BlockchainNode](req.Prepared)
	if err != nil {
		return domain.Result{}, fmt.Errorf("toTyped: %w", err)
	}

	if node.DeletionTimestamp != nil {
		return domain.Result{}, nil
	}

	// The Blueprint declares the flag URL pattern and the business-hours window.
	// The constructor owns the time check and the HTTP call — the Blueprint owns
	// the configuration. The binary is identical across environments and schedules.
	flagUrl := req.Args().String("flagUrl")
	start := req.Args().String("businessHoursStart")
	end := req.Args().String("businessHoursEnd")

	bizHours := r.inBusinessHours(start, end)
	featureEnabled := bizHours && r.checkFlag(ctx, flagUrl)

	annotation := "false"
	if featureEnabled {
		annotation = "true"
	}

	replicas := int32(node.Spec.Replicas)
	if replicas == 0 {
		replicas = 1
	}

	spec := deployments.Spec{
		Name:      node.Name,
		Namespace: node.Namespace,
		Image:     node.Spec.Image,
		Replicas:  replicas,
		Annotations: map[string]string{
			"feature.demo/v2-enabled": annotation,
		},
	}
	if _, err := deployments.Apply(ctx, r.kube, node, spec); err != nil {
		return domain.Result{}, fmt.Errorf("blockchainnode deployment: %w", err)
	}

	return domain.Result{}, r.kube.PatchStatus(ctx, node, map[string]any{
		"phase":           "Running",
		"network":         node.Spec.Network,
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
