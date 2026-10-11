package catalog_test

import (
	"github.com/intentd/inrun/pkg/catalog"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func catalogWithCronCondition(cron string, duration types.Duration) *catalog.Catalog {
	return catalog.NewCatalogForTest(map[string]types.CRDEntry{
		"myresource": {
			OperatorBox: &types.OperatorBoxConfig{
				Reconcile: &types.ReconcileConfig{
					OnReconcile: &types.HookTemplates{
						Deployments: []types.DeploymentTemplateSource{
							{
								Name: "my-app",
								Conditions: []types.Condition{
									{Cron: cron, Duration: duration},
								},
							},
						},
					},
				},
			},
		},
	})
}

func TestCronConditionWarnings_NoDuration(t *testing.T) {
	e := catalogWithCronCondition("0 9 * * 1", types.Duration{})
	warnings := e.CronConditionWarnings()
	if len(warnings) == 0 {
		t.Fatal("expected warning for cron without duration")
	}
	if !strings.Contains(warnings[0], "0 9 * * 1") {
		t.Errorf("warning should contain the cron expression, got: %s", warnings[0])
	}
}

func TestCronConditionWarnings_WithDuration(t *testing.T) {
	e := catalogWithCronCondition("0 9 * * 1", types.Duration{Duration: 4 * 3600 * 1000000000}) // 4h
	warnings := e.CronConditionWarnings()
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings when duration is set, got: %v", warnings)
	}
}

func TestCronConditionWarnings_NoCronCondition(t *testing.T) {
	e := catalogWithCronCondition("", types.Duration{})
	warnings := e.CronConditionWarnings()
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings when no cron condition, got: %v", warnings)
	}
}
