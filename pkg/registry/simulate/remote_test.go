package simulate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
	"github.com/intentd/inrun/pkg/merger"
)

const remoteTestCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: webapps.testground.inrun.dev
spec:
  group: testground.inrun.dev
  names:
    kind: WebApp
    plural: webapps
    singular: webapp
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          x-kubernetes-preserve-unknown-fields: true
`

const remoteTestSecret = `
apiVersion: v1
kind: Secret
metadata:
  name: remote-token
  namespace: default
stringData:
  token: test-token
`

const remoteTestBlueprint = `
apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: remote-harness-test
  version: 0.1.0

spec:
  crds:
    webapp:
      apiTypes:
        group: testground.inrun.dev
        version: v1alpha1
        kind: WebApp
        plural: webapps
      crdFile: ./crd.yaml
      setup:
        - ./secret.yaml
      operatorBox:
        reconcile:
          default: false
          remote:
            endpoint: ENDPOINT
            auth:
              secretRef:
                name: remote-token
                namespace: default
                key: token
          managedResources:
            - group: apps
              plural: deployments
            - group: ""
              plural: services
`

// TestRun_RemoteReconciler proves a remote reconciler runs in simulation: the
// token Secret from setup is seeded and sent, the endpoint is called, and the
// resources it returns are applied to the fake cluster.
func TestRun_RemoteReconciler(t *testing.T) {
	var calls, badAuth atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-token" {
			badAuth.Add(1)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "ok",
			"status": map[string]interface{}{"phase": "Running"},
			"resources": []map[string]interface{}{
				{"type": "deployment", "fields": map[string]interface{}{"name": "my-app", "image": "nginx:1.25", "replicas": 1, "port": 80}},
				{"type": "service", "fields": map[string]interface{}{"name": "my-app-svc", "port": 80, "targetPort": 80}},
			},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	files := map[string]string{
		"crd.yaml":       remoteTestCRD,
		"secret.yaml":    remoteTestSecret,
		"blueprint.yaml": strings.Replace(remoteTestBlueprint, "ENDPOINT", srv.URL+"/reconcile", 1),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	m := merger.New(filepath.Join(dir, "blueprint.yaml"))
	if err := m.Merge(); err != nil {
		t.Fatalf("merging blueprint: %v", err)
	}
	bp, err := pipeline.BuildExpanded(config.NewDefaultConfig(), m)
	if err != nil {
		t.Fatalf("building blueprint: %v", err)
	}

	cr := object.New("testground.inrun.dev/v1alpha1", "WebApp", "my-app", object.InNamespace("default"),
		object.WithSpec(map[string]any{"image": "nginx:1.25"}))

	result, err := Run(context.Background(), bp, "webapp", cr, 2, RunOptions{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for i, c := range result.Cycles {
		if c.Error != nil {
			t.Fatalf("cycle %d: %v", i+1, c.Error)
		}
	}
	if calls.Load() == 0 {
		t.Fatal("remote endpoint was never called")
	}
	if badAuth.Load() != 0 {
		t.Errorf("%d call(s) without the token from the setup Secret", badAuth.Load())
	}

	applied := map[string]bool{}
	for _, op := range result.AllOps {
		if op.Cycle == 1 && op.Verb == "apply" {
			applied[op.Resource] = true
		}
	}
	for _, res := range []string{"deployments", "services"} {
		if !applied[res] {
			t.Errorf("expected an apply of %s in cycle 1; ops: %+v", res, result.AllOps)
		}
	}
}
