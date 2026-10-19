// Package reconcilers builds the reconciler a CRD gets. Its sub-packages hold
// the kinds: generic (hooks and declarative templates), remote, and mux, which
// routes CRs on serve targets to their own constructor or remote reconciler.
// The runtime and inrun simulate both call Factory, so a CRD is reconciled the
// same way in both.
package reconcilers

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/generic"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/mux"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/remote"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// Deps are what a reconciler may need from the runtime.
type Deps struct {
	Kube       kubeclient.Interface
	Informer   cache.SharedIndexInformer
	Event      event.Recorder
	StoreFor   func(schema.GroupVersionKind) cache.Store
	IndexerFor func(schema.GroupVersionKind) cache.Indexer
	Blueprint  *blueprint.Blueprint
	// OwnNamespace is where a remote reconciler's secretRef without a
	// namespace resolves.
	OwnNamespace string
}

// Factory returns the factory for crd's reconciler; each call builds a fresh
// one for one worker. The kind is crd.ReconcilerKind(): declarative and hooks
// use the generic reconciler, remote a remote one, constructor its compiled
// constructor. CRDs whose serve targets declare
// their own constructor or remote reconciler are wrapped in the mux.
func Factory(crd types.CRDEntry, d Deps) (func() domain.Reconciler, error) {
	base, err := baseFactory(crd, d)
	if err != nil {
		return nil, err
	}
	if !crd.HasTargetConstructorFactories() && !crd.HasTargetRemoteDeclarations() {
		return base, nil
	}
	ctors, err := crd.TargetConstructors()
	if err != nil {
		return nil, err
	}
	remoteDecls := crd.TargetRemoteDeclarations()
	newRemotes := make(map[string]func() domain.Reconciler, len(remoteDecls))
	for name, decl := range remoteDecls {
		newRemote, err := remoteFactory(crd, decl, crd.EffectiveOperatorBox(name), d)
		if err != nil {
			return nil, fmt.Errorf("serve target %q: %w", name, err)
		}
		newRemotes[name] = newRemote
	}
	return func() domain.Reconciler {
		targets := make(map[string]domain.Reconciler, len(ctors)+len(newRemotes))
		for name, ctor := range ctors {
			targets[name] = ctor(constructorKube(crd, d))
		}
		for name, newRemote := range newRemotes {
			targets[name] = newRemote()
		}
		return mux.New(d.Informer, targets, base())
	}, nil
}

// baseFactory is the factory for CRs on no serve target, or one without its
// own reconciler.
func baseFactory(crd types.CRDEntry, d Deps) (func() domain.Reconciler, error) {
	switch crd.ReconcilerKind() {
	case types.KindDeclarative, types.KindHooks:
		return genericFactory(crd, d, nil)

	case types.KindRemote:
		return remoteFactory(crd, crd.Box().Reconcile.Remote, crd.Box(), d)

	default:
		ctor, ok := crd.Constructor()
		if !crd.ConstructorEnabled() || !ok {
			return nil, fmt.Errorf("reconcile.default is false but no constructor is registered for %s", crd.GVKString())
		}
		// Constructor authors reach the informer and event recorder through
		// kube (kube.GetInformer() and so on); args arrive on each request.
		kube := constructorKube(crd, d)
		return func() domain.Reconciler {
			return ctor(kube)
		}, nil
	}
}

// remoteFactory is the factory for a remote reconciler declared in box (the
// CRD's or a serve target's). A mapped remote with templates in box becomes
// the generic reconciler's hook: its call drives the Result, and the
// templates run after it with .response in scope.
func remoteFactory(crd types.CRDEntry, decl *types.RemoteReconcilerDeclaration, box *types.OperatorBoxConfig, d Deps) (func() domain.Reconciler, error) {
	// remote.New fails only on an unsupported protocol; check it once here.
	if _, err := remote.New(decl, crd.GVK(), d.Kube, d.Event, crd.ManagedResources(), d.OwnNamespace); err != nil {
		return nil, err
	}
	kube := remoteKube(d)
	if decl.Mapped() && box.HasTemplates() {
		return genericFactory(crd, d, func() domain.ObjectHooks {
			r, _ := remote.New(decl, crd.GVK(), kube, d.Event, crd.ManagedResources(), d.OwnNamespace)
			return r.Hooks()
		})
	}
	return func() domain.Reconciler {
		r, _ := remote.New(decl, crd.GVK(), kube, d.Event, crd.ManagedResources(), d.OwnNamespace)
		return r
	}, nil
}

// genericFactory is the factory for the generic reconciler: the CRD's
// templates, and its hooks when registered. call, when set, gives the hooks
// instead: a mapped remote's call, run before the templates.
func genericFactory(crd types.CRDEntry, d Deps, call func() domain.ObjectHooks) (func() domain.Reconciler, error) {
	var hooks domain.AnyReconcileHooks
	if factory, ok := crd.HookFactory(); ok {
		hooks = factory()
	}
	// Hooks read reconcile.args with req.Args(), like every reconciler.
	kube := d.Kube.WithForceConflict(crd.ResolveForceConflict())
	object, _, err := crd.RuntimeObjects()
	if err != nil {
		return nil, err
	}
	return func() domain.Reconciler {
		r := generic.New(crd, d.Informer, kube, hooks,
			func() domain.Object { return object.DeepCopyObject().(domain.Object) },
			d.Blueprint)
		if call != nil {
			return r.WithCall(call())
		}
		return r
	}, nil
}

// remoteKube is the kube client a remote reconciler gets: it reads children
// from the informer stores.
func remoteKube(d Deps) kubeclient.Interface {
	if d.StoreFor == nil {
		return d.Kube
	}
	return d.Kube.WithStoreFor(d.StoreFor)
}

// constructorKube is the kube client a constructor gets: the informer, event
// recorder, stores and indexers, and the CRD's force-conflict setting.
func constructorKube(crd types.CRDEntry, d Deps) kubeclient.Interface {
	kube := d.Kube.WithInformer(d.Informer).WithEventRecorder(d.Event)
	if d.StoreFor != nil {
		kube = kube.WithStoreFor(d.StoreFor)
	}
	if d.IndexerFor != nil {
		kube = kube.WithIndexerFor(d.IndexerFor)
	}
	return kube.WithForceConflict(crd.ResolveForceConflict())
}
