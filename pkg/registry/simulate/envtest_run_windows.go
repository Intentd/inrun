//go:build windows

package simulate

import (
	"context"
	"errors"
)

// runWithEnvtest is unavailable on Windows: controller-runtime's envtest
// (v0.25.2) does not compile there. In-memory simulation works; --envtest
// and --compare need a Linux or macOS build.
func runWithEnvtest(context.Context, RunOptions) (*Result, error) {
	return nil, errors.New("envtest is not available in Windows builds of inrun: use the in-memory simulation, or run --envtest and --compare on Linux or macOS")
}
