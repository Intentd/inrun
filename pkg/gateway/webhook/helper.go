package webhook

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/pkg/types"
	admissionv1 "k8s.io/api/admissionregistration/v1"
)

var (
	setFieldPath   = structured.SetNestedPath
	convertToType  = types.ConvertToType
	readLocal      = fileio.ReadLocal
	scalarToString = types.ScalarToString
	resolveScalar  = types.ResolveScalarField
)

// ── Pointer helpers ───────────────────────────────────────────────────────────

func matchPolicyPtr(p admissionv1.MatchPolicyType) *admissionv1.MatchPolicyType       { return &p }
func failurePolicyPtr(p admissionv1.FailurePolicyType) *admissionv1.FailurePolicyType { return &p }
func reinvocationPolicyPtr(p admissionv1.ReinvocationPolicyType) *admissionv1.ReinvocationPolicyType {
	return &p
}

// admissionv1FailurePolicyType converts a string failure policy to the typed form.
// Unrecognised values default to Ignore.
func admissionv1FailurePolicyType(policy string) admissionv1.FailurePolicyType {
	switch strings.ToLower(policy) {
	case "fail":
		return admissionv1.Fail
	default:
		return admissionv1.Ignore
	}
}

// runtimeEndpoint builds the in-cluster URL for this Inrun instance's own
// runtime — the same service the gateway is deployed alongside, not an
// operator-declared cross: endpoint. Port comes from config's INRUN_PORT
// (default "8080") — the runtime and gateway both read the same env var
// convention for their own /blueprint server, so the gateway's own configured
// value is the runtime's too.
func (ws *WebhookServer) runtimeEndpoint() string {
	svc := ws.blueprint.RuntimeServiceName()
	ns := ws.config.Cluster().Namespace()
	port := ws.config.Health().Port()
	return fmt.Sprintf("http://%s.%s.svc:%s", svc, ns, port)
}
