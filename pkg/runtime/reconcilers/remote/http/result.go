package http

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/remote/contract"
)

// toResult converts a contract.Result into a domain.Result.
func (r *Reconciler) toResult(ctx context.Context, req domain.Request, res contract.Result) (domain.Result, error) {
	switch res.Result {
	case "ok", "":
	case "error":
		msg := res.Error
		if msg == "" {
			msg = "remote reconciler returned result=error with no message"
		}
		logger.FromContext(ctx).Error().
			Str("reconciler", "remote").
			Str("key", req.Key).
			Str("endpoint", r.decl.Endpoint).
			Msg(msg)
		return domain.Result{}, fmt.Errorf("%s", msg)
	default:
		return domain.Result{}, fmt.Errorf("remote reconciler: unknown result %q (use \"ok\" or \"error\"; request a requeue with the requeue field)", res.Result)
	}

	return res.Directives, nil
}
