package database

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Config holds connection-pool configuration for the Supabase Postgres instance.
type Config struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// Validate returns a non-nil error if any required Config field is empty.
func (c Config) Validate() error {
	if c.URL == "" {
		return errors.New("database: SUPABASE_DB_URL is required")
	}
	return nil
}

// ConfigFromEnv reads SUPABASE_DB_URL and optional pool-tuning env vars. Invalid
// numeric/duration overrides are logged at WARN and fall back to defaults, matching
// the tolerant pattern used for SHUTDOWN_TIMEOUT elsewhere in the backend.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		URL:             os.Getenv("SUPABASE_DB_URL"),
		MaxConns:        10,
		MinConns:        0,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
	}

	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n > 0 {
			cfg.MaxConns = int32(n)
		} else {
			slog.Warn("database: invalid DB_MAX_CONNS, using default", "value", v, "default", cfg.MaxConns)
		}
	}
	if v := os.Getenv("DB_MIN_CONNS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n >= 0 {
			cfg.MinConns = int32(n)
		} else {
			slog.Warn("database: invalid DB_MIN_CONNS, using default", "value", v, "default", cfg.MinConns)
		}
	}
	if v := os.Getenv("DB_MAX_CONN_LIFETIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.MaxConnLifetime = d
		} else {
			slog.Warn("database: invalid DB_MAX_CONN_LIFETIME, using default", "value", v, "default", cfg.MaxConnLifetime)
		}
	}
	if v := os.Getenv("DB_MAX_CONN_IDLE_TIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.MaxConnIdleTime = d
		} else {
			slog.Warn("database: invalid DB_MAX_CONN_IDLE_TIME, using default", "value", v, "default", cfg.MaxConnIdleTime)
		}
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
