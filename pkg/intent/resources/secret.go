package resources

import (
	"encoding/base64"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/kubekit/core/secrets"
)

// SecretFields are the fields accepted in the remote reconciler
// intent form for a secret resource.
// Data values must be base64-encoded; StringData values are plaintext.
type SecretFields struct {
	Name        string            `json:"name"`
	Data        map[string]string `json:"data"`
	StringData  map[string]string `json:"stringData"`
	SecretType  string            `json:"secretType"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// BuildSecret builds the Secret a "secret" resource intent describes, owned
// by owner. Required: name.
func BuildSecret(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f SecretFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("secret intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("secret intent: name is required")
	}
	if len(f.Data) == 0 && len(f.StringData) == 0 {
		return nil, fmt.Errorf("secret intent: data or stringData is required")
	}

	var data map[string][]byte
	if len(f.Data) > 0 {
		data = make(map[string][]byte, len(f.Data))
		for k, v := range f.Data {
			decoded, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				return nil, fmt.Errorf("secret intent: data[%q] is not valid base64: %w", k, err)
			}
			data[k] = decoded
		}
	}

	namespace := resolveNamespace(owner, "")
	spec := secrets.Spec{Name: f.Name, Namespace: namespace, Type: f.SecretType}

	spec.Data = data
	spec.StringData = f.StringData
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	obj := secrets.Build(owner, spec)
	rawMap, err := toObjectMap(obj, "v1", "Secret")
	if err != nil {
		return nil, fmt.Errorf("secret intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("secret intent: %w", err)
	}
	return rawMap, nil
}
