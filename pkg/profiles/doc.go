// Package profiles defines the named presets (resource, security, probe,
// autoscaler, HPA, PDB, network policy and others) and the Apply* functions
// that expand them into full types at Blueprint load time. The runtime never
// sees a profile name.
package profiles
