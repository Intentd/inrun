package kubeclient

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/intentd/inrun/pkg/config"
)

type forceConflictClient struct {
	Interface
	forceConflict *bool
}

func (f *forceConflictClient) ForceConflict() *bool { return f.forceConflict }

func TestApplyOptions(t *testing.T) {
	trueValue := true
	falseValue := false

	tests := []struct {
		name     string
		client   Interface
		resource *bool
		want     bool
	}{
		{
			name:     "resource overrides crd",
			client:   &forceConflictClient{forceConflict: &falseValue},
			resource: &trueValue,
			want:     true,
		},
		{
			name:   "crd used when resource unset",
			client: &forceConflictClient{forceConflict: &falseValue},
			want:   false,
		},
		{
			name: "defaults to true",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyOptions(tt.client, tt.resource)
			assert.Equal(t, tt.want, got.Force)
			assert.Equal(t, config.FieldManagerTemplates, got.FieldManager)
		})
	}
}
