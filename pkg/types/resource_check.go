package types

// UsesTemplates safely checks a HookTemplates pointer and returns true
// if the selected slice contains at least one template.
// Used by pkg/blueprint builtin registry to detect resource usage.
func UsesTemplates[T any](tpl *HookTemplates, sel func(*HookTemplates) []T) bool {
	if tpl == nil {
		return false
	}
	return len(sel(tpl)) > 0
}
