package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const invalidTextRepresentationDriverName = "repository-invalid-text-representation"

var registerInvalidTextRepresentationDriver sync.Once

type invalidTextRepresentationDriver struct{}

func (invalidTextRepresentationDriver) Open(string) (driver.Conn, error) {
	return invalidTextRepresentationConn{}, nil
}

type invalidTextRepresentationConn struct{}

func invalidTextRepresentationError() error {
	return &pgconn.PgError{Code: "22P02"}
}

func (invalidTextRepresentationConn) Prepare(string) (driver.Stmt, error) {
	return invalidTextRepresentationStmt{}, nil
}

func (invalidTextRepresentationConn) Close() error { return nil }

func (invalidTextRepresentationConn) Begin() (driver.Tx, error) {
	return invalidTextRepresentationTx{}, nil
}

func (invalidTextRepresentationConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return nil, invalidTextRepresentationError()
}

func (invalidTextRepresentationConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return nil, invalidTextRepresentationError()
}

type invalidTextRepresentationStmt struct{}

func (invalidTextRepresentationStmt) Close() error  { return nil }
func (invalidTextRepresentationStmt) NumInput() int { return -1 }

func (invalidTextRepresentationStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, invalidTextRepresentationError()
}

func (invalidTextRepresentationStmt) Query([]driver.Value) (driver.Rows, error) {
	return nil, invalidTextRepresentationError()
}

type invalidTextRepresentationTx struct{}

func (invalidTextRepresentationTx) Commit() error   { return nil }
func (invalidTextRepresentationTx) Rollback() error { return nil }

func newInvalidTextRepresentationDB(t *testing.T) *gorm.DB {
	t.Helper()
	registerInvalidTextRepresentationDriver.Do(func() {
		sql.Register(invalidTextRepresentationDriverName, invalidTextRepresentationDriver{})
	})
	sqlDB, err := sql.Open(invalidTextRepresentationDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return db
}

func TestClassifyMalformedClientIDAtRepositoryLookups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(context.Context, *gorm.DB) error
		want error
	}{
		{
			name: "cardgroup FindByID",
			run: func(ctx context.Context, db *gorm.DB) error {
				_, err := NewCardgroupRepository(db).FindByID(ctx, "malformed")
				return err
			},
			want: ErrNotFound,
		},
		{
			name: "card FindByID",
			run: func(ctx context.Context, db *gorm.DB) error {
				_, err := NewCardRepository(db).FindByID(ctx, "malformed")
				return err
			},
			want: ErrNotFound,
		},
		{
			name: "card FindByIDForUpdateTx",
			run: func(ctx context.Context, db *gorm.DB) error {
				_, err := NewCardRepository(db).FindByIDForUpdateTx(ctx, db, "malformed")
				return err
			},
			want: ErrNotFound,
		},
		{
			name: "master cardgroup FindByID",
			run: func(ctx context.Context, db *gorm.DB) error {
				_, err := NewMasterCardgroupRepository(db).FindByID(ctx, "malformed")
				return err
			},
			want: ErrNotFound,
		},
		{
			name: "master cardgroup FindPublishedByID",
			run: func(ctx context.Context, db *gorm.DB) error {
				_, err := NewMasterCardgroupRepository(db).FindPublishedByID(ctx, "malformed")
				return err
			},
			want: ErrNotFound,
		},
		{
			name: "master cardgroup FindPublishedByIDTx",
			run: func(ctx context.Context, db *gorm.DB) error {
				_, err := NewMasterCardgroupRepository(db).FindPublishedByIDTx(ctx, db, "malformed")
				return err
			},
			want: ErrNotFound,
		},
		{
			name: "user preference UpsertLastViewedCardgroup",
			run: func(ctx context.Context, db *gorm.DB) error {
				return NewUserPreferenceRepository(db).UpsertLastViewedCardgroup(ctx, "user-id", "malformed")
			},
			want: ErrCardgroupNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.run(context.Background(), newInvalidTextRepresentationDB(t))
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				t.Fatalf("classified error must not retain an internal PostgreSQL error: %v", err)
			}
		})
	}
}
