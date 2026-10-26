package core

import (
	"context"
	"fmt"
	"slices"
	"strings"

	client "github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Selection is the scope of one run: the CRD's migrate.selection merged with
// the CLI flags.
type Selection struct {
	Namespaces []string
	Name       string
	Selector   string
}

// Selection merges the CRD's declared selection with the CLI flags.
func (r *Runner) Selection(crd *types.CRDEntry) (Selection, error) {
	out := Selection{Namespaces: r.Namespaces, Name: r.Name, Selector: r.Selector}
	if crd.Migrate == nil {
		return out, nil
	}
	sel := crd.Migrate.Selection

	// The CLI can narrow the declared namespaces, never widen them.
	if len(sel.Namespaces) > 0 {
		if len(out.Namespaces) == 0 {
			out.Namespaces = sel.Namespaces
		} else {
			out.Namespaces = structured.Intersect(out.Namespaces, sel.Namespaces)
			if len(out.Namespaces) == 0 {
				return out, fmt.Errorf("--namespace %s is outside migrate.selection.namespaces %s",
					strings.Join(r.Namespaces, ","), strings.Join(sel.Namespaces, ","))
			}
		}
	}

	if sel.Name != "" && r.Name != "" && sel.Name != r.Name {
		return out, fmt.Errorf("migrate.selection declares name %q; CLI requested %q — these cannot both apply", sel.Name, r.Name)
	}
	if sel.Name != "" {
		out.Name = sel.Name
	}

	// Label selectors AND on commas.
	if sel.Selector != "" {
		if out.Selector == "" {
			out.Selector = sel.Selector
		} else {
			out.Selector = out.Selector + "," + sel.Selector
		}
	}
	return out, nil
}

// Admits reports whether cr passes the CRD's migrate.selection when/or
// conditions. CRs pass when none are declared.
func (r *Runner) Admits(crd *types.CRDEntry, cr *unstructured.Unstructured) bool {
	if crd.Migrate == nil || !crd.Migrate.Selection.HasConditions() {
		return true
	}
	sel := crd.Migrate.Selection
	resolver := template.NewResolverFromMap(cr.Object).WithUserFunctions(r.Functions)
	return types.EvaluateConditions(cr.Object, sel.When, sel.Or, resolver.TemplateEvaluator())
}

// Includes reports whether a CR with this identity and labels is in the
// selection. Used where the API server cannot filter, such as recorded
// sources in a snapshot.
func (s Selection) Includes(name, namespace string, lbls map[string]string) (bool, error) {
	if s.Name != "" && name != s.Name {
		return false, nil
	}
	if len(s.Namespaces) > 0 && namespace != "" && !slices.Contains(s.Namespaces, namespace) {
		return false, nil
	}
	matcher, err := k8slabels.Parse(s.Selector)
	if err != nil {
		return false, fmt.Errorf("parse selector %q: %w", s.Selector, err)
	}
	return matcher.Matches(k8slabels.Set(lbls)), nil
}

// IsEmpty reports whether the selection matches everything.
func (s Selection) IsEmpty() bool {
	return len(s.Namespaces) == 0 && s.Name == "" && s.Selector == ""
}

// Equal reports whether two selections are the same.
func (s Selection) Equal(o Selection) bool {
	return s.Name == o.Name && s.Selector == o.Selector && slices.Equal(s.Namespaces, o.Namespaces)
}

// ListSources lists the CRs of resource in sel.
func (r *Runner) ListSources(
	ctx context.Context,
	sel Selection,
	gvk schema.GroupVersionKind,
	resource dynamic.NamespaceableResourceInterface,
	clusterScoped bool,
	log zerolog.Logger,
) ([]unstructured.Unstructured, error) {
	if clusterScoped && len(sel.Namespaces) > 0 {
		log.Warn().Strs("namespaces", sel.Namespaces).
			Msg("--namespace ignored: CRD is cluster-scoped")
	}

	opts := metav1.ListOptions{Limit: r.PageSize, LabelSelector: sel.Selector}
	if sel.Name != "" {
		opts.FieldSelector = "metadata.name=" + sel.Name
	}

	items, err := client.ListAllForNamespaces(ctx, resource, clusterScoped, sel.Namespaces, opts)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", gvk.Kind, err)
	}
	return items, nil
}
