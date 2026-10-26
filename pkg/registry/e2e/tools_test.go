package e2e

import (
	"testing"

	"github.com/intentd/inrun/pkg/registry/e2e/expect"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestScanToolRequirements_ExternalJQ(t *testing.T) {
	c := expect.Call("health", "http://x.invalid")
	c.JQ = ".json.state"
	_, needJQ, _, _ := scanToolRequirements(types.E2E{Spec: types.E2ESpec{Expect: []types.E2EExpectation{{External: []types.E2EExternalCheck{c}}}}})
	assert.True(t, needJQ)
}
