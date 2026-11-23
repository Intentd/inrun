package external

import (
	"github.com/intentd/tools/external"

	"github.com/intentd/inrun/pkg/types"
)

// RequestHeaders returns spec's headers for one request: each value rendered
// with render, plus the idempotency key on POST and PATCH. identity names
// the CR (ObjectIdentity); method, url and body are the request as sent.
// A header the user declares under the key's name wins over the generated
// key.
func RequestHeaders(render func(string) (string, error), spec types.CallSpec, identity, method, url, body string) (map[string]string, error) {
	return external.RequestHeaders(render, spec.Headers, toIdempotency(spec.IdempotencyKey), identity, method, url, body)
}

// ObjectIdentity names the CR in data (a template context or an object
// map): its uid, or namespace/name when it has none yet.
func ObjectIdentity(data map[string]interface{}) string {
	return external.ObjectIdentity(data)
}
