// pkg/merger/pull_stub.go
//
// Stand-in for pull.go in the runtime and gateway builds. See the comment
// at the top of pull.go for why. PullToDir/PullModuleToDir are called by
// pkg/registry/module (module import resolution) — another authoring-time-only
// path the runtime and gateway never exercise, since they only ever read
// the blueprint.yaml key from an already-merged ConfigMap.

//go:build runtime || gateway

package merger

import (
	"fmt"

	"github.com/intentd/inrun/pkg/util/fileio"
)

func PullToDir(url, version string, oci bool, auth *fileio.FileAuth) (dir string, cleanup func(), err error) {
	return "", nil, fmt.Errorf("pulling registry pattern %q@%s is not supported in this build — "+
		"registry/module imports are authoring-time only; run 'inrun generate bundle' to "+
		"pre-merge before deploying the runtime or gateway", url, version)
}

func PullModuleToDir(url, version string, oci bool, auth *fileio.FileAuth) (dir string, cleanup func(), err error) {
	return "", nil, fmt.Errorf("pulling module %q@%s is not supported in this build — "+
		"registry/module imports are authoring-time only; run 'inrun generate bundle' to "+
		"pre-merge before deploying the runtime or gateway", url, version)
}
