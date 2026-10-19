//go:build ignore

package hooks

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/template/resources"

	apiv1 "github.com/intentd/inrun-hooks-demo/api/v1alpha1"
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/resources/core/cronjobs"
	"github.com/intentd/inrun/pkg/resources/core/services"
	"github.com/intentd/inrun/pkg/resources/core/statefulsets"
	"github.com/intentd/inrun/pkg/types"
)

const (
	// Test values
	postgresUser     = "postgres"
	postgresPassword = "postgres-password"
	defaultReplicas  = "1"
)

// DatabaseHooks returns the Database CRD's hooks, registered in the Blueprint
// under hooks.function. They run inside the generic reconciler, so Inrun
// still handles the informer, queue, finalizer, events, Ready status and
// metrics; the hooks add typed spec access and the children the templates
// cannot express.
func DatabaseHooks() domain.AnyReconcileHooks {
	return domain.ReconcileHooks[*apiv1.Database]{
		OnReconcile: onDatabaseReconcile,
		OnDelete:    onDatabaseDelete,
	}
}

// onDatabaseReconcile runs on every reconcile cycle for a Database CR.
// It creates the StatefulSet, Service, and optionally a backup CronJob.
func onDatabaseReconcile(ctx context.Context, req domain.Request, obj *apiv1.Database) (domain.Result, error) {
	kube, ok := kubeclient.FromContext(ctx)
	if !ok {
		return domain.Result{}, fmt.Errorf("kubeclient not in context")
	}

	// ── Type-safe access to spec fields ───────────────────────────────────
	// This is why typed hooks exist: obj.Spec.Engine, not
	// unstructured map navigation with type assertions.
	engine := obj.Spec.Engine
	version := obj.Spec.Version
	storage := obj.Spec.Storage
	image := engineImage(engine, version)

	// ── Create the StatefulSet via pkg/resources ───────────────────────────
	// The resource package handles owner references, idempotency and system
	// labels; the hook only provides the resolved spec.
	spec := resources.StatefulSetSpec(
		types.StatefulSetTemplateSource{
			Name:               obj.Name,
			Namespace:          obj.Namespace,
			Image:              image,
			Replicas:           defaultReplicas,
			Port:               dbPort(engine),
			ServiceAccountName: obj.Name + "-sa", // declared in the Blueprint onCreate
			Env: []types.EnvVar{
				{Name: "POSTGRES_USER", Value: postgresUser},
				{Name: "POSTGRES_PASSWORD", Value: postgresPassword},
			},
			Labels: types.Labels{
				"db-engine":    engine,
				"db-version":   version,
				"storage-size": storage,
			},
		},
		obj.Name,
		nil,
	)
	if err := statefulsets.Apply(ctx, kube.Clientset(), obj, spec, kubeclient.ApplyOptions(kube, spec.ForceConflict)); err != nil {
		return domain.Result{}, fmt.Errorf("database statefulSet: %w", err)
	}

	// ── Create the Service ─────────────────────────────────────────────────
	svcSpec := resources.ServiceSpec(
		types.ServiceTemplateSource{
			Name:       obj.Name + "-svc",
			Namespace:  obj.Namespace,
			Port:       dbPort(engine),
			TargetPort: dbPort(engine),
			Type:       "ClusterIP",
		},
		obj.Name,
	)
	if err := services.Apply(ctx, kube.Clientset(), obj, svcSpec, kubeclient.ApplyOptions(kube, svcSpec.ForceConflict)); err != nil {
		return domain.Result{}, fmt.Errorf("database service: %w", err)
	}

	// ── Conditional: backup CronJob ────────────────────────────────────────
	if obj.Spec.Backup {
		cronSpec := resources.CronJobSpec(
			types.CronJobTemplateSource{
				Name:      obj.Name + "-backup",
				Namespace: obj.Namespace,
				Schedule:  "0 2 * * *", // daily at 2am
				Image:     backupImage(engine),
				Command:   []string{"/bin/backup.sh"},
				Args:      []string{obj.Name, obj.Namespace},
			},
			obj.Name,
			nil,
		)
		if err := cronjobs.Apply(ctx, kube.Clientset(), obj, cronSpec, kubeclient.ApplyOptions(kube, cronSpec.ForceConflict)); err != nil {
			return domain.Result{}, fmt.Errorf("database backup cronjob: %w", err)
		}
	}

	return domain.Result{}, nil
}

// onDatabaseDelete runs when a Database CR is being deleted.
// Owner references handle StatefulSet, Service, and CronJob cleanup automatically.
// This hook handles external cleanup — nothing in this example, but the
// pattern is here for operators that create external resources.
func onDatabaseDelete(ctx context.Context, req domain.Request, obj *apiv1.Database) (domain.Result, error) {
	// For this example the database is purely in-cluster.
	// In a real external provisioner, you would:
	//   return externalDB.Delete(ctx, obj.Spec.InstanceID)
	return domain.Result{}, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

func engineImage(engine, version string) string {
	if version == "" {
		version = "latest"
	}
	switch engine {
	case "mysql":
		return fmt.Sprintf("mysql:%s", version)
	default:
		return fmt.Sprintf("postgres:%s", version)
	}
}

func dbPort(engine string) string {
	if engine == "mysql" {
		return "3306"
	}
	return "5432"
}

func backupImage(engine string) string {
	if engine == "mysql" {
		return "myorg/mysql-backup:latest"
	}
	return "myorg/postgres-backup:latest"
}
