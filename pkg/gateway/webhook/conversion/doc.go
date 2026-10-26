// Package conversion serves the gateway's /convert webhook: the API server
// sends a ConversionReview when it needs a CR at another version, and the
// handler converts it with the rules the Blueprint declares for its kind.
package conversion
