package generate

import (
	"fmt"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	defaultConfigMapName = "inrun-catalog"
	defaultConfigMapKey  = "catalog.yaml"
)

// ConfigMap renders a Namespace + ConfigMap from pre-expanded catalog bytes.
// expandedYAML must be the output of catalog.Catalog.SerializeExpanded() —
// fully resolved, no OCI imports remaining.
func ConfigMap(expandedYAML []byte, namespace string) ([]byte, error) {
	if namespace == "" {
		namespace = config.GetStrEnv("INRUN_NAMESPACE", "inrun-system")
	}
	return renderNamespaceAndConfigMap(expandedYAML, namespace)
}

// renderNamespaceAndConfigMap is the full standalone output: Namespace + ConfigMap.
func renderNamespaceAndConfigMap(expandedYAML []byte, namespace string) ([]byte, error) {
	nsBytes, err := renderNamespace(namespace)
	if err != nil {
		return nil, err
	}
	cmBytes, err := renderConfigMapBytes(expandedYAML, namespace)
	if err != nil {
		return nil, err
	}
	return []byte("---\n" + string(nsBytes) + "\n---\n" + string(cmBytes)), nil
}

// renderConfigMapBytes marshals a ConfigMap embedding the given expanded catalog YAML.
// The Namespace is intentionally excluded — callers prepend it so that
// bundle assembly can include it exactly once.
func renderConfigMapBytes(expandedYAML []byte, namespace string) ([]byte, error) {
	cm := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultConfigMapName,
			Namespace: namespace,
			Labels:    labels.InrunBaseLabels(),
		},
		Data: map[string]string{
			defaultConfigMapKey: string(expandedYAML),
		},
	}
	out, err := yaml.Marshal(cm)
	if err != nil {
		return nil, fmt.Errorf("marshal configmap: %w", err)
	}
	return out, nil
}
