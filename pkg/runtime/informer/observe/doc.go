// Package observe watches the secondary resources an operator declares and
// turns relevant changes into enqueue requests for the owning CR. It never
// reconciles anything itself.
package observe
