package resources

import (
	"reflect"
	"sort"
	"testing"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/structured"
)

// templateOnly lists, per pod kind, the template fields kubewright's spec has
// no field for: entry controls the runtime reads itself, and fields merged into
// another (a StatefulSet's tag into its image). Every other template field must
// reach kubewright's spec of the same name. Adding a field here is a decision,
// not a fix: it means the field is not applied to the object.
var templateOnly = map[string][]string{
	"Pod":         nil,
	"Deployment":  {"Autoscale"},
	"ReplicaSet":  {"Autoscale"},
	"StatefulSet": {"Autoscale"},
	"Job":         nil,
	"CronJob":     nil,
}

// entryControls are read by the runtime from the entry, never sent to kubewright.
var entryControls = []string{"ForEach", "ForceConflict", "Or", "Reconcile", "When"}

// TestPodKindsCarryEveryField fills every field of each pod kind's template,
// runs it through the same resolve and spec steps the runtime uses, and checks
// every field kubewright's spec shares with the template arrived set. A new
// template field that is not wired through fails here by name.
func TestPodKindsCarryEveryField(t *testing.T) {
	owner := object.Owner(object.WithUID("uid-1"))
	r := template.NewResolverFromMap(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "app", "namespace": "default"},
	})
	reg := &types.PresetRegistry{}
	skip := "Preset"

	kinds := []struct {
		name string
		run  func(t *testing.T) (src, spec any)
	}{
		{"Pod", func(t *testing.T) (any, any) {
			src := structured.NonZero[types.PodTemplateSource](skip)
			res, err := ResolvePod(r, src)
			must(t, err)
			return src, PodSpec(res, owner, reg)
		}},
		{"Deployment", func(t *testing.T) (any, any) {
			src := structured.NonZero[types.DeploymentTemplateSource](skip)
			res, err := ResolveDeployment(r, src)
			must(t, err)
			return src, DeploymentSpec(res, owner, reg)
		}},
		{"ReplicaSet", func(t *testing.T) (any, any) {
			src := structured.NonZero[types.ReplicaSetTemplateSource](skip)
			res, err := ResolveReplicaSet(r, src)
			must(t, err)
			return src, ReplicaSetSpec(res, owner, reg)
		}},
		{"StatefulSet", func(t *testing.T) (any, any) {
			src := structured.NonZero[types.StatefulSetTemplateSource](skip)
			res, err := ResolveStatefulSet(r, src)
			must(t, err)
			return src, StatefulSetSpec(res, owner, reg)
		}},
		{"Job", func(t *testing.T) (any, any) {
			src := structured.NonZero[types.JobTemplateSource](skip)
			res, err := ResolveJob(r, src)
			must(t, err)
			return src, JobSpec(res, res.BackoffLimit, owner, reg)
		}},
		{"CronJob", func(t *testing.T) (any, any) {
			src := structured.NonZero[types.CronJobTemplateSource](skip)
			res, err := ResolveCronJob(r, src)
			must(t, err)
			return src, CronJobSpec(res, owner, reg)
		}},
	}

	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			src, spec := k.run(t)
			sv := reflect.ValueOf(spec)
			skip := map[string]bool{}
			for _, f := range entryControls {
				skip[f] = true
			}
			for _, f := range templateOnly[k.name] {
				skip[f] = true
			}

			var lost, unlisted []string
			st := reflect.TypeOf(src)
			for i := 0; i < st.NumField(); i++ {
				name := st.Field(i).Name
				if skip[name] {
					continue
				}
				f := sv.FieldByName(name)
				switch {
				case !f.IsValid():
					unlisted = append(unlisted, name)
				case f.IsZero():
					lost = append(lost, name)
				}
			}
			sort.Strings(lost)
			sort.Strings(unlisted)
			if len(lost) > 0 {
				t.Errorf("%s: template fields set but not in kubewright's spec: %v (wire them through Resolve%s and %sSpec)", k.name, lost, k.name, k.name)
			}
			if len(unlisted) > 0 {
				t.Errorf("%s: template fields with no kubewright field: %v (map them, or list them in templateOnly)", k.name, unlisted)
			}
		})
	}
}

// TestContainersReachKubewright checks one container's fields, presets and
// conversions in detail, for init containers and containers alike.
func TestContainersReachKubewright(t *testing.T) {
	owner := object.Owner()
	r := template.NewResolverFromMap(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "app", "namespace": "default"},
		"spec":     map[string]interface{}{"image": "app:1.2", "port": 8080},
	})
	src := types.DeploymentTemplateSource{
		InitContainers: []types.Container{{Name: "migrate", Image: "{{ .spec.image }}", Args: []string{"migrate", "", "up"}}},
		Containers: []types.Container{{
			Name: "app", Image: "{{ .spec.image }}", Port: "{{ .spec.port }}", Protocol: "udp",
			ImagePullPolicy: "IfNotPresent",
			Env:             []types.EnvVar{{Name: "WHO", Value: "{{ .metadata.name }}"}},
		}},
	}
	res, err := ResolveDeployment(r, src)
	must(t, err)
	spec := DeploymentSpec(res, owner, &types.PresetRegistry{})

	if len(spec.InitContainers) != 1 || len(spec.Containers) != 1 {
		t.Fatalf("got %d init containers and %d containers, want 1 and 1", len(spec.InitContainers), len(spec.Containers))
	}
	init, app := spec.InitContainers[0], spec.Containers[0]
	if init.Image != "app:1.2" || !reflect.DeepEqual(init.Args, []string{"migrate", "", "up"}) {
		t.Errorf("init container = %+v; want the resolved image and all three args, the empty one kept", init)
	}
	if app.Image != "app:1.2" || app.Port != 8080 || app.Protocol != "UDP" || app.ImagePullPolicy != "IfNotPresent" {
		t.Errorf("container = %+v; want image app:1.2, port 8080, protocol UDP, pull policy IfNotPresent", app)
	}
	if len(app.Env) != 1 || app.Env[0].Value != "app" {
		t.Errorf("container env = %+v; want WHO=app", app.Env)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
