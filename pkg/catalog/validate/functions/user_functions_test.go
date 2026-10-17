package notes

import (
	"testing"

	"github.com/intentd/inrun/pkg/catalog"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithNotes(reg types.NoteRegistry) *checker {
	return newExec(&catalog.Catalog{Notes: reg})
}

func TestValidateUserNotes_Empty(t *testing.T) {
	e := catalogWithNotes(types.NoteRegistry{})
	assert.NoError(t, e.validateUserNotes())
}

func TestValidateUserNotes_Valid(t *testing.T) {
	e := catalogWithNotes(types.NoteRegistry{Functions: []types.UserDefinedNote{
		{Name: "appImage", Expression: `{{ .spec.image }}`},
		{Name: "fullName", Expression: `{{ .metadata.namespace }}-{{ .metadata.name }}`},
	}})
	assert.NoError(t, e.validateUserNotes())
}

func TestValidateUserNotes_MissingName(t *testing.T) {
	e := catalogWithNotes(types.NoteRegistry{Functions: []types.UserDefinedNote{
		{Name: "", Expression: `{{ .spec.image }}`},
	}})
	err := e.validateUserNotes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

func TestValidateUserNotes_MissingExpression(t *testing.T) {
	e := catalogWithNotes(types.NoteRegistry{Functions: []types.UserDefinedNote{
		{Name: "myNote", Expression: ""},
	}})
	err := e.validateUserNotes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expression must not be empty")
}

func TestValidateUserNotes_Duplicate(t *testing.T) {
	e := catalogWithNotes(types.NoteRegistry{Functions: []types.UserDefinedNote{
		{Name: "myNote", Expression: `{{ .spec.image }}`},
		{Name: "myNote", Expression: `{{ .spec.replicas }}`},
	}})
	err := e.validateUserNotes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate note name")
}

func TestValidateUserNotes_InvalidTemplate(t *testing.T) {
	e := catalogWithNotes(types.NoteRegistry{Functions: []types.UserDefinedNote{
		{Name: "broken", Expression: `{{ .spec.image`},
	}})
	err := e.validateUserNotes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid expression")
}

func TestValidateUserNotes_ShadowBuiltin_NoAck(t *testing.T) {
	// "default" is a built-in note — without shadow: true this warns but does not error
	e := catalogWithNotes(types.NoteRegistry{Functions: []types.UserDefinedNote{
		{Name: "default", Expression: `{{ .spec.image }}`},
	}})
	assert.NoError(t, e.validateUserNotes())
}

func TestValidateUserNotes_ShadowBuiltin_Acked(t *testing.T) {
	e := catalogWithNotes(types.NoteRegistry{Functions: []types.UserDefinedNote{
		{Name: "default", Expression: `{{ .spec.image }}`, Shadow: true},
	}})
	assert.NoError(t, e.validateUserNotes())
}

func TestValidateUserNotes_WithDescription(t *testing.T) {
	e := catalogWithNotes(types.NoteRegistry{Functions: []types.UserDefinedNote{
		{Name: "taggedImage", Description: "full image with tag", Expression: `{{ .spec.image }}:{{ .spec.tag }}`},
	}})
	assert.NoError(t, e.validateUserNotes())
}
