package database

import (
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/rotisserie/eris"
)

// ConvertSchemeForMigrateForTest exposes convertSchemeForMigrate to the
// external _test package without widening the public API surface.
var ConvertSchemeForMigrateForTest = convertSchemeForMigrate

// MigrateStepsForTest applies n migration steps against the embedded source.
// Positive n applies that many up steps; negative n rolls that many down.
// Used by migration-level tests that need to exercise both directions
// (e.g. assert that down really removes a created index).
//
// Test-only: not part of the package public API.
func MigrateStepsForTest(url string, n int) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return eris.Wrap(err, "database: open migrations FS")
	}
	defer src.Close()

	m, err := migrate.NewWithSourceInstance("iofs", src, convertSchemeForMigrate(url))
	if err != nil {
		return eris.Wrap(err, "database: init migrate (test)")
	}
	defer m.Close()

	if err := m.Steps(n); err != nil {
		return eris.Wrapf(err, "database: migrate steps %d (test)", n)
	}
	return nil
}

// MigrateForceForTest forcibly sets the schema_migrations version to the given
// value and clears the dirty flag. This is the idiomatic golang-migrate
// recovery mechanism after a failed migration leaves the database in a dirty
// state. version must be the integer timestamp of the last successfully applied
// migration (i.e. the predecessor of the failed one).
//
// Test-only: not part of the package public API.
func MigrateForceForTest(url string, version int) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return eris.Wrap(err, "database: open migrations FS")
	}
	defer src.Close()

	m, err := migrate.NewWithSourceInstance("iofs", src, convertSchemeForMigrate(url))
	if err != nil {
		return eris.Wrap(err, "database: init migrate (test)")
	}
	defer m.Close()

	if err := m.Force(version); err != nil {
		return eris.Wrapf(err, "database: migrate force version %d (test)", version)
	}
	return nil
}
