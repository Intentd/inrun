package validate

import (
	"fmt"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/types"
)

// validatePublish checks the publish: block for structural correctness.
// Semantic enforcement (does intent.yaml exist) happens
// at push time — the catalog may be authored without an intent file.
func (e *executor) validatePublish() error {
	pub := e.cat.Publish
	if pub == nil {
		return nil
	}

	if err := validatePublishTests(e.cat, pub); err != nil {
		return err
	}

	return nil
}

func validatePublishTests(cat *catalog.Catalog, pub *types.PublishConfig) error {
	if !pub.TestsConfig().IntentEnabled() {
		return nil
	}

	// intent: true requires gateway.api — intent play targets the serve endpoint.
	if !cat.IsGatewayEnabled() || !cat.Gateway.HasAPI() {
		return fmt.Errorf("%s publish.tests.intent: true requires gateway.api.enabled: true — intent play targets the serve endpoint", failureMark())
	}

	// intent: true requires intent.yaml or intent.json to be present.
	if !cat.HasIntentFiles() {
		return fmt.Errorf("%spublish.tests.intent: true requires intent.yaml or intent.json in the pattern directory", failureMark())
	}

	return nil
}
