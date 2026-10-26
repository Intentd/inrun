package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

// ── presence ─────────────────────────────────────────────────────────────────

func TestValidateCRMigrate_Nil(t *testing.T) {
	c := blueprintWithMigrate(nil)
	require.NoError(t, c.validateMigrate())
}

// ── required destination fields ──────────────────────────────────────────────

func TestValidateCRMigrate_NoAPIVersion(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{Kind: "App"},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "apiVersion is required")
}

func TestValidateCRMigrate_NoKind(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{APIVersion: "test.migrate/v1"},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "kind is required")
}

// ── labels ───────────────────────────────────────────────────────────────────

func TestValidateCRMigrate_ValidLabels(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Labels: map[string]string{
					"app.kubernetes.io/name": "hello",
					"tier":                   "gold",
				},
			},
		},
	})
	require.NoError(t, c.validateMigrate())
}

func TestValidateCRMigrate_InvalidLabelKey(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Labels: map[string]string{"invalid key!": "value"},
			},
		},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.to.metadata.labels")
	assert.ErrorContains(t, err, "invalid key!")
}

func TestValidateCRMigrate_InvalidLabelValue(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Labels: map[string]string{"tier": "value/with/slashes"},
			},
		},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.to.metadata.labels")
}

func TestValidateCRMigrate_ValidLabelsTemplate(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Labels: map[string]string{
					"example.io/owner": "{{ .metadata.name }}",
				},
			},
		},
	})
	require.NoError(t, c.validateMigrate())
}

func TestValidateCRMigrate_InvalidLabelsTemplate(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Labels: map[string]string{
					"example.io/owner": "{{ .metadata.name }",
				},
			},
		},
	})
	require.Error(t, c.validateMigrate())
}

// ── annotations ──────────────────────────────────────────────────────────────

func TestValidateCRMigrate_ValidAnnotations(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Annotations: map[string]string{
					"example.io/owner": "team-a",
				},
			},
		},
	})
	require.NoError(t, c.validateMigrate())
}

func TestValidateCRMigrate_InvalidAnnotationKey(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Annotations: map[string]string{"invalid annotation!": "value"},
			},
		},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.to.metadata.annotations")
	assert.ErrorContains(t, err, "invalid annotation!")
}

func TestValidateCRMigrate_AnnotationValueNotChecked(t *testing.T) {
	// Annotation values are unconstrained; anything goes.
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Annotations: map[string]string{
					"example.io/desc": "any value with / slashes and spaces",
				},
			},
		},
	})
	require.NoError(t, c.validateMigrate())
}

func TestValidateCRMigrate_ValidAnnotationsTemplate(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Annotations: map[string]string{
					"example.io/owner": "{{ .metadata.name }}",
				},
			},
		},
	})
	require.NoError(t, c.validateMigrate())
}

func TestValidateCRMigrate_InvalidAnnotationsTemplate(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Annotations: map[string]string{
					"example.io/owner": "{{ .metadata.name }",
				},
			},
		},
	})
	require.Error(t, c.validateMigrate())
}

// ── prune warning ────────────────────────────────────────────────────────────

func TestValidateCRMigrate_PruneWarning(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
		Prune: true,
	})
	require.NoError(t, c.validateMigrate())

	w := crdPtr(c, "app").Warnings
	require.True(t, w.HasWarnings())
	assert.Contains(t, w.String(), "migrate.prune")
	assert.Contains(t, w.String(), "destructive")
}

// ── plural warning ────────────────────────────────────────────────────────────

func TestValidateCRMigrate_NoPluralWarning(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
	})
	require.NoError(t, c.validateMigrate())

	w := crdPtr(c, "app").Warnings
	require.True(t, w.HasWarnings())
	assert.Contains(t, w.String(), "migrate.to.plural is not declared")
	assert.Contains(t, w.String(), "explicitly if the destination's plural differs")
}

func TestValidateCRMigrate_PluralNoWarning(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Plural:     "apps",
		},
		Prune: false,
	})
	require.NoError(t, c.validateMigrate())

	w := crdPtr(c, "app").Warnings
	require.False(t, w.HasWarnings())
}

func TestValidateCRMigrate_NoPruneNoWarning(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Plural:     "apps",
		},
		Prune: false,
	})
	require.NoError(t, c.validateMigrate())
	assert.False(t, crdPtr(c, "app").Warnings.HasWarnings())
}

// ── invalid declaration does not also warn ───────────────────────────────────

func TestValidateCRMigrate_InvalidDoesNotWarn(t *testing.T) {
	// A broken declaration fails; prune on a broken declaration must not
	// produce a warning, because the warning is about a config that will
	// actually run.
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			Kind: "App", // APIVersion missing
		},
		Prune: true,
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.False(t, crdPtr(c, "app").Warnings.HasWarnings())
}

// ── metadata preservation does not require validation ────────────────────────

func TestValidateCRMigrate_PreserveSourceDoesNotAffectValidity(t *testing.T) {
	// PreserveSource is a *bool with a defined default and no invalid values,
	// so it must never cause validateMigrate to fail, regardless of setting.
	for _, tc := range []struct {
		name  string
		value *bool
	}{
		{"unset", nil},
		{"true", ptr.To(true)},
		{"false", ptr.To(false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := blueprintWithMigrate(&types.CRMigrate{
				To: types.CRMigrateTarget{
					APIVersion: "test.migrate/v1",
					Kind:       "App",
					Metadata: types.CRMigrateMetadata{
						PreserveSource: tc.value,
					},
				},
			})
			require.NoError(t, c.validateMigrate())
		})
	}
}

// ── both invalid at once ─────────────────────────────────────────────────────

func TestValidateCRMigrate_EmptyTo(t *testing.T) {
	// An empty migrate.to declares no destination: with no field-level
	// migration either, there is nothing to migrate.
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "nothing to migrate")
}

// ── conflicting migrations ───────────────────────────────────────────────────

func TestValidateCRMigrate_ConflictsWithFieldMigration(t *testing.T) {
	c := blueprintWithMigrateAndField(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
	}, &types.ServeConfig{
		Enabled: true,
		Fields: map[string]types.ServeFieldConfig{
			"timeout": {
				Path: "caller.timeout",
				Migrate: &types.ServeFieldMigrate{
					From: "timeoutSeconds",
				},
			},
		},
	})

	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "cannot declare both a CR-level migration")
	assert.ErrorContains(t, err, "serve.fields.timeout.migrate.from")
	assert.ErrorContains(t, err, "Run the field migration first")
}

// ── destination namespace ────────────────────────────────────────────────────

func TestValidateCRMigrate_ValidTargetNamespace(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Namespace:  "platform-01",
		},
	})
	require.NoError(t, c.validateMigrate())
}

func TestValidateCRMigrate_InvalidTargetNamespace(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Namespace:  "Invalid_Namespace!",
		},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.to.namespace")
	assert.ErrorContains(t, err, "Invalid_Namespace!")
}

func TestValidateCRMigrate_EmptyTargetNamespaceAllowed(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
	})
	require.NoError(t, c.validateMigrate())
}

// ── selection: name ──────────────────────────────────────────────────────────

func TestValidateCRMigrate_ValidSelectionName(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
		Selection: types.MigrateSelection{
			Name: "my-app",
		},
	})
	require.NoError(t, c.validateMigrate())
}

func TestValidateCRMigrate_InvalidSelectionName(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
		Selection: types.MigrateSelection{
			Name: "Invalid_Name!",
		},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.selection.name")
}

// ── selection: namespaces ────────────────────────────────────────────────────

func TestValidateCRMigrate_ValidSelectionNamespaces(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
		Selection: types.MigrateSelection{
			Namespaces: []string{"platform-01", "platform-02"},
		},
	})
	require.NoError(t, c.validateMigrate())
}

func TestValidateCRMigrate_InvalidSelectionNamespace(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
		Selection: types.MigrateSelection{
			Namespaces: []string{"platform-01", "Invalid_Namespace!", "platform-03"},
		},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.selection.namespaces[1]")
	assert.ErrorContains(t, err, "Invalid_Namespace!")
}

func TestValidateCRMigrate_EmptySelectionNamespacesAllowed(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
		Selection: types.MigrateSelection{
			Namespaces: []string{},
		},
	})
	require.NoError(t, c.validateMigrate())
}

// ── selection: selector ──────────────────────────────────────────────────────

func TestValidateCRMigrate_ValidSelector(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
		},
		Selection: types.MigrateSelection{
			Selector: "app=nginx,tier in (gold,silver)",
		},
	})
	require.NoError(t, c.validateMigrate())
}

// func TestValidateCRMigrate_InvalidSelector(t *testing.T) {
// 	c := blueprintWithMigrate(&types.CRMigrate{
// 		To: types.CRMigrateTarget{
// 			APIVersion: "t

// ── reserved domain ──────────────────────────────────────────────────────────

func TestValidateCRMigrate_ReservedLabelRejected(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Labels: map[string]string{"inrun.dev/managed": "true"},
			},
		},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "reserved")
}

func TestValidateCRMigrate_ReservedAnnotationRejected(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1",
			Kind:       "App",
			Metadata: types.CRMigrateMetadata{
				Annotations: map[string]string{"sub.inrun.dev/x": "y"},
			},
		},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "reserved")
}

// ── scope ────────────────────────────────────────────────────────────────────

func writeCRD(t *testing.T, kind, scope string) string {
	t.Helper()
	body := fmt.Sprintf(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: %[2]ss.test.migrate
spec:
  group: test.migrate
  scope: %[1]s
  names:
    kind: %[3]s
    plural: %[2]ss
  versions:
    - name: v1
      served: true
      storage: true
`, scope, strings.ToLower(kind), kind)
	path := filepath.Join(t.TempDir(), kind+".yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func scopeChecker(t *testing.T, srcNamespaced *bool, srcScope, dstScope, toNamespace string) *checker {
	t.Helper()
	entry := types.CRDEntry{
		Namespaced: srcNamespaced,
		// Parse has already filled these from migrate.to.crdFile.
		Migrate: &types.CRMigrate{To: types.CRMigrateTarget{
			APIVersion:      "test.migrate/v1",
			Kind:            "Dest",
			Plural:          "dests",
			Namespace:       toNamespace,
			ResolvedCRDFile: writeCRD(t, "Dest", dstScope),
		}},
	}
	if srcScope != "" {
		entry.ResolvedCRDFile = writeCRD(t, "Src", srcScope)
	}
	return newBlueprintExec(map[string]types.CRDEntry{"app": entry})
}

func TestValidateCRMigrate_Scope_ClusterToNamespacedNeedsNamespace(t *testing.T) {
	c := scopeChecker(t, ptr.To(false), "", "Namespaced", "")
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.to.namespace is required")
}

func TestValidateCRMigrate_Scope_ClusterToNamespacedWithNamespace(t *testing.T) {
	c := scopeChecker(t, ptr.To(false), "", "Namespaced", "platform")
	require.NoError(t, c.validateMigrate())
	assert.False(t, crdPtr(c, "app").Warnings.Contains("ignored"))
}

func TestValidateCRMigrate_Scope_SourceFileWinsOverDefault(t *testing.T) {
	// namespaced: is unset (default true), but the source crdFile says Cluster.
	c := scopeChecker(t, nil, "Cluster", "Namespaced", "")
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.to.namespace is required")
}

func TestValidateCRMigrate_Scope_NamespacedToClusterWarns(t *testing.T) {
	c := scopeChecker(t, nil, "", "Cluster", "")
	require.NoError(t, c.validateMigrate())
	assert.True(t, crdPtr(c, "app").Warnings.Contains("collisions"))
}

func TestValidateCRMigrate_Scope_IgnoredNamespaceWarns(t *testing.T) {
	c := scopeChecker(t, nil, "", "Namespaced", "platform")
	require.NoError(t, c.validateMigrate())
	assert.True(t, crdPtr(c, "app").Warnings.Contains("migrate.to.namespace is ignored"))
}

func TestValidateCRMigrate_Scope_NamespacedToNamespacedQuiet(t *testing.T) {
	c := scopeChecker(t, nil, "", "Namespaced", "")
	require.NoError(t, c.validateMigrate())
	w := crdPtr(c, "app").Warnings
	assert.False(t, w.Contains("collisions"))
	assert.False(t, w.Contains("ignored"))
}

func TestValidateCRMigrate_Scope_BadScope(t *testing.T) {
	c := scopeChecker(t, nil, "", "Global", "")
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "must be Namespaced or Cluster")
}

// ── destination and conditions ──────────────────────────────────────────────

func TestValidateCRMigrate_DestinationIsSource(t *testing.T) {
	c := newBlueprintExec(map[string]types.CRDEntry{"app": {
		APITypes: types.APITypes{Group: "test.migrate", Version: "v1", Kind: "App", Plural: "apps"},
		Migrate:  &types.CRMigrate{To: types.CRMigrateTarget{APIVersion: "test.migrate/v2", Kind: "App"}},
	}})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "names the source itself")
}

func TestValidateCRMigrate_BadSelectionCondition(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{APIVersion: "test.migrate/v1", Kind: "App"},
		Selection: types.MigrateSelection{
			When: []types.Condition{{Field: "spec.tier", Equals: "a", NotEquals: "b"}},
		},
	})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.selection condition on spec.tier")
}

func TestValidateCRMigrate_GoodSelectionCondition(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{
		To: types.CRMigrateTarget{APIVersion: "test.migrate/v1", Kind: "App"},
		Selection: types.MigrateSelection{
			Or: []types.Condition{{Field: "status.phase", Equals: "Ready"}},
		},
	})
	require.NoError(t, c.validateMigrate())
}

// ── schema compatibility ─────────────────────────────────────────────────────

func writeSchemaCRD(t *testing.T, kind, specProps string) string {
	t.Helper()
	body := fmt.Sprintf(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: %[1]ss.test.migrate
spec:
  group: test.migrate
  scope: Namespaced
  names:
    kind: %[2]s
    plural: %[1]ss
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
%[3]s`, strings.ToLower(kind), kind, specProps)
	path := filepath.Join(t.TempDir(), kind+".yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func schemaChecker(t *testing.T, srcProps, dstProps string) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{"app": {
		ResolvedCRDFile: writeSchemaCRD(t, "Src", srcProps),
		Migrate: &types.CRMigrate{To: types.CRMigrateTarget{
			APIVersion: "test.migrate/v1", Kind: "Dest", Plural: "dests",
			ResolvedCRDFile: writeSchemaCRD(t, "Dest", dstProps),
		}},
	}})
}

const props = `                image:
                  type: string
                replicas:
                  type: integer
`

func TestValidateCRMigrate_SchemaCompatible(t *testing.T) {
	c := schemaChecker(t, props, props+`                extra:
                  type: boolean
`)
	require.NoError(t, c.validateMigrate())
}

func TestValidateCRMigrate_SchemaMissingField(t *testing.T) {
	c := schemaChecker(t, props, `                image:
                  type: string
`)
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "Dest cannot hold every source field")
	assert.ErrorContains(t, err, "spec.replicas: missing in destination")
}

func TestValidateCRMigrate_SchemaTypeChange(t *testing.T) {
	c := schemaChecker(t, props, `                image:
                  type: string
                replicas:
                  type: string
`)
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "spec.replicas: source is integer, destination is string")
}

// ── selection-only (field level) ─────────────────────────────────────────────

func fieldServe() *types.ServeConfig {
	return &types.ServeConfig{Enabled: true, Fields: map[string]types.ServeFieldConfig{
		"replicas": {Migrate: &types.ServeFieldMigrate{From: "spec.count"}},
	}}
}

func TestValidateMigrate_SelectionScopesFieldMigration(t *testing.T) {
	c := blueprintWithMigrateAndField(&types.CRMigrate{
		Selection: types.MigrateSelection{Selector: "tier=gold", When: []types.Condition{{Field: "spec.env", Equals: "prod"}}},
	}, fieldServe())
	require.NoError(t, c.validateMigrate())
}

func TestValidateMigrate_SelectionWithNothingToMigrate(t *testing.T) {
	c := blueprintWithMigrate(&types.CRMigrate{Selection: types.MigrateSelection{Selector: "tier=gold"}})
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "nothing to migrate")
}

func TestValidateMigrate_PruneNeedsTo(t *testing.T) {
	c := blueprintWithMigrateAndField(&types.CRMigrate{Prune: true}, fieldServe())
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.prune needs migrate.to")
}

func TestValidateMigrate_BadSelectionOnFieldLevel(t *testing.T) {
	c := blueprintWithMigrateAndField(&types.CRMigrate{
		Selection: types.MigrateSelection{Selector: "=bad"},
	}, fieldServe())
	err := c.validateMigrate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migrate.selection.selector")
}
