package term

// HealthIconReady returns the green "healthy" icon.
func HealthIconReady() string {
	return HealthIcon("ready")
}

// HealthIconRunning returns the green "running" icon.
func HealthIconRunning() string {
	return HealthIcon("running")
}

// HealthIconActive returns the green "active" icon.
func HealthIconActive() string {
	return HealthIcon("active")
}

// HealthIconHealthy returns the green "healthy" icon.
func HealthIconHealthy() string {
	return HealthIcon("healthy")
}

// HealthIconPending returns the yellow "pending" icon.
func HealthIconPending() string {
	return HealthIcon("pending")
}

// HealthIconProgressing returns the yellow "progressing" icon.
func HealthIconProgressing() string {
	return HealthIcon("progressing")
}

// HealthIconWarning returns the yellow "warning" icon.
func HealthIconWarning() string {
	return HealthIcon("warning")
}

// HealthIconInfo returns the blue "info" icon.
func HealthIconInfo() string {
	return HealthIcon("info")
}

// HealthIconError returns the red "error" icon.
func HealthIconError() string {
	return HealthIcon("error")
}

// HealthIconFailed returns the red "failed" icon.
func HealthIconFailed() string {
	return HealthIcon("failed")
}

// HealthIconUnhealthy returns the red "unhealthy" icon.
func HealthIconUnhealthy() string {
	return HealthIcon("unhealthy")
}

// HealthIconDegraded returns the red "degraded" icon.
func HealthIconDegraded() string {
	return HealthIcon("degraded")
}

// HealthIconUnknown returns the gray "unknown" icon.
func HealthIconUnknown() string {
	return HealthIcon("unknown")
}
