package types

// namer is a resource template with a name, for diagnostics and checks that
// walk every template.
type namer interface {
	GetName() string
}

// GetName returns the template's name field, which may still be a template
// expression.
func (t PodTemplateSource) GetName() string                { return t.Name }
func (t DeploymentTemplateSource) GetName() string         { return t.Name }
func (t ReplicaSetTemplateSource) GetName() string         { return t.Name }
func (t StatefulSetTemplateSource) GetName() string        { return t.Name }
func (t ServiceTemplateSource) GetName() string            { return t.Name }
func (t IngressTemplateSource) GetName() string            { return t.Name }
func (t NetworkPolicyTemplateSource) GetName() string      { return t.Name }
func (t JobTemplateSource) GetName() string                { return t.Name }
func (t CronJobTemplateSource) GetName() string            { return t.Name }
func (t SecretTemplateSource) GetName() string             { return t.Name }
func (t ConfigMapTemplateSource) GetName() string          { return t.Name }
func (t ServiceAccountTemplateSource) GetName() string     { return t.Name }
func (t RoleTemplateSource) GetName() string               { return t.Name }
func (t RoleBindingTemplateSource) GetName() string        { return t.Name }
func (t ClusterRoleTemplateSource) GetName() string        { return t.Name }
func (t ClusterRoleBindingTemplateSource) GetName() string { return t.Name }
func (t PVTemplateSource) GetName() string                 { return t.Name }
func (t PVCTemplateSource) GetName() string                { return t.Name }
func (t HPATemplateSource) GetName() string                { return t.Name }
func (t PDBTemplateSource) GetName() string                { return t.Name }
func (t NamespaceTemplateSource) GetName() string          { return t.Name }
func (t LimitRangeTemplateSource) GetName() string         { return t.Name }
func (t ResourceQuotaTemplateSource) GetName() string      { return t.Name }
func (t CustomResourceTemplateSource) GetName() string     { return t.Metadata.Name }
