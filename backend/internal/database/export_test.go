package database

import (
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// ConvertSchemeForMigrateForTest exposes convertSchemeForMigrate to the
// external _test package without widening the public API surface.
var ConvertSchemeForMigrateForTest = convertSchemeForMigrate

// NewMigrateInstanceForTest returns a *migrate.Migrate bound to the embedded
// migrations FS and the given DSN. The caller is responsible for calling
// m.Close() after use. Intended for down→up roundtrip tests only. The caller
// must defer m.Close() or otherwise close the migrate instance before invoking
// Migrate again to avoid locking the database.
func NewMigrateInstanceForTest(dsn string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}
	return migrate.NewWithSourceInstance("iofs", src, convertSchemeForMigrate(dsn))
}
