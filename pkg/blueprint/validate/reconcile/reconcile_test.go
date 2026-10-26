package reconcile

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/types"
)

func blueprintWithQueue(crdName string, q *types.Queue) *checker {
	if q.Empty() {
		q = &types.Queue{}
	}
	return newBlueprintExec(map[string]types.CRDEntry{
		crdName: {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{Queue: *q},
		}},
	})
}

func TestValidateQueue_Nil(t *testing.T) {
	e := blueprintWithQueue("app", nil)
	assert.NoError(t, e.validateQueue())
}

func TestValidateQueue_OnLimitValid(t *testing.T) {
	e := blueprintWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop: ptr.To(true),
			},
		},
	})
	assert.NoError(t, e.validateQueue())
}

func TestValidateQueue_UnLimitedValid(t *testing.T) {
	e := blueprintWithQueue("app", &types.Queue{MaxDepth: 0})
	assert.NoError(t, e.validateQueue())
}

func TestValidateQueue_LimitedValid(t *testing.T) {
	e := blueprintWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop: ptr.To(true),
			},
		},
	})
	assert.NoError(t, e.validateQueue())
}

func TestValidateQueue_UnLimitedInValid(t *testing.T) {
	e := blueprintWithQueue("app", &types.Queue{
		MaxDepth: 0,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop: ptr.To(true),
			},
		},
	})

	err := e.validateQueue()
	require.Error(t, err)
	assert.ErrorContains(t, err, "'queue.behaviour' configuration is only valid when 'queue.maxDepth' is greater than 0")
}

func TestValidateQueue_OnLimitDropValid(t *testing.T) {
	e := blueprintWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop: ptr.To(false),
			},
		},
	})
	assert.NoError(t, e.validateQueue())
}

func TestValidateQueue_OnLimitWithValueInValid(t *testing.T) {
	e := blueprintWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnLimit: &types.QueueBehaviourSetting{
				Drop:  ptr.To(true),
				Value: 5,
			},
		},
	})

	err := e.validateQueue()
	require.Error(t, err)
	assert.ErrorContains(t, err, "is not allowed in onLimit configuration")
}

func TestValidateQueue_onThresholdWarning(t *testing.T) {
	e := blueprintWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnThreshold: &types.QueueBehaviourSetting{
				Drop:  ptr.To(false),
				Value: 5,
			},
		},
	})

	err := e.validateQueue()
	assert.NoError(t, err)

	entry := e.Blueprint.EnabledCRDs()["app"]
	if !entry.Warnings.HasWarnings() {
		t.Fatal("expected a warning for an explicit drop: false")
	}

}

func TestValidateQueue_onThresholdWithoutDropHasNoWarning(t *testing.T) {
	e := blueprintWithQueue("app", &types.Queue{
		MaxDepth: 100,
		Cfg: &types.QueueBehaviour{
			OnThreshold: &types.QueueBehaviourSetting{Value: 5},
		},
	})

	require.NoError(t, e.validateQueue())
	entry := e.Blueprint.EnabledCRDs()["app"]
	if entry.Warnings.HasWarnings() {
		t.Fatal("drop omitted is the default; no warning expected")
	}
}

// ── Remote reconciler validation ─────────────────────────────────────────────

func remoteResources() []domain.ManagedResource {
	return []domain.ManagedResource{{Group: "apps", Version: "v1", Plural: "deployments"}}
}

func blueprintWithRemote(endpoint string, protocol types.RemoteReconcileProtocol, resources []domain.ManagedResource) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
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
	e := blueprintWithRemote("https://svc.internal/reconcile", types.RemoteReconcileProtocolHTTP, remoteResources())
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemote_NoEndpoint(t *testing.T) {
	e := blueprintWithRemote("", types.RemoteReconcileProtocolHTTP, remoteResources())
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "endpoint is required")
}

func TestValidateRemote_InvalidType(t *testing.T) {
	e := blueprintWithRemote("https://svc.internal/reconcile", "websocket", remoteResources())
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "not valid")
}

func TestValidateRemote_NoManagedResources_Info(t *testing.T) {
	e := blueprintWithRemote("https://svc.internal/reconcile", types.RemoteReconcileProtocolHTTP, nil)
	crd := crdPtr(e, "app")
	require.NoError(t, e.validateManagedResources("app", crd))
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
	e := blueprintWith(map[string]types.CRDEntry{"app": crd})
	err := e.validateRemote("app", crdPtr(e, "app"))
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
	e := blueprintWith(map[string]types.CRDEntry{"app": crd})
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "cannot declare both")
}

func TestValidateRemote_TypeOmitted_Valid(t *testing.T) {
	e := blueprintWithRemote("https://svc.internal/reconcile", "", remoteResources())
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemote_TemplateEndpoint_Valid(t *testing.T) {
	e := blueprintWithRemote("http://{{ .metadata.namespace }}-svc.svc.cluster.local/reconcile", types.RemoteReconcileProtocolHTTP, remoteResources())
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemote_TemplateEndpoint_InvalidTemplate(t *testing.T) {
	e := blueprintWithRemote("http://{{ .metadata.namespace }-svc/reconcile", types.RemoteReconcileProtocolHTTP, remoteResources())
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "invalid template")
}

func TestValidateArgs_ValidTemplate(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
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
	require.NoError(t, e.validateManagedResources("app", crdPtr(e, "app")))
}

func TestValidateArgs_InvalidTemplate(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
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
	err := e.validateManagedResources("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "invalid template")
}

func TestValidateRemote_DefaultFalse_Valid(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				Default:          ptr.To(false),
				ManagedResources: remoteResources(),
				Remote: &types.RemoteReconcilerDeclaration{
					Endpoint: "https://svc.internal/reconcile",
				},
			},
		}},
	})
	crd := crdPtr(e, "app")
	require.NoError(t, e.validateRemote("app", crd))
	require.NoError(t, e.validateManagedResources("app", crd))
}

// ── Remote auth validation ────────────────────────────────────────────────────

func remoteWithAuth(auth *types.ExternalAuth) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
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
	e := remoteWithAuth(&types.ExternalAuth{
		SecretRef: &types.APISecretRef{Name: "my-token", Key: "token", Namespace: "default"},
	})
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemote_Auth_SecretRef_MissingName(t *testing.T) {
	e := remoteWithAuth(&types.ExternalAuth{
		SecretRef: &types.APISecretRef{Key: "token", Namespace: "default"},
	})
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "name")
}

func TestValidateRemote_Auth_SecretRef_MissingKey(t *testing.T) {
	e := remoteWithAuth(&types.ExternalAuth{
		SecretRef: &types.APISecretRef{Name: "my-token", Namespace: "default"},
	})
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "key")
}

func TestValidateRemote_Auth_SecretRef_NoNamespace_Warning(t *testing.T) {
	e := remoteWithAuth(&types.ExternalAuth{
		SecretRef: &types.APISecretRef{Name: "my-token", Key: "token"},
	})
	crd := crdPtr(e, "app")
	require.NoError(t, e.validateRemote("app", crd))
	assert.True(t, crd.Warnings.Contains("namespace is empty"))
}

func TestValidateRemote_Auth_Env_NoValidation(t *testing.T) {
	e := remoteWithAuth(&types.ExternalAuth{Env: "RECONCILER_TOKEN"})
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

// crdPtr returns a pointer to the named CRD entry from the executor's blueprint.
func crdPtr(e *checker, name string) *types.CRDEntry {
	entry := e.Blueprint.EnabledCRDs()[name]
	return &entry
}

// ── Remote request validation ─────────────────────────────────────────────────

// remoteResourcesWithKind returns managed resources with Kind set so LookupBuiltIn resolves them.
func remoteResourcesWithKind() []domain.ManagedResource {
	return []domain.ManagedResource{
		{Kind: "Deployment", Group: "apps", Version: "v1", Plural: "deployments"},
		{Kind: "Service", Group: "", Version: "v1", Plural: "services"},
	}
}

func blueprintWithRemoteRequest(request *types.RemoteRequestConfig) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
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
	e := blueprintWithRemoteRequest(nil)
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemoteRequest_ChildrenDisabled_SkipsChildrenValidation(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Enabled: ptr.To(false),
			// invalid paths that would normally error — should be skipped
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"nonexistent": {},
			},
		},
	})
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemoteRequest_ObjectExclude_Valid(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Object: &types.RemoteRequestObjectConfig{
			Exclude: []string{"metadata.managedFields", "metadata.annotations"},
		},
	})
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemoteRequest_ObjectExclude_InvalidPath(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Object: &types.RemoteRequestObjectConfig{
			Exclude: []string{"metadata..managedFields"},
		},
	})
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "request.object.exclude")
	assert.ErrorContains(t, err, "empty segment")
}

func TestValidateRemoteRequest_ObjectExclude_LeadingDot(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Object: &types.RemoteRequestObjectConfig{
			Exclude: []string{".metadata.managedFields"},
		},
	})
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "request.object.exclude")
}

func TestValidateRemoteRequest_Children_RootExclude_Valid(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Exclude: []string{"metadata.managedFields"},
		},
	})
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemoteRequest_Children_RootExclude_InvalidPath(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Exclude: []string{"spec.template..annotations"},
		},
	})
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "request.children.exclude")
	assert.ErrorContains(t, err, "empty segment")
}

func TestValidateRemoteRequest_Children_ResourcesFilter_Valid(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"deployment": {},
				"service":    nil,
			},
		},
	})
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemoteRequest_Children_ResourceNotInManaged(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"configmap": {},
			},
		},
	})
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, "request.children.resources")
	assert.ErrorContains(t, err, "configmap")
	assert.ErrorContains(t, err, "not declared in reconcile.managedResources")
}

func TestValidateRemoteRequest_Children_PerResourceExclude_Valid(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"deployment": {Exclude: []string{"spec.template.metadata.annotations"}},
			},
		},
	})
	require.NoError(t, e.validateRemote("app", crdPtr(e, "app")))
}

func TestValidateRemoteRequest_Children_PerResourceExclude_InvalidPath(t *testing.T) {
	e := blueprintWithRemoteRequest(&types.RemoteRequestConfig{
		Children: &types.RemoteRequestChildrenConfig{
			Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
				"deployment": {Exclude: []string{"spec.template."}},
			},
		},
	})
	err := e.validateRemote("app", crdPtr(e, "app"))
	assert.ErrorContains(t, err, `resources["deployment"].exclude`)
}

// ── Prune request validation ─────────────────────────────────────────────────

func pruneCRD(prune *types.PruneConfig, managed ...domain.ManagedResource) *types.CRDEntry {
	return &types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
		Remote:           &types.RemoteReconcilerDeclaration{Endpoint: "http://r"},
		ManagedResources: managed,
		Prune:            prune,
	}}}
}

func TestValidatePrune(t *testing.T) {
	deploy, pvc := domain.ManagedResource{Kind: "deployment"}, domain.ManagedResource{Kind: "persistentvolumeclaim"}

	assert.NoError(t, validatePrune("app", pruneCRD(nil)))
	assert.NoError(t, validatePrune("app", pruneCRD(&types.PruneConfig{Enabled: true}, deploy)))
	assert.NoError(t, validatePrune("app", pruneCRD(&types.PruneConfig{Enabled: true, Keep: []string{"pvc"}}, deploy, pvc)))

	err := validatePrune("app", pruneCRD(&types.PruneConfig{Enabled: true}))
	assert.ErrorContains(t, err, "managedResources is empty")

	err = validatePrune("app", pruneCRD(&types.PruneConfig{Enabled: true, Keep: []string{"secret"}}, deploy))
	assert.ErrorContains(t, err, `prune.keep[0]: "secret" is not declared`)

	// A serve target is checked against its own managedResources.
	crd := pruneCRD(nil, deploy)
	crd.Serve = &types.ServeConfig{Target: types.ServeTargetValue{Entries: map[string]*types.ServeTargetConfig{
		"web": {OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
			Prune: &types.PruneConfig{Enabled: true, Keep: []string{"persistentvolumeclaim"}},
		}}},
	}}}
	err = validatePrune("app", crd)
	assert.ErrorContains(t, err, "serve.target[web].reconcile.prune.keep[0]")
}

func TestValidateManagedResources_PruneOnDeclarative(t *testing.T) {
	crd := &types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
		Prune: &types.PruneConfig{Enabled: true},
	}}}
	err := (&checker{}).validateManagedResources("app", crd)
	assert.ErrorContains(t, err, "reconcile.prune apply only to hooks")
}
