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
// immediately (fail-fast), matching the auth.Config fail-fast pattern.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		URL:             os.Getenv("SUPABASE_DB_URL"),
		MaxConns:        10,
		MinConns:        0,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
	}

	if err := parseInt32Env("DB_MAX_CONNS", &cfg.MaxConns, 1); err != nil {
		return Config{}, err
	}
	if err := parseInt32Env("DB_MIN_CONNS", &cfg.MinConns, 0); err != nil {
		return Config{}, err
	}
	if err := parseDurationEnv("DB_MAX_CONN_LIFETIME", &cfg.MaxConnLifetime); err != nil {
		return Config{}, err
	}
	if err := parseDurationEnv("DB_MAX_CONN_IDLE_TIME", &cfg.MaxConnIdleTime); err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// parseInt32Env overwrites *out with the env value if set. min is the minimum
// permitted value (inclusive); values below it are rejected.
func parseInt32Env(key string, out *int32, min int32) error {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return fmt.Errorf("database: invalid %s %q: %w", key, v, err)
	}
	if int32(n) < min {
		return fmt.Errorf("database: %s must be >= %d, got %d", key, min, n)
	}
	*out = int32(n)
	return nil
}

// parseDurationEnv overwrites *out with the env value if set. Only positive
// durations are accepted.
func parseDurationEnv(key string, out *time.Duration) error {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("database: invalid %s %q: %w", key, v, err)
	}
	if d <= 0 {
		return fmt.Errorf("database: %s must be > 0, got %s", key, v)
	}
	*out = d
	return nil
}
