package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB bundles a single pgxpool with a GORM handle that shares the same
// underlying connections. The two fields must be obtained together via Open;
// reassigning them after construction breaks the shared-pool invariant.
type DB struct {
	Pool *pgxpool.Pool
	GORM *gorm.DB
}

// Open parses cfg.URL, applies pool tuning, verifies connectivity with a Ping,
// and wires up GORM over the same pool via database/sql's stdlib adapter.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	pcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("database: parse DSN: %w", err)
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		pcfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("database: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: gorm open: %w", err)
	}
	return &DB{Pool: pool, GORM: gormDB}, nil
}

// Close releases pool resources. Safe to call on a nil receiver or an
// already-closed DB.
func (d *DB) Close() {
	if d == nil || d.Pool == nil {
		return
	}
	d.Pool.Close()
}
