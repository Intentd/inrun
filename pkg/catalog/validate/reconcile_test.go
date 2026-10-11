package validate

import (
	"testing"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithQueue(crdName string, q *types.Queue) *executor {
	if q.Empty() {
		q = &types.Queue{}
	}
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{Queue: *q},
		}},
	})
}

func TestValidateQueue_Nil(t *testing.T) {
	k := catalogWithQueue("app", nil)
	assert.NoError(t, k.validateQueue())
}

func TestValidateQueue_OnLimitValid(t *testing.T) {
	k := catalogWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop: boolPtr(true),
			},
		},
	})
	assert.NoError(t, k.validateQueue())
}

func TestValidateQueue_UnLimitedValid(t *testing.T) {
	k := catalogWithQueue("app", &types.Queue{MaxDepth: 0})
	assert.NoError(t, k.validateQueue())
}

func TestValidateQueue_LimitedValid(t *testing.T) {
	k := catalogWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop: boolPtr(true),
			},
		},
	})
	assert.NoError(t, k.validateQueue())
}

func TestValidateQueue_UnLimitedInValid(t *testing.T) {
	k := catalogWithQueue("app", &types.Queue{
		MaxDepth: 0,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop: boolPtr(true),
			},
		},
	})

	err := k.validateQueue()
	require.Error(t, err)
	assert.ErrorContains(t, err, "'queue.behaviour' configuration is only valid when 'queue.maxDepth' is greater than 0")
}

func TestValidateQueue_OnLimitDropValid(t *testing.T) {
	k := catalogWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop: boolPtr(false),
			},
		},
	})
	assert.NoError(t, k.validateQueue())
}

func TestValidateQueue_OnLimitWithValueInValid(t *testing.T) {
	k := catalogWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop:  boolPtr(true),
				Value: 5,
			},
		},
	})

	err := k.validateQueue()
	require.Error(t, err)
	assert.ErrorContains(t, err, "is not allowed in onLimit configuration")
}

func TestValidateQueue_onThresholdWarning(t *testing.T) {
	k := catalogWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnThreshold: &types.QueueBehaviourSetting{
				Drop:  boolPtr(false),
				Value: 5,
			},
		},
	})

	err := k.validateQueue()
	assert.NoError(t, err)

	entry := k.k.EnabledCRDs()["app"]

	if !entry.Warnings.HasWarnings() {
		t.Fatal("expected crd to have warning")
	}

	// Check if drop is set correctly
	cfg := entry.QueueConfig().Behaviour()
	// got := cfg.onThreshold.ShouldDrop()
	got := *cfg.OnThreshold.Drop

	if !got {
		t.Fatalf("expected onThreshold.drop to be true. got: %v", got)
	}
}

// ── Remote reconciler validation ─────────────────────────────────────────────

func remoteResources() []domain.ManagedResource {
	return []domain.ManagedResource{{Group: "apps", Version: "v1", Plural: "deployments"}}
}

func catalogWithRemote(endpoint string, protocol types.RemoteReconcileProtocol, resources []domain.ManagedResource) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		"app": {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				ManagedResources: resources,
				Remote: &types.RemoteReconcilerDeclaration{
					Protocol: protocol,
					Endpoint: endpoint,
				},
			},
		}},
	})
}

func TestValidateRemote_Valid(t *testing.T) {
	k := catalogWithRemote("https://svc.internal/reconcile", types.RemoteReconcileProtocolHTTP, remoteResources())
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemote_NoEndpoint(t *testing.T) {
	k := catalogWithRemote("", types.RemoteReconcileProtocolHTTP, remoteResources())
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "endpoint is required")
}

func TestValidateRemote_InvalidType(t *testing.T) {
	k := catalogWithRemote("https://svc.internal/reconcile", "websocket", remoteResources())
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "not valid")
}

func TestValidateRemote_NoManagedResources_Info(t *testing.T) {
	k := catalogWithRemote("https://svc.internal/reconcile", types.RemoteReconcileProtocolHTTP, nil)
	crd := crdPtr(k, "app")
	require.NoError(t, k.validateManagedResources("app", crd))
	assert.True(t, crd.Info.HasInfo())
	assert.True(t, crd.Info.Contains("RBAC will not be generated"))
}

func TestValidateRemote_ConflictsWithHooks(t *testing.T) {
	crd := types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{
			Hooks:            &types.HookDeclaration{Location: "github.com/test/rec", Function: "New"},
			ManagedResources: remoteResources(),
			Remote: &types.RemoteReconcilerDeclaration{
				Endpoint: "https://svc.internal/reconcile",
			},
		},
	}}
	k := catalogWith(map[string]types.CRDEntry{"app": crd})
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "cannot declare both")
}

func TestValidateRemote_ConflictsWithConstructor(t *testing.T) {
	crd := types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{
		Reconcile: &types.ReconcileConfig{
			ConstructorDecl:  &types.ConstructorDeclaration{Location: "github.com/test/rec", Function: "New"},
			ManagedResources: remoteResources(),
			Remote: &types.RemoteReconcilerDeclaration{
				Endpoint: "https://svc.internal/reconcile",
			},
		},
	}}
	k := catalogWith(map[string]types.CRDEntry{"app": crd})
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "cannot declare both")
}

func TestValidateRemote_TypeOmitted_Valid(t *testing.T) {
	k := catalogWithRemote("https://svc.internal/reconcile", "", remoteResources())
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemote_TemplateEndpoint_Valid(t *testing.T) {
	k := catalogWithRemote("http://{{ .metadata.namespace }}-svc.svc.cluster.local/reconcile", types.RemoteReconcileProtocolHTTP, remoteResources())
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemote_TemplateEndpoint_InvalidTemplate(t *testing.T) {
	k := catalogWithRemote("http://{{ .metadata.namespace }-svc/reconcile", types.RemoteReconcileProtocolHTTP, remoteResources())
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "invalid template")
}

func TestValidateArgs_ValidTemplate(t *testing.T) {
	k := newCatalogExec(map[string]types.CRDEntry{
		"app": {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				ManagedResources: remoteResources(),
				Args:             map[string]interface{}{"replicas": "{{ .spec.replicas }}"},
				Remote: &types.RemoteReconcilerDeclaration{
					Endpoint: "https://svc.internal/reconcile",
				},
			},
		}},
	})
	require.NoError(t, k.validateManagedResources("app", crdPtr(k, "app")))
}

func TestValidateArgs_InvalidTemplate(t *testing.T) {
	k := newCatalogExec(map[string]types.CRDEntry{
		"app": {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				ManagedResources: remoteResources(),
				Args:             map[string]interface{}{"replicas": "{{ .spec.replicas }"},
				Remote: &types.RemoteReconcilerDeclaration{
					Endpoint: "https://svc.internal/reconcile",
				},
			},
		}},
	})
	err := k.validateManagedResources("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "invalid template")
}

func TestValidateRemote_DefaultFalse_Valid(t *testing.T) {
	k := newCatalogExec(map[string]types.CRDEntry{
		"app": {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				Default:          boolPtr(false),
				ManagedResources: remoteResources(),
				Remote: &types.RemoteReconcilerDeclaration{
					Endpoint: "https://svc.internal/reconcile",
				},
			},
		}},
	})
	crd := crdPtr(k, "app")
	require.NoError(t, k.validateRemote("app", crd))
	require.NoError(t, k.validateManagedResources("app", crd))
}

// ── Remote auth validation ────────────────────────────────────────────────────

func remoteWithAuth(auth *types.ExternalAuth) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		"app": {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				Remote: &types.RemoteReconcilerDeclaration{
					Endpoint: "https://svc.internal/reconcile",
					Auth:     auth,
				},
			},
		}},
	})
}

func TestValidateRemote_Auth_SecretRef_Valid(t *testing.T) {
	k := remoteWithAuth(&types.ExternalAuth{
		SecretRef: &types.APISecretRef{Name: "my-token", Key: "token", Namespace: "default"},
	})
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemote_Auth_SecretRef_MissingName(t *testing.T) {
	k := remoteWithAuth(&types.ExternalAuth{
		SecretRef: &types.APISecretRef{Key: "token", Namespace: "default"},
	})
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "name")
}

func TestValidateRemote_Auth_SecretRef_MissingKey(t *testing.T) {
	k := remoteWithAuth(&types.ExternalAuth{
		SecretRef: &types.APISecretRef{Name: "my-token", Namespace: "default"},
	})
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "key")
}

func TestValidateRemote_Auth_SecretRef_NoNamespace_Warning(t *testing.T) {
	k := remoteWithAuth(&types.ExternalAuth{
		SecretRef: &types.APISecretRef{Name: "my-token", Key: "token"},
	})
	crd := crdPtr(k, "app")
	require.NoError(t, k.validateRemote("app", crd))
	assert.True(t, crd.Warnings.Contains("namespace is empty"))
}

func TestValidateRemote_Auth_Env_NoValidation(t *testing.T) {
	k := remoteWithAuth(&types.ExternalAuth{Env: "RECONCILER_TOKEN"})
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

// crdPtr returns a pointer to the named CRD entry from the executor's catalog.
func crdPtr(k *executor, name string) *types.CRDEntry {
	e := k.k.EnabledCRDs()[name]
	return &e
}

// ── Remote request validation ─────────────────────────────────────────────────

// remoteResourcesWithKind returns managed resources with Kind set so LookupBuiltIn resolves them.
func remoteResourcesWithKind() []domain.ManagedResource {
	return []domain.ManagedResource{
		{Kind: "Deployment", Group: "apps", Version: "v1", Plural: "deployments"},
		{Kind: "Service", Group: "", Version: "v1", Plural: "services"},
	}
}

func catalogWithRemoteRequest(request *types.RemoteRequestConfig) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		"app": {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				ManagedResources: remoteResourcesWithKind(),
				Remote: &types.RemoteReconcilerDeclaration{
					Endpoint: "https://svc.internal/reconcile",
					Request:  request,
				},
			},
		}},
	})
}

func TestValidateRemoteRequest_Nil(t *testing.T) {
	k := catalogWithRemoteRequest(nil)
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemoteRequest_ChildrenDisabled_SkipsChildrenValidation(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Enabled: boolPtr(false),
			// invalid paths that would normally error — should be skipped
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"nonexistent": {},
			},
		},
	})
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemoteRequest_ObjectExclude_Valid(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Object: &types.RemoteRequestObjectConfig{
			Exclude: []string{"metadata.managedFields", "metadata.annotations"},
		},
	})
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemoteRequest_ObjectExclude_InvalidPath(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Object: &types.RemoteRequestObjectConfig{
			Exclude: []string{"metadata..managedFields"},
		},
	})
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "request.object.exclude")
	assert.ErrorContains(t, err, "empty segment")
}

func TestValidateRemoteRequest_ObjectExclude_LeadingDot(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Object: &types.RemoteRequestObjectConfig{
			Exclude: []string{".metadata.managedFields"},
		},
	})
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "request.object.exclude")
}

func TestValidateRemoteRequest_Children_RootExclude_Valid(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Exclude: []string{"metadata.managedFields"},
		},
	})
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemoteRequest_Children_RootExclude_InvalidPath(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Exclude: []string{"spec.template..annotations"},
		},
	})
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "request.children.exclude")
	assert.ErrorContains(t, err, "empty segment")
}

func TestValidateRemoteRequest_Children_ResourcesFilter_Valid(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"deployment": {},
				"service":    nil,
			},
		},
	})
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemoteRequest_Children_ResourceNotInManaged(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"configmap": {},
			},
		},
	})
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, "request.children.resources")
	assert.ErrorContains(t, err, "configmap")
	assert.ErrorContains(t, err, "not declared in reconcile.managedResources")
}

func TestValidateRemoteRequest_Children_PerResourceExclude_Valid(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"deployment": {Exclude: []string{"spec.template.metadata.annotations"}},
			},
		},
	})
	require.NoError(t, k.validateRemote("app", crdPtr(k, "app")))
}

func TestValidateRemoteRequest_Children_PerResourceExclude_InvalidPath(t *testing.T) {
	k := catalogWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"deployment": {Exclude: []string{"spec.template."}},
			},
		},
	})
	err := k.validateRemote("app", crdPtr(k, "app"))
	assert.ErrorContains(t, err, `resources["deployment"].exclude`)
}
