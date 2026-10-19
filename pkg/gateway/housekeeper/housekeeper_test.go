package housekeeper

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
)

const testBlueprint = `
apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: housekeeper
gateway:
  endpoint: http://inrun-gateway:8080
  api:
    enabled: true
    auth:
      tokens:
        - name: console
          secretRef:
            name: console-token
            key: token
security:
  deletionProtection:
    enabled: true
    strictMode: %s
spec:
  crds:
    app:
      apiTypes:
        group: test.inrun.dev
        version: v1alpha1
        kind: App
        plural: apps
      operatorBox:
        reconcile:
          onCreate:
            configMaps:
              - name: "{{ .metadata.name }}"
                data:
                  key: value
`

// newTestHousekeeper returns a Housekeeper on a fake clientset, trusting a
// certificate file in a temp dir.
func newTestHousekeeper(t *testing.T, strictMode bool, objects ...runtime.Object) (*Housekeeper, *fake.Clientset) {
	t.Helper()
	strict := "false"
	if strictMode {
		strict = "true"
	}
	bp, err := blueprint.ParseBytes([]byte(fmt.Sprintf(testBlueprint, strict)), t.TempDir())
	if err != nil {
		t.Fatalf("parse blueprint: %v", err)
	}

	certFile := filepath.Join(t.TempDir(), "tls.crt")
	if err := os.WriteFile(certFile, []byte("FAKECERT"), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	cfg := config.NewDefaultConfig()
	cfg.Security().Webhooks.TLSCert = certFile

	client := fake.NewClientset(objects...)
	return New(client, bp, cfg), client
}
