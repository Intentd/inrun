package reconcile

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// validateRemoteMapping checks the parts of reconcile.remote that let a
// service keep its own shape: request.method, request.body and
// request.finalize, and the response mapping.
func (c *checker) validateRemoteMapping(name string, crd *types.CRDEntry, remote *types.RemoteReconcilerDeclaration) error {
	at := func(field string) string { return fmt.Sprintf("CRD %q: reconcile.remote.%s", name, field) }
	funcMap := buildFuncMapForValidation(c.Cat.Notes)
	body := func(field, value string) error {
		if !isTemplate(value) {
			return nil
		}
		return validateTemplate("reconcile.remote", name, name, field, value, funcMap)
	}

	if rq := remote.Request; rq != nil {
		if err := checkMethod(at("request.method"), rq.Method); err != nil {
			return err
		}
		if err := body("request.body", rq.Body); err != nil {
			return err
		}
		if (len(rq.When) > 0 || len(rq.Or) > 0) && !remote.Mapped() {
			return fmt.Errorf("%s %s needs reconcile.remote.response: a remote that answers in Inrun's Result is called on every reconcile",
				failureMark(), at("request.when/or"))
		}
		if err := checkCallConditions(at("request"), rq.When, rq.Or); err != nil {
			return err
		}
		if len(rq.ExpectedStatus) > 0 && !remote.Mapped() {
			return fmt.Errorf("%s %s needs reconcile.remote.response: a remote that answers in Inrun's Result always replies 200",
				failureMark(), at("request.expectedStatus"))
		}
		if err := checkStatusCodes(at("request.expectedStatus"), rq.ExpectedStatus); err != nil {
			return err
		}
		if rq.Finalize != nil {
			if !remote.Mapped() {
				return fmt.Errorf("%s %s needs reconcile.remote.response: a remote that answers in Inrun's Result handles deletion itself (finalizing: true)",
					failureMark(), at("request.finalize"))
			}
			if err := checkMethod(at("request.finalize.method"), rq.Finalize.Method); err != nil {
				return err
			}
			if err := body("request.finalize.body", rq.Finalize.Body); err != nil {
				return err
			}
			if err := checkStatusCodes(at("request.finalize.expectedStatus"), rq.Finalize.ExpectedStatus); err != nil {
				return err
			}
			if err := body("request.finalize.endpoint", rq.Finalize.Endpoint); err != nil {
				return err
			}
			if err := checkCallConditions(at("request.finalize"), rq.Finalize.When, rq.Finalize.Or); err != nil {
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
