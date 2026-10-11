package secrets

import (
	"encoding/base64"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/shared"
)

// Build converts a flat intent fields map into a full Secret object map
// suitable for inclusion in domain.Result.Resources and application via SSA.
// Data values must be base64-encoded strings; StringData values are plaintext.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.SecretIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("secret.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("secret.BuildFromIntent: name is required")
	}
	if len(f.Data) == 0 && len(f.StringData) == 0 {
		return nil, fmt.Errorf("secret.BuildFromIntent: data or stringData is required")
	}

	var data map[string][]byte
	if len(f.Data) > 0 {
		data = make(map[string][]byte, len(f.Data))
		for k, v := range f.Data {
			decoded, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				return nil, fmt.Errorf("secret.BuildFromIntent: data[%q] is not valid base64: %w", k, err)
			}
			data[k] = decoded
		}
	}

	namespace := shared.ResolveNamespace(owner, "")
	spec := ResolvedSecretSpec{Name: f.Name, Namespace: namespace, Type: f.SecretType}

	obj := buildSecret(owner, spec, namespace, data, f.StringData)
	rawMap, err := shared.ToObjectMap(obj, "v1", "Secret")
	if err != nil {
		return nil, fmt.Errorf("secret.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("secret.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}
