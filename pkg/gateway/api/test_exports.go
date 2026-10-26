package api

import (
	"context"
	"net/http"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
)

func WithTokenName(ctx context.Context, name string) context.Context {
	return contextWithTokenName(ctx, name)
}

func ParsePath(path string) (kind, ns, name string, err error) {
	return parsePath(path)
}

func ExportedSchemaHandler(bp *blueprint.Blueprint) http.Handler {
	return schemaHandler(bp)
}

func ExportedResourcesHandler(kube kubeclient.Interface, bp *blueprint.Blueprint, notes types.KELFunctionRegistry) http.Handler {
	return resourcesHandler(kube, &ClusterRegistry{clients: map[string]kubeclient.Interface{}}, bp)
}

func ExportedApplyHandler(kube kubeclient.Interface, bp *blueprint.Blueprint, notes types.KELFunctionRegistry) http.Handler {
	return applyHandler(kube, &ClusterRegistry{clients: map[string]kubeclient.Interface{}}, bp)
}

func ExportedCheckServePermission(w http.ResponseWriter, r *http.Request, crd *types.CRDEntry, class types.ServeEndpointClass, op, ns, alias string) bool {
	return checkServePermission(w, r, crd, class, op, ns, alias)
}

func ExportedWriteKubeError(w http.ResponseWriter, err error) {
	writeKubeError(w, err)
}
