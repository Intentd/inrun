package blueprint

// HasMigrate reports whether at least one CRD has migration configured
func (b *Blueprint) HasMigrate() bool {
	if b == nil {
		return false
	}
	for _, crd := range b.Enabled() {
		if crd.HasMigrate() {
			return true
		}
	}
	return false
}
