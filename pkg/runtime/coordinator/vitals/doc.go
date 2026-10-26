// Package vitals tracks runtime health: CRDHealth for each CRD's lifecycle
// state and counters, RuntimeHealth for readiness and leadership. The Build*
// handlers serve this state, the Blueprint and live CRs over HTTP.
package vitals
