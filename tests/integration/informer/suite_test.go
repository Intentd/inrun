//go:build integration

package informer_test

import (
	"os"
	"testing"

	"github.com/intentd/inrun/tests/integration/testenv"
	"k8s.io/client-go/rest"
)

var (
	testCfg     *rest.Config
	testEnv     *testenv.Env
	crdFixtures = []string{"../../fixtures/crds"}
)

func TestMain(m *testing.M) {
	testEnv = testenv.Start(crdFixtures)
	testCfg = testEnv.Config
	code := m.Run()
	testEnv.Stop()
	os.Exit(code)
}
