package apply

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManagesMetadata(t *testing.T) {
	u := ownerObj("a", "default")
	assert.False(t, managesMetadata(u))

	// Status writes under the same manager do not count.
	u.SetManagedFields([]metav1.ManagedFieldsEntry{
		{Manager: FieldManager, Operation: metav1.ManagedFieldsOperationApply, Subresource: "status"},
		{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
	})
	assert.False(t, managesMetadata(u))

	u.SetManagedFields(append(u.GetManagedFields(),
		metav1.ManagedFieldsEntry{Manager: FieldManager, Operation: metav1.ManagedFieldsOperationApply}))
	assert.True(t, managesMetadata(u))
}

func TestMetadata_NothingToDo(t *testing.T) {
	// No keys and nothing previously applied: no API call, so a nil client is fine.
	require.NoError(t, Metadata(context.Background(), nil, ownerObj("a", "default"), nil, nil))
}

func TestMetadata_RejectsReserved(t *testing.T) {
	err := Metadata(context.Background(), nil, ownerObj("a", "default"), nil,
		map[string]string{"inrun.dev/cleanup-due": "now"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}
