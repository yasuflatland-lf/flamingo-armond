package database

import (
	"errors"
	"fmt"
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

// Validate returns a non-nil error if any required or range-constrained Config
// field is invalid.
func (c Config) Validate() error {
	if c.URL == "" {
		return errors.New("database: SUPABASE_DB_URL is required")
	}
	if c.MaxConns < 0 {
		return fmt.Errorf("database: MaxConns must be >= 0, got %d", c.MaxConns)
	}
	if c.MinConns < 0 {
		return fmt.Errorf("database: MinConns must be >= 0, got %d", c.MinConns)
	}
	if c.MaxConns > 0 && c.MinConns > c.MaxConns {
		return fmt.Errorf("database: MinConns (%d) must not exceed MaxConns (%d)", c.MinConns, c.MaxConns)
	}
	if c.MaxConnLifetime < 0 {
		return fmt.Errorf("database: MaxConnLifetime must be >= 0, got %s", c.MaxConnLifetime)
	}
	if c.MaxConnIdleTime < 0 {
		return fmt.Errorf("database: MaxConnIdleTime must be >= 0, got %s", c.MaxConnIdleTime)
	}
	return nil
}

// ConfigFromEnv reads SUPABASE_DB_URL and optional pool-tuning env vars.
// An unset env var applies the default; an invalid value returns an error
// immediately (fail-fast), matching the auth.Config pattern used in PR7.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		URL:             os.Getenv("SUPABASE_DB_URL"),
		MaxConns:        10,
		MinConns:        0,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
	}

	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return Config{}, fmt.Errorf("database: invalid DB_MAX_CONNS %q: %w", v, err)
		}
		if n <= 0 {
			return Config{}, fmt.Errorf("database: DB_MAX_CONNS must be > 0, got %d", n)
		}
		cfg.MaxConns = int32(n)
	}

	if v := os.Getenv("DB_MIN_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return Config{}, fmt.Errorf("database: invalid DB_MIN_CONNS %q: %w", v, err)
		}
		if n < 0 {
			return Config{}, fmt.Errorf("database: DB_MIN_CONNS must be >= 0, got %d", n)
		}
		cfg.MinConns = int32(n)
	}

	if v := os.Getenv("DB_MAX_CONN_LIFETIME"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("database: invalid DB_MAX_CONN_LIFETIME %q: %w", v, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("database: DB_MAX_CONN_LIFETIME must be > 0, got %s", v)
		}
		cfg.MaxConnLifetime = d
	}

	if v := os.Getenv("DB_MAX_CONN_IDLE_TIME"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("database: invalid DB_MAX_CONN_IDLE_TIME %q: %w", v, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("database: DB_MAX_CONN_IDLE_TIME must be > 0, got %s", v)
		}
		cfg.MaxConnIdleTime = d
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
