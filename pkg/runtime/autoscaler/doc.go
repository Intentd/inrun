// Package autoscaler adjusts an operatorBox's worker count, queue depth and
// resync interval at runtime from time and metric conditions, without
// restarting workers. One Autoscaler runs per operatorBox that declares
// autoscale and restores the declared baseline on shutdown.
package autoscaler
