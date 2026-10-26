package kubectl

import (
	"fmt"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func TestAssertGetError(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		contains string
		wantErr  bool
	}{
		{
			name:     "not found matches",
			output:   `Error from server (NotFound): appconfigs.demo.inrun.dev "api-server" not found`,
			contains: "not found",
		},
		{
			name:     "not found does not match",
			output:   `Error from server (NotFound): appconfigs.demo.inrun.dev "api-server" not found`,
			contains: "forbidden",
			wantErr:  true,
		},
		{
			name:     "forbidden matches explicit assertion",
			output:   `Error from server (Forbidden): access denied`,
			contains: "Forbidden",
		},
		{
			name:     "forbidden does not match",
			output:   `Error from server (Forbidden): access denied`,
			contains: "not found",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := types.E2EKubectlGet{
				Kind: "appconfigs.demo.inrun.dev",
				Name: "api-server",
				Assert: &types.Assertions{TextAssertion: types.TextAssertion{
					Contains: tt.contains,
				}},
			}

			err := assertGetError(
				tt.output,
				fmt.Errorf("kubectl command failed"),
				e,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
