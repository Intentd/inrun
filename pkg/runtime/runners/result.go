package runners

import (
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
	restypes "github.com/intentd/kubewright/core/types"
)

// logResult logs what a write to a child resource did. Created, Replaced and
// Deleted are logged at info; the rest, which change nothing, at debug. An
// empty namespace is the owner's, as the resource packages resolve it.
func logResult(kind, name, namespace string, owner domain.Object, res restypes.Result) {
	if namespace == "" && owner != nil {
		namespace = owner.GetNamespace()
	}

	event := logger.Debug()
	msg := kind + " " + string(res)
	switch res {
	case restypes.Created, restypes.Deleted:
		event = logger.Info()
	case restypes.Replaced:
		event = logger.Info()
		msg = kind + " replaced: an immutable field changed"
	case restypes.Unchanged:
		msg = kind + " already exists — skipping"
	case restypes.NotFound:
		msg = kind + " already deleted — skipping"
	case restypes.NotOwned:
		msg = kind + " not owned — skipping delete"
	}

	event = event.Str(kind, name)
	if namespace != "" {
		event = event.Str("namespace", namespace)
	}
	if owner != nil {
		event = event.Str("owner", owner.GetName())
	}
	event.Msg(msg)
}
