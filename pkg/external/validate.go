package external

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

// ValidateCalls checks a list of ExternalCallSpecs for name uniqueness and semantic correctness.
// Called from pkg/catalog during inrun validate.
func ValidateCalls(crdName, location string, calls []types.ExternalCallSpec) error {
	seen := make(map[string]bool, len(calls))
	for i, call := range calls {
		loc := fmt.Sprintf("CRD %q: %s[%d]", crdName, location, i)
		if call.Name == "" {
			return fmt.Errorf("%s: name must not be empty", loc)
		}
		if seen[call.Name] {
			return fmt.Errorf("CRD %q: %s: duplicate call name %q — names must be unique", crdName, location, call.Name)
		}
		seen[call.Name] = true
		if err := ValidateCall(loc, call); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCall checks a single ExternalCallSpec for semantic correctness.
func ValidateCall(location string, call types.ExternalCallSpec) error {
	// protocol must be a known value
	if err := validateProtocol(location, call.Protocol); err != nil {
		return err
	}

	// url is always required
	if call.URL == "" {
		return fmt.Errorf("%s: url is required", location)
	}

	// protocol-specific field rules
	switch call.Protocol {
	case types.ProtocolHTTP, "":
		if err := validateHTTPCall(location, call); err != nil {
			return err
		}
	case types.ProtocolPrometheus:
		if call.Query == "" {
			return fmt.Errorf("%s: query is required for protocol: prometheus", location)
		}
		if call.Body != "" || call.Method != "" || len(call.Headers) > 0 || call.ExpectedStatus != 0 {
			return fmt.Errorf("%s: body, method, headers, and expectedStatus are HTTP-only fields", location)
		}
	case types.ProtocolRedis:
		if call.Query == "" {
			return fmt.Errorf("%s: query is required for protocol: redis (e.g. \"GET mykey\")", location)
		}
		if call.Body != "" || call.Method != "" || len(call.Headers) > 0 || call.ExpectedStatus != 0 {
			return fmt.Errorf("%s: body, method, headers, and expectedStatus are HTTP-only fields", location)
		}
	case types.ProtocolPostgres:
		if call.Query == "" {
			return fmt.Errorf("%s: query is required for protocol: postgres", location)
		}
		if call.Body != "" || call.Method != "" || len(call.Headers) > 0 || call.ExpectedStatus != 0 {
			return fmt.Errorf("%s: body, method, headers, and expectedStatus are HTTP-only fields", location)
		}
	case types.ProtocolMongo:
		if call.Query == "" {
			return fmt.Errorf("%s: query is required for protocol: mongo (e.g. \"mydb.mycollection\")", location)
		}
		if call.Body != "" || call.Method != "" || len(call.Headers) > 0 || call.ExpectedStatus != 0 {
			return fmt.Errorf("%s: body, method, headers, and expectedStatus are HTTP-only fields", location)
		}
	case types.ProtocolKafka:
		if call.Query == "" {
			return fmt.Errorf("%s: query is required for protocol: kafka (e.g. \"group/topic\" or \"@topic\")", location)
		}
		if call.Body != "" || call.Method != "" || len(call.Headers) > 0 || call.ExpectedStatus != 0 {
			return fmt.Errorf("%s: body, method, headers, and expectedStatus are HTTP-only fields", location)
		}
		// case types.ProtocolGRPC, types.ProtocolNATS, types.ProtocolMQTT:
		// 	if call.Body != "" || call.Method != "" || len(call.Headers) > 0 || call.ExpectedStatus != 0 {
		// 		return fmt.Errorf("%s: body, method, headers, and expectedStatus are HTTP-only fields", location)
		// 	}
	}

	// cacheFor must parse as a duration
	if call.CacheFor != "" {
		if _, err := utils.ParseTimeDuration(call.CacheFor); err != nil {
			return fmt.Errorf("%s: cacheFor %q is not a valid duration: %w", location, call.CacheFor, err)
		}
	}

	// timeout must parse as a duration
	if call.Timeout != "" {
		if _, err := utils.ParseTimeDuration(call.Timeout); err != nil {
			return fmt.Errorf("%s: timeout %q is not a valid duration: %w", location, call.Timeout, err)
		}
	}

	// auth: exactly one source required when auth is declared
	if call.Auth != nil {
		if err := validateAuth(location, call.Auth); err != nil {
			return err
		}
	}

	return nil
}

func validateProtocol(location string, p types.ExternalProtocol) error {
	switch p {
	case "", types.ProtocolHTTP, types.ProtocolPrometheus,
		types.ProtocolRedis, types.ProtocolPostgres,
		types.ProtocolMongo, types.ProtocolKafka:
		// types.ProtocolGRPC, types.ProtocolNATS, types.ProtocolMQTT — reserved for future support
		return nil
	}
	return fmt.Errorf("%s: unknown protocol %q — valid values: http, prometheus, redis, postgres, mongo, kafka", location, p)
}

func validateHTTPCall(location string, call types.ExternalCallSpec) error {
	if call.Method != "" {
		switch call.Method {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
		default:
			return fmt.Errorf("%s: method %q is not a valid HTTP method", location, call.Method)
		}
	}
	if call.Query != "" {
		return fmt.Errorf("%s: query is not used by protocol: http — use url: to encode query parameters", location)
	}
	if call.PoolSize != 0 {
		return fmt.Errorf("%s: poolSize is only meaningful for stateful protocols (redis, postgres, kafka)", location)
	}
	return nil
}

func validateAuth(location string, auth *types.ExternalAuth) error {
	hasSecretRef := auth.SecretRef != nil
	hasEnv := auth.Env != ""
	if !hasSecretRef && !hasEnv {
		return fmt.Errorf("%s: auth: exactly one of secretRef or env must be set", location)
	}
	if hasSecretRef && hasEnv {
		return fmt.Errorf("%s: auth: secretRef and env are mutually exclusive", location)
	}
	if hasSecretRef {
		if auth.SecretRef.Name == "" {
			return fmt.Errorf("%s: auth.secretRef.name is required", location)
		}
		if auth.SecretRef.Namespace == "" {
			return fmt.Errorf("%s: auth.secretRef.namespace is required", location)
		}
		if auth.SecretRef.Key == "" {
			return fmt.Errorf("%s: auth.secretRef.key is required", location)
		}
	}
	return nil
}
