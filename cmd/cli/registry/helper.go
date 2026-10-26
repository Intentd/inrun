//go:build !runtime && !gateway

package registry

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry"
	"gopkg.in/yaml.v3"
)

// extractRuntimeVersionFromGoMod scans go.mod for the inrun runtime dependency
// and returns its version (e.g. "v0.7.6"). Returns "" if go.mod is absent or the
// dependency is not declared.
func extractRuntimeVersionFromGoMod(dir string) string {
	f, err := os.Open(filepath.Join(dir, registry.FileGoMod))
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// inline form:  require github.com/intentd/inrun vX.Y.Z
		if strings.HasPrefix(line, "require github.com/intentd/inrun ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[2]
			}
		}
		// block form (inside require (...)):  github.com/intentd/inrun vX.Y.Z
		if strings.HasPrefix(line, "github.com/intentd/inrun ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ""
	}
	return ""
}

// detectTypedBlueprint parses a blueprint.yaml and returns a PatternTyped if any
// CRD declares customHooks or customConstructor. Returns nil on parse error.
func detectTypedBlueprint(blueprintPath string) *registry.PatternTyped {
	bp, err := blueprint.ParseFile(blueprintPath)
	if err != nil {
		return nil
	}
	var t registry.PatternTyped
	for _, name := range bp.CRDNames() {
		entry, ok := bp.CRDEntry(name)
		if !ok {
			continue
		}
		if entry.CustomHooksEnabled() {
			t.HasHooks = true
		}
		if entry.ConstructorEnabled() {
			t.HasConstructor = true
		}
	}
	if !t.HasHooks && !t.HasConstructor {
		return nil
	}
	return &t
}

// simulateFileHasAssertions returns true when simulate.yaml contains an
// expect: block — meaning the simulate gate will actually assert something.
func simulateFileHasAssertions(path string) bool {
	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return false
	}
	var doc struct {
		Spec *struct {
			Expect *struct{} `yaml:"expect"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	return doc.Spec != nil && doc.Spec.Expect != nil
}

// e2eFileHasAssertions returns true when e2e.yaml contains an expect: block —
// meaning the e2e gate will actually assert something.
func e2eFileHasAssertions(path string) bool {
	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return false
	}
	var doc struct {
		Spec *struct {
			Expect []struct{} `yaml:"expect"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	return doc.Spec != nil && len(doc.Spec.Expect) > 0
}

// countSimulateAssertions counts the total number of discrete assertions in
// simulate.yaml: each ops/absent rule counts as one, plus one each for
// steady, steadyAt, and noErrors when set. Recurses into crds: sub-expects.
func countSimulateAssertions(path string) int {
	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return 0
	}
	type expectBlock struct {
		Steady   *bool                   `yaml:"steady"`
		SteadyAt *int                    `yaml:"steadyAt"`
		NoErrors bool                    `yaml:"noErrors"`
		Ops      []struct{}              `yaml:"ops"`
		Absent   []struct{}              `yaml:"absent"`
		CRDs     map[string]*expectBlock `yaml:"crds"`
	}
	var doc struct {
		Spec *struct {
			Expect *expectBlock `yaml:"expect"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Spec == nil || doc.Spec.Expect == nil {
		return 0
	}
	var count func(e *expectBlock) int
	count = func(e *expectBlock) int {
		if e == nil {
			return 0
		}
		n := len(e.Ops) + len(e.Absent)
		if e.Steady != nil {
			n++
		}
		if e.SteadyAt != nil {
			n++
		}
		if e.NoErrors {
			n++
		}
		for _, crd := range e.CRDs {
			n += count(crd)
		}
		return n
	}
	return count(doc.Spec.Expect)
}

func detectRunner() string {
	switch {
	case os.Getenv("GITHUB_ACTIONS") == "true":
		return "github-actions"
	case os.Getenv("GITLAB_CI") == "true":
		return "gitlab-ci"
	case os.Getenv("CIRCLECI") == "true":
		return "circleci"
	case os.Getenv("JENKINS_URL") != "":
		return "jenkins"
	case os.Getenv("BUILDKITE") == "true":
		return "buildkite"
	case os.Getenv("DRONE") == "true":
		return "drone"
	case os.Getenv("CI") == "true":
		return "ci"
	default:
		return "local"
	}
}

// checkPublished returns the digest ref already has in the registry, for the
// new artifact to record, or "" when ref is not published. Without
// --overwrite, a published ref is an error.
func checkPublished(ctx context.Context, ref *registry.Ref) (string, error) {
	client, err := registry.NewClient()
	if err != nil {
		return "", err
	}
	current, err := client.Published(ctx, ref)
	if err != nil {
		return "", err
	}
	if current != "" && !pushOverwrite {
		return "", fmt.Errorf("%s is already published\n  Bump metadata.version to publish a change, or pass --overwrite to replace it", ref.String())
	}
	return current, nil
}
