package presets

import respresets "github.com/intentd/inrun/pkg/resources/presets"

// builtin holds the built-in presets that a Blueprint's names are checked against.
var builtin = respresets.Builtin()

func isBuiltinResource(name string) bool      { _, ok := builtin.Resource(name); return ok }
func isBuiltinProbe(name string) bool         { _, ok := builtin.Probe(name); return ok }
func isBuiltinSecurity(name string) bool      { _, ok := builtin.ContainerSecurity(name); return ok }
func isBuiltinRollingUpdate(name string) bool { _, ok := builtin.RollingUpdate(name); return ok }
func isBuiltinHPA(name string) bool           { _, ok := builtin.HPA(name); return ok }
func isBuiltinPDB(name string) bool           { _, ok := builtin.PDB(name); return ok }
func isBuiltinNetworkPolicy(name string) bool { _, ok := builtin.NetworkPolicy(name); return ok }
func isBuiltinResourceQuota(name string) bool { _, ok := builtin.ResourceQuota(name); return ok }
