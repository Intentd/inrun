package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func TestToDomainObjectNil(t *testing.T) {
	obj, ok := ToDomainObject(nil)
	assert.False(t, ok)
	assert.Nil(t, obj)
}

func TestToDomainObjectTombstone(t *testing.T) {
	obj, ok := ToDomainObject(cache.DeletedFinalStateUnknown{})
	assert.False(t, ok)
	assert.Nil(t, obj)
}

func TestToUnstructuredNil(t *testing.T) {
	obj, ok := ToUnstructured(nil)
	assert.False(t, ok)
	assert.Nil(t, obj)
}

func TestToUnstructuredTombstone(t *testing.T) {
	obj, ok := ToUnstructured(cache.DeletedFinalStateUnknown{})
	assert.False(t, ok)
	assert.Nil(t, obj)
}

func TestToDomainObject(t *testing.T) {
	// Use *unstructured.Unstructured which implements both metav1.Object and runtime.Object
	u := UnstructuredForTest()
	obj, ok := ToDomainObject(u)
	assert.True(t, ok)
	assert.NotNil(t, obj)
	assert.Equal(t, u, obj)
}

func TestToDomainObjectWithTombstone(t *testing.T) {
	// Test with a tombstone containing a valid object
	u := UnstructuredForTest()
	tombstone := cache.DeletedFinalStateUnknown{Obj: u}

	obj, ok := ToDomainObject(tombstone)
	assert.True(t, ok)
	assert.NotNil(t, obj)
	assert.Equal(t, u, obj)
}

func TestUnwrapCacheTombstone(t *testing.T) {
	obj := map[string]interface{}{}
	tombstone := cache.DeletedFinalStateUnknown{Obj: obj}
	assert.Equal(t, obj, UnwrapCacheTombstone(tombstone))
}

func TestToUnstructuredWithTombstone(t *testing.T) {
	u := UnstructuredForTest()
	tombstone := cache.DeletedFinalStateUnknown{Obj: u}

	result, ok := ToUnstructured(tombstone)
	assert.True(t, ok)
	assert.NotNil(t, result)
	assert.Equal(t, u, result)
}

func TestFromCache_ReturnsACopy(t *testing.T) {
	cached := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "resourceVersion": "7"},
	}}
	got, ok := FromCache(cached)
	assert.True(t, ok)
	got.SetAnnotations(map[string]string{"inrun.dev/health": "{}"})
	assert.Nil(t, cached.GetAnnotations(), "writes to the copy never reach the cached object")

	tomb, ok := FromCache(cache.DeletedFinalStateUnknown{Obj: cached})
	assert.True(t, ok)
	assert.NotSame(t, cached, tomb)
}
