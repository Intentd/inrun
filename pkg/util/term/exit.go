package term

import (
	"os"

	"github.com/intentd/inrun/pkg/logger"
)

// Exit exits in error with a code
func Exit(err error) {
	if err != nil {
		os.Stderr.WriteString(FailureMark() + " " + err.Error() + "\n")
	}
	os.Exit(1)
}

// FatalExit logs err and its reason, then exits the process. It never returns.
func FatalExit(reason string, err error) {
	logger.Error().AnErr(reason, err)
	Exit(err)
}
