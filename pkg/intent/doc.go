// Package intent turns an intent into an object and reads it back.
//
// An intent is a flat payload in the caller's own vocabulary, with no
// apiVersion, kind or spec. BuildCR maps a gateway intent onto a CRD using the
// CRD's serve declaration. BuildResource maps a resource intent, which a
// remote reconciler returns, onto a built-in kind such as a Deployment; the
// builders for each kind are in package resources. FromObject and Target read
// an intent and its target back from a stored resource, so the runtime can
// reconcile it per target.
package intent
