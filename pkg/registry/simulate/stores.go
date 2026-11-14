package simulate

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/runtime/informer/observe"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/kubeclient"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// fakeStores serves, from the fake cluster, the informer stores the runtime
// keeps for a CRD's watched kinds: its observe.watch entries, with their
// declared indexes, and its managed resources. Each call lists the kind
// afresh, so a read sees every write made earlier in the run.
type fakeStores struct {
	kube    kubeclient.Interface
	watched map[schema.GroupVersionKind]watchedKind
}

// watchedKind is one kind the runtime would watch: where to list it and the
// indexes its informer carries.
type watchedKind struct {
	gvr      schema.GroupVersionResource
	indexers cache.Indexers
}

// newFakeStores resolves crd's watched kinds the way the observer does:
// explicit watch entries first, then managed resources not already covered.
func newFakeStores(kube kubeclient.Interface, bp *blueprint.Blueprint, crd *types.CRDEntry) *fakeStores {
	s := &fakeStores{kube: kube, watched: map[schema.GroupVersionKind]watchedKind{}}
	add := func(mr domain.ManagedResource, indexers cache.Indexers) {
		gvr, ok := bp.ResolveGVR(mr)
		if !ok {
			return
		}
		gvk, ok := bp.ResolveGVK(mr)
		if !ok {
			return
		}
		if _, covered := s.watched[gvk]; covered {
			return
		}
		s.watched[gvk] = watchedKind{gvr: gvr, indexers: indexers}
	}
	for _, w := range crd.WatchEntries() {
		add(w.ToManagedResource(), observe.Indexers(w))
	}
	for _, mr := range crd.AllManagedResources() {
		add(mr, observe.Indexers(types.WatchEntry{}))
	}
	return s
}

// IndexerFor returns an indexer over the current objects of gvk, or nil when
// the runtime would keep no informer for it.
func (s *fakeStores) IndexerFor(gvk schema.GroupVersionKind) cache.Indexer {
	w, ok := s.watched[gvk]
	if !ok {
		return nil
	}
	ul, err := s.kube.DynamicClient().Resource(w.gvr).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil
	}
	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, w.indexers)
	for i := range ul.Items {
		if err := idx.Add(&ul.Items[i]); err != nil {
			return nil
		}
	}
	return idx
}

// StoreFor returns IndexerFor as a store, or nil for an unwatched kind.
func (s *fakeStores) StoreFor(gvk schema.GroupVersionKind) cache.Store {
	if idx := s.IndexerFor(gvk); idx != nil {
		return idx
	}
	return nil
}
