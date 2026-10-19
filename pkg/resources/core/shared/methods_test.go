package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestResolveOwnerReferences(t *testing.T) {
	owner := &unstructured.Unstructured{}
	owner.SetAPIVersion("demo.inrun.dev/v1alpha1")
	owner.SetKind("Application")
	owner.SetName("my-app")
	owner.SetUID("test-uid")

	refs := ResolveOwnerReferences(owner)

	require.Len(t, refs, 1)
	assert.Equal(t, "demo.inrun.dev/v1alpha1", refs[0].APIVersion)
	assert.Equal(t, "Application", refs[0].Kind)
	assert.Equal(t, "my-app", refs[0].Name)
	assert.Equal(t, types.UID("test-uid"), refs[0].UID)
	assert.True(t, *refs[0].Controller)
	assert.True(t, *refs[0].BlockOwnerDeletion)
}
