// Package kubefake is the one fake Kubernetes client Inrun uses, in simulate
// and in tests: nothing else in pkg imports client-go's fakes.
//
// Kubeclient (New) implements kubeclient.Interface in memory and records every
// operation (Ops), which is what inrun simulate replays and asserts on. For
// code that takes a plain clientset or dynamic client, NewClientset and
// NewDynamic return client-go's fakes, and the Action types write reactors.
package kubefake
