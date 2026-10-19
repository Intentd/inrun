// Package presets names the field settings teams repeat: resource requests
// and limits, probe timings, container and pod security, rolling update,
// HPA and PDB behavior, network policy rules and namespace quotas. A spec
// field names a preset ("medium", "restricted", "safe") instead of spelling
// the settings out.
//
// Builtin returns the built-in presets. A Registry merges your own with
// With, where yours replace built-ins of the same name, and looks them up by
// name, ignoring case:
//
//	reg := presets.Builtin().With(own)
//	if r, ok := reg.Resource("medium"); ok {
//		container.Resources = r.Requirements()
//	}
package presets
