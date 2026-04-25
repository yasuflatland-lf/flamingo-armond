package database

import (
	"context"

	"github.com/rotisserie/eris"

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
		return nil, eris.Wrap(err, "database: parse DSN")
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
		return nil, eris.Wrap(err, "database: connect")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, eris.Wrap(err, "database: ping")
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		pool.Close()
		return nil, eris.Wrap(err, "database: gorm open")
	}
	return &DB{Pool: pool, GORM: gormDB}, nil
}

// Close releases pool resources. Tolerates a nil receiver and a nil pool, but
// must be called at most once on a live DB — pgxpool.Close panics on a
// double-close.
func (d *DB) Close() {
	if d == nil || d.Pool == nil {
		return
	}
	d.Pool.Close()
}
