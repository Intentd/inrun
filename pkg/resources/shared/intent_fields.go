package shared

// DeploymentIntentFields are the fields accepted in the remote reconciler
// intent form for a deployment resource.
type DeploymentIntentFields struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Replicas    int32             `json:"replicas"`
	Port        int32             `json:"port"`
	Env         map[string]string `json:"env"`
	Command     []string          `json:"command"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// ServiceIntentFields are the fields accepted in the remote reconciler
// intent form for a service resource.
type ServiceIntentFields struct {
	Name        string            `json:"name"`
	Port        int32             `json:"port"`
	TargetPort  int32             `json:"targetPort"`
	Type        string            `json:"type"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// ConfigMapIntentFields are the fields accepted in the remote reconciler
// intent form for a configmap resource.
type ConfigMapIntentFields struct {
	Name        string            `json:"name"`
	Data        map[string]string `json:"data"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// SecretIntentFields are the fields accepted in the remote reconciler
// intent form for a secret resource.
// Data values must be base64-encoded; StringData values are plaintext.
type SecretIntentFields struct {
	Name        string            `json:"name"`
	Data        map[string]string `json:"data"`
	StringData  map[string]string `json:"stringData"`
	SecretType  string            `json:"secretType"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// ServiceAccountIntentFields are the fields accepted in the remote reconciler
// intent form for a serviceaccount resource.
type ServiceAccountIntentFields struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// NamespaceIntentFields are the fields accepted in the remote reconciler
// intent form for a namespace resource.
type NamespaceIntentFields struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// PDBIntentFields  are the fields accepted in the remote reconciler
// intent form for a PodDisruptionBudget resource.
type PDBIntentFields struct {
	Name           string            `json:"name"`
	Selector       map[string]string `json:"selector"`
	MinAvailable   string            `json:"minAvailable"`
	MaxUnavailable string            `json:"maxUnavailable"`
	Labels         map[string]string `json:"labels"`
	Annotations    map[string]string `json:"annotations"`
}

// CustomResourceIntentFields are the fields accepted in the remote reconciler
// intent form for a custom resource (type: custom).
// The spec is schema-agnostic; any top-level fields beyond spec must use full form.
type CustomResourceIntentFields struct {
	APIVersion  string                 `json:"apiVersion"`
	Kind        string                 `json:"kind"`
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace,omitempty"`
	Spec        map[string]interface{} `json:"spec,omitempty"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
}

// IngressIntentFields are the fields accepted in the remote reconciler
// intent form for an ingress resource.
type IngressIntentFields struct {
	Name         string            `json:"name"`
	Host         string            `json:"host"`
	ServiceName  string            `json:"serviceName"`
	ServicePort  int32             `json:"servicePort"`
	Path         string            `json:"path"`
	PathType     string            `json:"pathType"`
	IngressClass string            `json:"ingressClass"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
}

// StatefulSetIntentFields are the fields accepted in the remote reconciler
// intent form for a statefulset resource.
type StatefulSetIntentFields struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Replicas    int32             `json:"replicas"`
	Port        int32             `json:"port"`
	ServiceName string            `json:"serviceName"`
	Env         map[string]string `json:"env"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// JobIntentFields are the fields accepted in the remote reconciler
// intent form for a job resource.
type JobIntentFields struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Command      []string          `json:"command"`
	Args         []string          `json:"args"`
	BackoffLimit int               `json:"backoffLimit"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
}

// CronJobIntentFields are the fields accepted in the remote reconciler
// intent form for a cronjob resource.
type CronJobIntentFields struct {
	Name              string            `json:"name"`
	Image             string            `json:"image"`
	Schedule          string            `json:"schedule"`
	Command           []string          `json:"command"`
	Args              []string          `json:"args"`
	ConcurrencyPolicy string            `json:"concurrencyPolicy"`
	Suspend           bool              `json:"suspend"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
}
