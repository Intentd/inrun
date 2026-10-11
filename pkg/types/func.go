package types

import (
	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/kubeclient"
)

// NewReconcilerFunc is the constructor signature every custom reconciler must match.
// It lives in pkg/types (not domain) to avoid an import cycle: domain must not
// import kubeclient, but the constructor function takes kubeclient.Interface.
//
// The runtime injects the primary CRD's informer and event recorder into kube
// before calling the constructor — access them via kube.GetInformer() and
// kube.GetEventRecorder(). Args arrive on each request: req.Args().
type NewReconcilerFunc func(kube kubeclient.Interface) domain.Reconciler
