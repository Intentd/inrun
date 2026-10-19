package publish

import (
	"fmt"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
)

// validatePublish checks the publish: block for structural correctness.
// Semantic enforcement (does intent.yaml exist) happens
// at push time — the blueprint may be authored without an intent file.
func (c *checker) validatePublish() error {
	pub := c.Blueprint.Publish
	if pub == nil {
		return nil
	}

	if err := validatePublishTests(c.Blueprint, pub); err != nil {
		return err
	}

	return nil
}

func validatePublishTests(bp *blueprint.Blueprint, pub *types.PublishConfig) error {
	if !pub.TestsConfig().IntentEnabled() {
		return nil
	}

	// intent: true requires gateway.api — intent play targets the serve endpoint.
	if !bp.IsGatewayEnabled() || !bp.Gateway.HasAPI() {
		return fmt.Errorf("%s publish.tests.intent: true requires gateway.api.enabled: true — intent play targets the serve endpoint", failureMark())
	}

	// intent: true requires intent.yaml or intent.json to be present.
	if !bp.HasIntentFiles() {
		return fmt.Errorf("%spublish.tests.intent: true requires intent.yaml or intent.json in the pattern directory", failureMark())
	}

	return nil
}
