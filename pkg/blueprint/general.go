package blueprint

// HasFieldMigrate reports whether at least one CRD has a serve field with
// migrate.from.
func (b *Blueprint) HasFieldMigrate() bool {
	if b == nil {
		return false
	}
	for _, crd := range b.Enabled() {
		if crd.HasServeFieldMigrate() {
			return true
		}
	}
	return false
}

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
