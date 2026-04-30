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
