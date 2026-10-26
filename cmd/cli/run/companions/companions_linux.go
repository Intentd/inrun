//go:build linux && !runtime && !gateway

package companions

import (
	"os/exec"
	"syscall"
)

// stopWithParent has the kernel send SIGTERM to the child when inrun exits for
// any reason, including an error exit that never reaches stopWithInrun.
func stopWithParent(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
