package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kome/declaration"
	"github.com/intentd/tools/condition"
)

func TestMigrationFor_Field(t *testing.T) {
	crd := &types.CRDEntry{
		APITypes:         types.APITypes{Group: "example.com", Version: "v1", Kind: "Website", Plural: "websites"},
		GroupVersionKind: schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Website"},
		Serve: &types.ServeConfig{Enabled: true, Fields: map[string]types.ServeFieldConfig{
			"replicas": {Path: "scaling.min", Migrate: &types.ServeFieldMigrate{From: "spec.replicas"}},
			"image":    {},
			"schedule": {Values: map[string]string{"schedule.minute": "{{ .value }}"}, Migrate: &types.ServeFieldMigrate{From: "schedule"}},
		}},
		Migrate: &types.CRMigrate{Selection: types.MigrateSelection{
			Namespaces: []string{"team"},
			When:       types.Conditions{{Condition: condition.Condition{Field: "spec.env", Equals: "prod"}}},
		}},
	}
	m, label := migrationFor("website", crd)
	assert.Equal(t, declaration.LevelField, m.Level)
	assert.Equal(t, "Website", label)
	assert.Equal(t, schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Website"}, m.Field.GVK)
	assert.Equal(t, []string{"team"}, m.Field.Selection.Namespaces, "migrate.selection scopes field migrations too")
	assert.Equal(t, "prod", m.Field.Selection.When[0].Equals)

	assert.Len(t, m.Field.Moves, 2, "only fields with migrate.from move")
	byKey := map[string]int{}
	for i, mv := range m.Field.Moves {
		byKey[mv.Key] = i
	}
	rep := m.Field.Moves[byKey["replicas"]]
	assert.Equal(t, "replicas", rep.From, "the spec. prefix is stripped")
	assert.Equal(t, []string{"scaling.min"}, rep.Destinations())
	assert.Equal(t, []string{"schedule.minute"}, m.Field.Moves[byKey["schedule"]].Destinations())
}

func TestMigrationFor_Kind(t *testing.T) {
	no := false
	crd := &types.CRDEntry{
		APITypes:         types.APITypes{Group: "old.io", Version: "v1", Kind: "App", Plural: "apps"},
		GroupVersionKind: schema.GroupVersionKind{Group: "old.io", Version: "v1", Kind: "App"},
		Migrate: &types.CRMigrate{
			To: types.CRMigrateTarget{APIVersion: "new.io/v1", Kind: "App", Namespace: "default", CopyStatus: true,
				Metadata: types.CRMigrateMetadata{PreserveSource: &no, Labels: map[string]string{"a": "b"}}},
			Prune: true,
		},
	}
	m, label := migrationFor("app", crd)
	assert.Equal(t, declaration.LevelKind, m.Level)
	assert.Equal(t, "App → App.new.io", label, "same kind: the group is what changes")
	k := m.Kind
	assert.Equal(t, "new.io", k.To.Group)
	assert.True(t, k.Prune)
	assert.True(t, k.CopyStatus)
	assert.False(t, k.PreserveSource)
	assert.Equal(t, "default", k.Namespace)
	assert.Equal(t, map[string]string{"a": "b"}, k.Labels)
}

func TestModeNames(t *testing.T) {
	assert.Equal(t, "forward", ModeForward.String())
	assert.Equal(t, "rollback", ModeRollback.String())
	assert.Equal(t, "clean", ModeClean.String())
}
