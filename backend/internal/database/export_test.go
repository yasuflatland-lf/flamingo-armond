package database

// ConvertSchemeForMigrateForTest exposes convertSchemeForMigrate to the
// external _test package without widening the public API surface.
var ConvertSchemeForMigrateForTest = convertSchemeForMigrate
