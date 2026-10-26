package registration

import (
	"fmt"
	"strings"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/util/fileio"
)

// The webhook configurations Inrun owns.
const (
	ValidatingConfigName          = "inrun-admission-validation"
	MutatingConfigName            = "inrun-admission-mutation"
	DeletionProtectionConfigName  = "inrun-deletion-protection"
	NamespaceProtectionConfigName = "inrun-namespace-protection"
	StrictModeConfigName          = "inrun-strict-mode-protection"
)

// CallerHousekeeper marks Options from the housekeeper, which applies the
// configurations on every sync and logs them at debug level.
const CallerHousekeeper = "housekeeper"

const (
	registrar = "registrar"

	maxAttempts          = 5
	delayBetweenAttempts = 5 * time.Second
	gracePeriodSeconds   = int64(30)
)

// Options says where the API server sends webhook requests.
type Options struct {
	Caller              string
	ServiceName         string
	ServiceNamespace    string
	Port                int32
	FailurePolicy       admissionv1.FailurePolicyType
	TLSCertFile         string
	InrunResourceLabels map[string]string
}

// NewOptions resolves Options from the Blueprint and config: the gateway's
// service, the HTTPS port and the certificate the configurations trust.
func NewOptions(bp *blueprint.Blueprint, cfg *config.Config) Options {
	return Options{
		FailurePolicy:       FailurePolicy(bp.WebhooksFailurePolicy()),
		Port:                cfg.HTTPSPortInt32(),
		ServiceName:         bp.GatewayServiceName(),
		ServiceNamespace:    cfg.Cluster().Namespace(),
		TLSCertFile:         cfg.Security().Webhooks.TLSCert,
		InrunResourceLabels: labels.InrunResourceLabels(),
	}
}

// CleanupOptions selects which admission configurations to remove.
type CleanupOptions struct {
	Mutating   bool
	Validating bool
}

// CleanupAll removes both admission configurations.
func CleanupAll() CleanupOptions {
	return CleanupOptions{Mutating: true, Validating: true}
}

// ReadCABundle reads the TLS certificate the configurations trust.
func ReadCABundle(certFile string) ([]byte, error) {
	if certFile == "" {
		return nil, fmt.Errorf("TLS_CERT is required for webhook registration")
	}
	data, err := fileio.ReadLocal(certFile)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", certFile, err)
	}
	return data, nil
}

// FailurePolicy converts a Blueprint failure policy to the typed form.
// Anything but "fail" is Ignore.
func FailurePolicy(policy string) admissionv1.FailurePolicyType {
	if strings.EqualFold(policy, "fail") {
		return admissionv1.Fail
	}
	return admissionv1.Ignore
}

// buildRules turns GVR entries into webhook rules.
func buildRules(gvrs []blueprint.GVREntry) []admissionv1.RuleWithOperations {
	rules := make([]admissionv1.RuleWithOperations, 0, len(gvrs))
	for _, gvr := range gvrs {
		ops := make([]admissionv1.OperationType, 0, len(gvr.Operations))
		for _, op := range gvr.Operations {
			ops = append(ops, admissionv1.OperationType(op))
		}
		rules = append(rules, admissionv1.RuleWithOperations{
			Operations: ops,
			Rule: admissionv1.Rule{
				APIGroups:   []string{gvr.Group},
				APIVersions: []string{gvr.Version},
				Resources:   []string{gvr.Resource},
			},
		})
	}
	return rules
}

// serviceRef points a webhook at path on the gateway's service.
func serviceRef(opts Options, path string) *admissionv1.ServiceReference {
	port := opts.Port
	return &admissionv1.ServiceReference{
		Name:      opts.ServiceName,
		Namespace: opts.ServiceNamespace,
		Path:      &path,
		Port:      &port,
	}
}
