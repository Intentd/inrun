package reconcile

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/external"

	"github.com/intentd/inrun/pkg/types"
)

// validateRemoteMapping checks the parts of reconcile.remote that let a
// service keep its own shape: request.method, request.body and
// request.finalize, and the response mapping.
func (c *checker) validateRemoteMapping(name string, crd *types.CRDEntry, remote *types.RemoteReconcilerDeclaration) error {
	at := func(field string) string { return fmt.Sprintf("CRD %q: reconcile.remote.%s", name, field) }
	funcMap := buildFuncMapForValidation(c.Blueprint.Functions)
	body := func(field, value string) error {
		if !isTemplate(value) {
			return nil
		}
		return validateTemplate("reconcile.remote", name, name, field, value, funcMap)
	}

	if rq := remote.Request; rq != nil {
		if err := c.checkCall(crd, at, "request", rq.CallSpec, body); err != nil {
			return err
		}
		if !remote.Mapped() {
			needsResponse := func(field, why string) error {
				return fmt.Errorf("%s %s needs reconcile.remote.response: %s", failureMark(), at("request."+field), why)
			}
			switch {
			case len(rq.When) > 0 || len(rq.Or) > 0:
				return needsResponse("when/or", "a remote that answers in Inrun's Result is called on every reconcile")
			case len(rq.ExpectedStatus) > 0:
				return needsResponse("expectedStatus", "a remote that answers in Inrun's Result always replies 200")
			case rq.URL != "":
				return needsResponse("url", "a remote that answers in Inrun's Result is called at remote.endpoint")
			case rq.Timeout != "":
				return needsResponse("timeout", "a remote that answers in Inrun's Result uses remote.timeout")
			case rq.Auth != nil:
				return needsResponse("auth", "a remote that answers in Inrun's Result uses remote.auth")
			case rq.Finalize != nil:
				return needsResponse("finalize", "a remote that answers in Inrun's Result handles deletion itself (finalizing: true)")
			}
		}
		if rq.Finalize != nil {
			if err := c.checkCall(crd, at, "request.finalize", *rq.Finalize, body); err != nil {
				return err
			}
		}
	}

	resp := remote.Response
	if resp == nil {
		return nil
	}
	if (resp.Status == nil || len(resp.Status.Fields) == 0) && resp.Requeue == nil {
		crd.Info.AddInfo(fmt.Sprintf("CRD %q: reconcile.remote.response maps nothing: the service's answer is "+
			"only checked against expectedStatus", name))
	}
	if resp.Status != nil {
		for i, f := range resp.Status.Fields {
			if f.Path == "" {
				return fmt.Errorf("%s %s[%d].path is required", failureMark(), at("response.status.fields"), i)
			}
			if err := body(fmt.Sprintf("response.status.fields[%d].value", i), f.Value); err != nil {
				return err
			}
		}
	}
	if resp.Requeue != nil {
		if err := checkRequeueAfter(at("response.requeue.after"), resp.Requeue.After); err != nil {
			return err
		}
	}
	return nil
}

// checkCall checks one of the remote's calls (request or finalize): the
// fields every call has, its templates and conditions. Credentials belong in
// auth, and an idempotency key on a method HTTP already makes safe to repeat
// does nothing; both are warnings.
func (c *checker) checkCall(crd *types.CRDEntry, at func(string) string, field string, call types.CallSpec,
	template func(field, value string) error) error {
	if err := external.ValidateCallSpec(at(field), call); err != nil {
		return fmt.Errorf("%s %w", failureMark(), err)
	}
	if call.RetryBackoff != nil {
		return fmt.Errorf("%s %s does not apply to the remote's calls: a failed call fails the reconcile, which is retried with the reconcile's backoff",
			failureMark(), at(field+".retryBackoff"))
	}
	for _, f := range []struct{ name, value string }{{"url", call.URL}, {"body", call.Body}} {
		if err := template(field+"."+f.name, f.value); err != nil {
			return err
		}
	}
	for name, value := range call.Headers {
		if err := template(field+".headers."+name, value); err != nil {
			return err
		}
		if strings.EqualFold(name, "Authorization") {
			crd.Warnings.AddWarning(fmt.Sprintf("%s sets Authorization: put credentials in auth, so they are read from a Secret and never rendered into logs",
				at(field+".headers")))
		}
	}
	if k := call.IdempotencyKey; k != nil && !k.Disabled {
		if err := template(field+".idempotencyKey.value", k.Value); err != nil {
			return err
		}
		if m := strings.ToUpper(call.Method); m != "" && m != "POST" && m != "PATCH" {
			crd.Warnings.AddWarning(fmt.Sprintf("%s has no effect on %s: the key is sent only on POST and PATCH",
				at(field+".idempotencyKey"), m))
		}
	}
	return checkCallConditions(at(field), call.When, call.Or)
}

// checkCallConditions checks a call's when/or: each condition compares one way.
func checkCallConditions(where string, when, or []types.Condition) error {
	for _, cond := range append(append([]types.Condition{}, when...), or...) {
		if err := cond.CheckComparison(); err != nil {
			return fmt.Errorf("%s %s condition on %s %v", failureMark(), where, cond.Field, err)
		}
	}
	return nil
}

// checkStatusCodes checks that every code in an expectedStatus list is an
// HTTP status code.
func checkStatusCodes(where string, codes []int) error {
	for _, code := range codes {
		if code < 100 || code > 599 {
			return fmt.Errorf("%s %s %d is not an HTTP status code (100–599)", failureMark(), where, code)
		}
	}
	return nil
}

// checkMethod accepts an empty method (the default) or a standard HTTP method.
func checkMethod(where, method string) error {
	switch method {
	case "", "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
		return nil
	}
	return fmt.Errorf("%s %s %q is not a valid HTTP method", failureMark(), where, method)
}
