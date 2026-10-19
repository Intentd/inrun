// Package webhook runs the gateway's HTTPS webhook server. It routes to the
// handlers in admission, conversion and protection, registers their
// configurations at startup through registration, and removes them on
// shutdown when the Blueprint asks. The housekeeper keeps them in sync in
// between.
package webhook
