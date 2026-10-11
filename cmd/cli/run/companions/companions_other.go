//go:build !linux && !runtime && !gateway

package companions

import "os/exec"

// stopWithParent is Linux-only; elsewhere children stop through stopWithInrun.
func stopWithParent(*exec.Cmd) {}
