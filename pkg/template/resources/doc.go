// Package resources resolves a Blueprint's resource templates: each function
// takes a kind's template source, evaluates every expression in it with a
// template.Resolver, and returns the same source with values filled in. The
// runners pass the result to the kind's package in pkg/resources to build and
// apply it.
//
// This is Inrun's side of the line with the resource library: Blueprint
// syntax in, a resource spec out.
package resources
