package function

import (
	"testing"

	"github.com/intentd/inrun/pkg/catalog"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithFunctions(reg types.KELFunctionRegistry) *checker {
	return newExec(&catalog.Catalog{Functions: reg})
}

func TestValidateUserFunctions_Empty(t *testing.T) {
	e := catalogWithFunctions(types.KELFunctionRegistry{})
	assert.NoError(t, e.validateUserFunctions())
}

func TestValidateUserFunctions_Valid(t *testing.T) {
	e := catalogWithFunctions(types.KELFunctionRegistry{Definitions: []types.UserDefinedFunction{
		{Name: "appImage", Expression: `{{ .spec.image }}`},
		{Name: "fullName", Expression: `{{ .metadata.namespace }}-{{ .metadata.name }}`},
	}})
	assert.NoError(t, e.validateUserFunctions())
}

func TestValidateUserFunctions_MissingName(t *testing.T) {
	e := catalogWithFunctions(types.KELFunctionRegistry{Definitions: []types.UserDefinedFunction{
		{Name: "", Expression: `{{ .spec.image }}`},
	}})
	err := e.validateUserFunctions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

func TestValidateUserFunctions_MissingExpression(t *testing.T) {
	e := catalogWithFunctions(types.KELFunctionRegistry{Definitions: []types.UserDefinedFunction{
		{Name: "myFunction", Expression: ""},
	}})
	err := e.validateUserFunctions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expression must not be empty")
}

func TestValidateUserFunctions_Duplicate(t *testing.T) {
	e := catalogWithFunctions(types.KELFunctionRegistry{Definitions: []types.UserDefinedFunction{
		{Name: "myFunction", Expression: `{{ .spec.image }}`},
		{Name: "myFunction", Expression: `{{ .spec.replicas }}`},
	}})
	err := e.validateUserFunctions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate function name")
}

func TestValidateUserFunctions_InvalidTemplate(t *testing.T) {
	e := catalogWithFunctions(types.KELFunctionRegistry{Definitions: []types.UserDefinedFunction{
		{Name: "broken", Expression: `{{ .spec.image`},
	}})
	err := e.validateUserFunctions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid expression")
}

func TestValidateUserFunctions_ShadowBuiltin_NoAck(t *testing.T) {
	// "default" is a built-in function — without shadow: true this warns but does not error
	e := catalogWithFunctions(types.KELFunctionRegistry{Definitions: []types.UserDefinedFunction{
		{Name: "default", Expression: `{{ .spec.image }}`},
	}})
	assert.NoError(t, e.validateUserFunctions())
}

func TestValidateUserFunctions_ShadowBuiltin_Acked(t *testing.T) {
	e := catalogWithFunctions(types.KELFunctionRegistry{Definitions: []types.UserDefinedFunction{
		{Name: "default", Expression: `{{ .spec.image }}`, Shadow: true},
	}})
	assert.NoError(t, e.validateUserFunctions())
}

func TestValidateUserFunctions_WithDescription(t *testing.T) {
	e := catalogWithFunctions(types.KELFunctionRegistry{Definitions: []types.UserDefinedFunction{
		{Name: "taggedImage", Description: "full image with tag", Expression: `{{ .spec.image }}:{{ .spec.tag }}`},
	}})
	assert.NoError(t, e.validateUserFunctions())
}
