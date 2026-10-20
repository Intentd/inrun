package runtimeapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/intentd/inrun/pkg/types"
	kubeutil "github.com/intentd/inrun/pkg/util/kube"
	"github.com/intentd/inrun/pkg/util/structured"
	"k8s.io/client-go/kubernetes"
)

const crossMetricHTTPTimeout = 5 * time.Second

// ResolveCrossToken resolves cross declaration token from either plan token or SecretRef.
// Used by autoscaler and reconciler packages currently
func ResolveCrossToken(ctx context.Context, cs kubernetes.Interface, s *types.CrossSource) (string, error) {
	if !s.HasAuth() {
		return "", nil
	}
	if s.HasSecretRef() {
		ref := s.Auth.SecretRef
		return kubeutil.ReadSecretKey(ctx, cs, ref.SecretNamespace(), ref.SecretName(), ref.SecretKey())
	}
	return structured.ResolveEnvVar(s.Auth.Token)

}

// FetchCrossMetricHTTP calls the remote operator's /blueprint/{crd} endpoint and
// extracts the named metric from the "metrics" key in the JSON response.
// This mirrors how readCross uses source.endpoint for CR observation.
// func FetchCrossMetricHTTP(ctx context.Context, cs kubernetes.Interface, source *types.CrossSource, endpoint string) (map[string]interface{}, bool)
// FetchCrossMetricHTTP calls the remote operator's /blueprint/{crd} endpoint and
// extracts the named metric from the "metrics" key in the JSON response.
// This mirrors how readCross uses source.endpoint for CR observation.

// fetchCrossViaHTTP fetches a CR's detail from an Inrun CR endpoint.
// The endpoint should be the /blueprint/{crd}/cr/{namespace}/{name} URL
// which is already built and running on every Inrun instance.
func FetchCrossViaHTTP(ctx context.Context, cs kubernetes.Interface, source *types.CrossSource) ([]byte, map[string]interface{}) {
	if ctx == context.TODO() {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, crossMetricHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, source.Endpoint, nil)
	if err != nil {
		return nil, nil
	}

	token, err := ResolveCrossToken(ctx, cs, source)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	if resp.StatusCode == http.StatusNotFound {
		return nil, map[string]interface{}{"found": "false"}
	}
	if code != http.StatusOK {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil
	}

	// Ensure "found" is set
	if _, ok := result["found"]; !ok {
		if result["name"] != nil {
			result["found"] = "true"
		} else {
			result["found"] = "false"
		}
	}

	return body, result
}
