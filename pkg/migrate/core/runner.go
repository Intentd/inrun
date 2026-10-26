// Package core holds what field-level and kind-level migrations share: the
// run settings, results, selection, and the merge-patch write.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/config"
	client "github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// Mode is the operation a run performs.
type Mode int

const (
	// ModeForward migrates CRs to the declared shape.
	ModeForward Mode = iota
	// ModeRollback restores CRs from their snapshots.
	ModeRollback
	// ModeClean removes migration annotations without touching spec.
	ModeClean
)

// String returns the CLI and JSON name of the mode.
func (m Mode) String() string {
	switch m {
	case ModeForward:
		return "forward"
	case ModeRollback:
		return "rollback"
	case ModeClean:
		return "clean"
	default:
		return fmt.Sprintf("MigrateMode(%d)", int(m))
	}
}

// Runner carries the settings of one migrate run.
type Runner struct {
	Kube      client.Interface
	Functions types.KELFunctionRegistry
	Mode      Mode
	DryRun    bool
	// Strict turns per-CR failures into a kind-level error.
	Strict bool
	// PageSize is the List page size; 0 lets the API server decide.
	PageSize int64
	// Namespaces, Name and Selector come from the CLI and are merged with
	// the CRD's declared selection.
	Namespaces []string
	Name       string
	Selector   string
	// Force makes server-side apply take ownership on conflict.
	Force bool
	Log   *zerolog.Logger
	// Progress, when set, is called after each CR with the kind's label
	// ("Old" or "Old → New") and the count so far.
	Progress func(label string, done, total int)
}

// Report calls Progress when it is set.
func (r *Runner) Report(label string, done, total int) {
	if r.Progress != nil {
		r.Progress(label, done, total)
	}
}

// DryRunOpt returns the server-side dry-run option when DryRun is set.
func (r *Runner) DryRunOpt() []string {
	if r.DryRun {
		return []string{metav1.DryRunAll}
	}
	return nil
}

// Patch writes a JSON merge patch. Merge patch is used because it does not
// validate the live object, which may still be in the old shape.
func (r *Runner) Patch(
	ctx context.Context,
	resource dynamic.NamespaceableResourceInterface,
	cr *unstructured.Unstructured,
	patch interface{},
) error {
	body, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}
	opts := metav1.PatchOptions{
		FieldManager: config.FieldManagerGateway,
		DryRun:       r.DryRunOpt(),
	}
	_, err = client.PatchUnstructured(ctx, resource, cr, k8stypes.MergePatchType, body, opts)
	return err
}

// Fail builds the result for a failed CR. It returns the error line to
// collect in strict mode, and logs the error otherwise.
func (r *Runner) Fail(cr *unstructured.Unstructured, err error, log zerolog.Logger) (CRResult, string) {
	result := CRResult{Namespace: cr.GetNamespace(), Name: cr.GetName(), Err: err}
	if r.Strict {
		return result, fmt.Sprintf("%s: %s", CRRef(cr), err)
	}
	log.Warn().Str("cr", CRRef(cr)).Err(err).Msg("migration skipped")
	return result, ""
}

// StrictError joins the collected strict-mode errors, or returns nil.
func StrictError(errs []string) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("migration errors:\n%s", strings.Join(errs, "\n"))
}

// MetaRef formats "namespace/name", or "name" when cluster-scoped.
func MetaRef(namespace, name string) string {
	if namespace != "" {
		return namespace + "/" + name
	}
	return name
}

// CRRef formats a CR as MetaRef does.
func CRRef(cr *unstructured.Unstructured) string {
	return MetaRef(cr.GetNamespace(), cr.GetName())
}
