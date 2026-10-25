package conversion

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// applyConversion converts obj from its current version to targetAPIVersion.
// targetAPIVersion is the full apiVersion string from the ConversionReview request.
func applyConversion(
	obj map[string]interface{},
	rules *types.ConversionRules,
	targetAPIVersion string,
) (map[string]interface{}, error) {
	sourceAPIVersion, ok := obj["apiVersion"].(string)
	if !ok || sourceAPIVersion == "" {
		return nil, fmt.Errorf("object missing apiVersion")
	}

	_, sourceVersion := splitAPIVersion(sourceAPIVersion)
	_, targetVersion := splitAPIVersion(targetAPIVersion)

	if sourceVersion == targetVersion {
		out := copyMap(obj)
		out["apiVersion"] = targetAPIVersion
		return out, nil
	}

	path := rules.FindPath(sourceVersion, targetVersion)
	if path == nil {
		return nil, fmt.Errorf(
			"no conversion path declared for %s → %s in kind %q.\n"+
				"Add a path to the Blueprint:\n"+
				"  conversion:\n"+
				"    paths:\n"+
				"      - from: %s\n"+
				"        to: %s\n"+
				"        spec:\n"+
				"          # fields in %s format",
			sourceVersion, targetVersion, rules.Kind,
			sourceVersion, targetVersion, targetVersion,
		)
	}

	resolver := template.NewResolverFromMap(obj)
	convertedSpec, err := resolver.ResolveNested(path.Spec, template.DropEmpty())
	if err != nil {
		return nil, fmt.Errorf("converting spec from %s to %s: %w", sourceVersion, targetVersion, err)
	}

	out := copyMap(obj)
	out["apiVersion"] = targetAPIVersion
	out["spec"] = convertedSpec

	return out, nil
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func splitAPIVersion(apiVersion string) (group, version string) {
	i := strings.LastIndex(apiVersion, "/")
	if i < 0 {
		return "", apiVersion
	}
	return apiVersion[:i], apiVersion[i+1:]
}

func joinAPIVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
}
