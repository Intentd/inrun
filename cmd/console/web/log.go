package web

import (
	"log"
	"strings"
)

// Log levels, lowest first.
const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

// logLevel is the minimum level printed. Set once at startup by SetLogLevel.
var logLevel = levelInfo

// SetLogLevel sets the minimum level printed: debug, info, warn or error.
// Anything else keeps info.
func SetLogLevel(s string) {
	switch strings.ToLower(s) {
	case "debug":
		logLevel = levelDebug
	case "warn", "warning":
		logLevel = levelWarn
	case "error":
		logLevel = levelError
	default:
		logLevel = levelInfo
	}
}

func logf(level int, prefix, format string, args ...any) {
	if level < logLevel {
		return
	}
	log.Printf(prefix+format, args...)
}

// Debugf, Infof, Warnf and Errorf print at their level when it is enabled.
func Debugf(format string, args ...any) { logf(levelDebug, "DEBUG: ", format, args...) }
func Infof(format string, args ...any)  { logf(levelInfo, "INFO: ", format, args...) }
func Warnf(format string, args ...any)  { logf(levelWarn, "WARN: ", format, args...) }
func Errorf(format string, args ...any) { logf(levelError, "ERROR: ", format, args...) }
