package runners

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	kwtypes "github.com/intentd/kubewright/core/types"
)

// nsGuard reports whether owner may write into namespace ns. A nil guard
// allows every namespace.
type nsGuard = func(ctx context.Context, owner domain.Object, ns string) bool

// child describes how one kind of child resource is written: how to read a
// template entry, how to build its kubewright spec, and which kubewright verbs
// write it. runChildren does the rest the same way for every kind.
type child[S, Spec any] struct {
	// log names the kind in log lines ("deployment"); resource is its
	// Kubernetes kind ("Deployment"); field is its Blueprint field
	// ("deployments"), which prefixes errors so they point at the entry.
	log, resource, field string

	entry   func(S) entryFields
	resolve func(*template.Resolver, S) (S, error)
	spec    func(S, domain.Object, *types.PresetRegistry) Spec

	create func(context.Context, kubernetes.Interface, kwtypes.Owner, Spec) (kwtypes.Result, error)
	// apply is nil for kinds that are only ever created (Namespace, ServiceAccount).
	apply func(context.Context, kubernetes.Interface, kwtypes.Owner, Spec, kwtypes.Options) (kwtypes.Result, error)
	// deleteIfOwned removes an entry's object once its conditions stop
	// passing. Cluster-scoped kinds ignore the namespace.
	deleteIfOwned func(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (kwtypes.Result, error)

	// clusterScoped kinds have no namespace: no namespace guard, and entries
	// are matched by name alone.
	clusterScoped bool
	// applyOnReconcile writes an onCreate entry with reconcile: true by Apply
	// alone, rather than Create and then Apply (Role, RoleBinding).
	applyOnReconcile bool

	// toNamespaces, when it returns names, writes the object into each of
	// those namespaces instead of the owner's; inNamespace sets a spec's
	// namespace for that.
	toNamespaces func(S) []string
	inNamespace  func(Spec, string) Spec

	// before runs once the spec is built, before it is written (Ingress TLS).
	before func(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec, ns string) error
	// after runs once the object is written (workload autoscale).
	after func(ctx context.Context, kube kubeclient.Interface, resolver *template.Resolver, owner domain.Object, src S, name, ns string) error
}

// entryFields are the fields every template entry has, whatever its kind.
type entryFields struct {
	Name, Namespace string
	When, Or        []types.Condition
	Reconcile       bool
	ForceConflict   *bool
}

// runChildren resolves and writes srcs, the template entries of one kind.
//
// update=false is the onCreate path: Create, which leaves an existing object
// alone. update=true is the onReconcile path: Apply, which corrects drift.
// reconcile: true on an onCreate entry means create it and keep it in sync,
// without a separate onReconcile entry.
//
// An entry whose conditions don't pass deletes the object it would write, if
// this owner made it, unless another entry still writes that name.
func runChildren[S, Spec any](
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []S,
	update bool,
	guard nsGuard,
	k child[S, Spec],
) error {
	passes := func(e entryFields) bool {
		return types.EvaluateConditions(resolver.Data(), e.When, e.Or, resolver.TemplateEvaluator())
	}
	target := func(e entryFields) (name, ns string) {
		name, _ = resolver.Resolve(e.Name)
		if k.clusterScoped {
			return name, ""
		}
		ns, _ = resolver.Resolve(e.Namespace)
		if ns == "" {
			ns = owner.GetNamespace()
		}
		return name, ns
	}

	// Names some entry still writes, so a failing entry doesn't delete them.
	active := make(map[string]bool, len(srcs))
	for _, s := range srcs {
		if e := k.entry(s); passes(e) {
			name, ns := target(e)
			active[ns+"/"+name] = true
		}
	}

	for i, src := range srcs {
		e := k.entry(src)
		passed := passes(e)
		name, ns := target(e)

		// The guard needs the resolved namespace which is now the case.
		if !k.clusterScoped && guard != nil && !guard(ctx, owner, ns) {
			continue // skipped: the guard has logged why
		}

		logger.FromContext(ctx).Debug().
			Str("resource", k.resource).
			Str("name", name).
			Str("namespace", ns).
			Bool("namespace_restricted", guard != nil).
			Int("index", i).
			Bool("condition_passed", passed).
			Msg(k.log + ": condition evaluation")

		if !passed {
			if (update || e.Reconcile) && !active[ns+"/"+name] {
				res, err := k.deleteIfOwned(ctx, kube.Clientset(), name, ns, labels.OwnedBy(owner))
				if err != nil {
					return fmt.Errorf("%s[%d]: conditional cleanup: %w", k.field, i, err)
				}
				logResult(k.log, name, ns, owner, res)
			}
			continue
		}

		resolved, err := k.resolve(resolver, src)
		if err != nil {
			return fmt.Errorf("%s[%d]: %w", k.field, i, err)
		}
		spec := k.spec(resolved, owner, resolver.Presets())

		if k.before != nil {
			if err := k.before(ctx, kube, owner, spec, ns); err != nil {
				return fmt.Errorf("%s[%d]: %w", k.field, i, err)
			}
		}

		if k.toNamespaces != nil {
			if listed := k.toNamespaces(resolved); len(listed) > 0 {
				if err := k.writeToNamespaces(ctx, kube, resolver, owner, spec, e, listed, update, guard, i); err != nil {
					return err
				}
				continue // written to the listed namespaces instead of the owner's
			}
		}

		if err := k.write(ctx, kube, owner, spec, e, name, ns, update, i); err != nil {
			return err
		}

		if k.after != nil {
			if err := k.after(ctx, kube, resolver, owner, src, name, ns); err != nil {
				return fmt.Errorf("%s[%d]: %w", k.field, i, err)
			}
		}
	}
	return nil
}

// write creates or applies one object, as the entry and path ask.
func (k child[S, Spec]) write(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec, e entryFields, name, ns string, update bool, i int) error {
	do := func(step string, apply bool) error {
		var res kwtypes.Result
		var err error
		if apply {
			res, err = k.apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, e.ForceConflict))
		} else {
			res, err = k.create(ctx, kube.Clientset(), owner, spec)
		}
		if err != nil {
			return fmt.Errorf("%s[%d].%s: %w", k.field, i, step, err)
		}
		logResult(k.log, name, ns, owner, res)
		return nil
	}

	switch {
	case k.apply == nil:
		return do("create", false)
	case update, k.applyOnReconcile && e.Reconcile:
		return do("update", true)
	}
	if err := do("create", false); err != nil {
		return err
	}
	if e.Reconcile {
		return do("reconcile", true)
	}
	return nil
}

// writeToNamespaces writes the object into each listed namespace the guard
// allows: Apply when it is kept in sync, Create otherwise.
func (k child[S, Spec]) writeToNamespaces(ctx context.Context, kube kubeclient.Interface, resolver *template.Resolver, owner domain.Object, spec Spec, e entryFields, listed []string, update bool, guard nsGuard, i int) error {
	namespaces, err := resolver.ResolveStringSlice(listed)
	if err != nil {
		return fmt.Errorf("%s[%d].toNamespaces: %w", k.field, i, err)
	}
	name, _ := resolver.Resolve(e.Name)
	sync := update || e.Reconcile
	for _, ns := range namespaces {
		if guard != nil && !guard(ctx, owner, ns) {
			continue // restricted: skip it, carry on with the others
		}
		nsSpec := k.inNamespace(spec, ns)
		var res kwtypes.Result
		step := "create"
		if sync {
			step = "update"
			res, err = k.apply(ctx, kube.Clientset(), owner, nsSpec, kubeclient.ApplyOptions(kube, e.ForceConflict))
		} else {
			res, err = k.create(ctx, kube.Clientset(), owner, nsSpec)
		}
		if err != nil {
			return fmt.Errorf("%s[%d].%s namespace=%s: %w", k.field, i, step, ns, err)
		}
		logResult(k.log, name, ns, owner, res)
	}
	return nil
}

// clusterDelete adapts a cluster-scoped DeleteIfOwned, which takes no
// namespace, to child.deleteIfOwned.
func clusterDelete(del func(context.Context, kubernetes.Interface, string, func(metav1.Object) bool) (kwtypes.Result, error)) func(context.Context, kubernetes.Interface, string, string, func(metav1.Object) bool) (kwtypes.Result, error) {
	return func(ctx context.Context, kube kubernetes.Interface, name, _ string, owned func(metav1.Object) bool) (kwtypes.Result, error) {
		return del(ctx, kube, name, owned)
	}
}

// noPresets adapts a spec builder that takes no preset registry to child.spec.
func noPresets[S, Spec any](build func(S, domain.Object) Spec) func(S, domain.Object, *types.PresetRegistry) Spec {
	return func(src S, owner domain.Object, _ *types.PresetRegistry) Spec { return build(src, owner) }
}
