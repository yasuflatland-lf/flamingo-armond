package database

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrate applies all pending up-migrations. Re-running is safe; ErrNoChange is
// returned as nil so callers can invoke this unconditionally on boot.
func Migrate(url string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("database: open migrations FS: %w", err)
	}
	defer func() {
		if err := src.Close(); err != nil {
			slog.Warn("database: close migrations source", "err", err)
		}
	}()

	migURL := convertSchemeForMigrate(url)
	m, err := migrate.NewWithSourceInstance("iofs", src, migURL)
	if err != nil {
		return fmt.Errorf("database: init migrate: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			slog.Warn("database: close migrate runner", "src_err", srcErr, "db_err", dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("database: migrate up: %w", err)
	}
	return nil
}

// convertSchemeForMigrate rewrites the DSN scheme to what the golang-migrate
// pgx/v5 driver expects. Only a prefix swap is performed so query strings and
// IPv6 hostnames in the DSN remain intact.
func convertSchemeForMigrate(url string) string {
	if strings.HasPrefix(url, "postgresql://") {
		return "pgx5://" + strings.TrimPrefix(url, "postgresql://")
	}
	if strings.HasPrefix(url, "postgres://") {
		return "pgx5://" + strings.TrimPrefix(url, "postgres://")
	}
	return url
}
