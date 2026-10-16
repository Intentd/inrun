// migrate.Migrator handles live CR schema migration: it reads migrate.from
// declarations from the Catalog's serve fields and brings existing CRs to the
// current schema, reusing the same value/values transform logic as the
// gateway intent apply path. It supports forward migration, rollback from
// the snapshot annotation, and annotation cleanup.
package migrate
