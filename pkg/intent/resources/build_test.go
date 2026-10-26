package resources

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
)

func TestSupportedIntentTypesAreSorted(t *testing.T) {
	got := Types()

	if !sortStringsAreSorted(got) {
		t.Fatalf("Types() is not sorted: %v", got)
	}

	if len(got) != len(resourceBuilders) {
		t.Fatalf("supported type count = %d, want %d", len(got), len(resourceBuilders))
	}

	for _, typ := range got {
		if _, ok := resourceBuilders[typ]; !ok {
			t.Errorf("supported type %q has no registered builder", typ)
		}
	}
}

func TestBuildFromIntentRejectsNilOwner(t *testing.T) {
	_, err := Build("deployment", map[string]interface{}{}, nil)

	if err == nil {
		t.Fatal("expected error for nil owner")
	}

	if !strings.Contains(err.Error(), "requires the owner object") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildFromIntentRejectsUnknownType(t *testing.T) {
	owner := object.New("", "", "owner")

	_, err := Build("unknown", map[string]interface{}{}, owner)

	if err == nil {
		t.Fatal("expected error for unknown type")
	}

	if !strings.Contains(err.Error(), "unknown resource type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildFromIntentChecksOwnerBeforeType(t *testing.T) {
	// nil owner AND unknown type — error should be about the owner, not the type.
	_, err := Build("unknown", map[string]interface{}{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "requires the owner object") {
		t.Fatalf("expected owner error, got: %v", err)
	}
}

func TestSupportedIntentTypesErrorIsDeterministic(t *testing.T) {
	first := strings.Join(Types(), ", ")
	second := strings.Join(Types(), ", ")

	if first != second {
		t.Fatalf("supported types are not deterministic:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestIntentBuildersRegistryMatchesSupportedTypes(t *testing.T) {
	got := Types()

	want := make([]string, 0, len(resourceBuilders))
	for typ := range resourceBuilders {
		want = append(want, typ)
	}

	// Types already sorts its result.
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registered types mismatch:\ngot:  %v\nwant: %v", got, want)
	}
}

func sortStringsAreSorted(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] > values[i] {
			return false
		}
	}
	return true
}

// containersAt reads the pod template's container names and their env names.
func containersAt(t *testing.T, obj map[string]interface{}, path ...string) (names []string, env [][]string) {
	t.Helper()
	cur := obj
	for _, p := range path {
		cur = cur[p].(map[string]interface{})
	}
	list, _ := cur["containers"].([]interface{})
	for _, c := range list {
		m := c.(map[string]interface{})
		names = append(names, m["name"].(string))
		var vars []string
		envList, _ := m["env"].([]interface{})
		for _, e := range envList {
			vars = append(vars, e.(map[string]interface{})["name"].(string))
		}
		env = append(env, vars)
	}
	return names, env
}

func TestBuild_PodKindsTakeContainers(t *testing.T) {
	owner := object.Owner()
	fields := func(extra map[string]interface{}) map[string]interface{} {
		f := map[string]interface{}{
			"name": "web",
			"env":  map[string]interface{}{"B": "2", "A": "1"},
			"containers": []interface{}{
				map[string]interface{}{"name": "app", "image": "nginx:1.27"},
				map[string]interface{}{"name": "sidecar", "image": "envoy:1.30"},
			},
			"initContainers": []interface{}{
				map[string]interface{}{"name": "migrate", "image": "app:1", "command": []interface{}{"migrate"}},
			},
		}
		for k, v := range extra {
			f[k] = v
		}
		return f
	}
	cases := []struct {
		typ   string
		extra map[string]interface{}
		path  []string
	}{
		{"deployment", nil, []string{"spec", "template", "spec"}},
		{"statefulset", nil, []string{"spec", "template", "spec"}},
		{"job", nil, []string{"spec", "template", "spec"}},
		{"cronjob", map[string]interface{}{"schedule": "*/5 * * * *"}, []string{"spec", "jobTemplate", "spec", "template", "spec"}},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			obj, err := Build(c.typ, fields(c.extra), owner)
			if err != nil {
				t.Fatal(err)
			}
			names, env := containersAt(t, obj, c.path...)
			if !reflect.DeepEqual(names, []string{"app", "sidecar"}) {
				t.Fatalf("containers = %v", names)
			}
			pod := obj
			for _, p := range c.path {
				pod = pod[p].(map[string]interface{})
			}
			if inits, _ := pod["initContainers"].([]interface{}); len(inits) != 1 {
				t.Fatalf("initContainers = %v", pod["initContainers"])
			}
			if c.typ == "deployment" || c.typ == "statefulset" {
				for i, vars := range env {
					if !reflect.DeepEqual(vars, []string{"A", "B"}) {
						t.Fatalf("container %d env = %v, want A, B inherited in key order", i, vars)
					}
				}
			}
		})
	}
}

func TestBuild_ImageOrContainers(t *testing.T) {
	owner := object.Owner()
	_, err := Build("deployment", map[string]interface{}{"name": "web"}, owner)
	if err == nil || !strings.Contains(err.Error(), "image or containers is required") {
		t.Fatalf("neither: err = %v", err)
	}
	_, err = Build("job", map[string]interface{}{
		"name": "web", "image": "nginx",
		"containers": []interface{}{map[string]interface{}{"name": "app", "image": "nginx"}},
	}, owner)
	if err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("both: err = %v", err)
	}
}

func TestBuild_Protocol(t *testing.T) {
	owner := object.Owner()
	port := func(obj map[string]interface{}, path ...string) string {
		cur := obj
		for _, p := range path {
			cur = cur[p].(map[string]interface{})
		}
		list, _ := cur["ports"].([]interface{})
		return list[0].(map[string]interface{})["protocol"].(string)
	}
	dep, err := Build("deployment", map[string]interface{}{"name": "dns", "image": "coredns", "port": 53, "protocol": "udp"}, owner)
	if err != nil {
		t.Fatal(err)
	}
	c := dep["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})
	if got := port(c); got != "UDP" {
		t.Errorf("deployment protocol = %q, want UDP", got)
	}
	svc, err := Build("service", map[string]interface{}{"name": "dns", "port": 53, "protocol": "UDP"}, owner)
	if err != nil {
		t.Fatal(err)
	}
	if got := port(svc, "spec"); got != "UDP" {
		t.Errorf("service protocol = %q, want UDP", got)
	}
	tcp, err := Build("deployment", map[string]interface{}{"name": "web", "image": "nginx", "port": 80}, owner)
	if err != nil {
		t.Fatal(err)
	}
	c = tcp["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})
	if got := port(c); got != "TCP" {
		t.Errorf("default protocol = %q, want TCP", got)
	}
}
