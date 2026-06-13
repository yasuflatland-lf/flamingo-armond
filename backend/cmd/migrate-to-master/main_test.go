package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"backend/internal/database"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var testDBURL string

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"postgres:15-alpine",
		tcpostgres.WithDatabase("flamingo_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcpostgres run: %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Fprintf(os.Stderr, "terminate container: %v\n", err)
		}
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "conn string: %v\n", err)
		return 1
	}
	if err := bootstrapAuthSchema(ctx, dsn); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		return 1
	}
	if err := database.Migrate(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		return 1
	}
	testDBURL = dsn
	return m.Run()
}

func bootstrapAuthSchema(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `
        DO $$
        BEGIN
            IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
                CREATE ROLE authenticated LOGIN PASSWORD 'test';
            END IF;
        END
        $$;
        CREATE SCHEMA IF NOT EXISTS auth;
        CREATE TABLE IF NOT EXISTS auth.users (
            id uuid PRIMARY KEY,
            email text
        );
        CREATE OR REPLACE FUNCTION auth.uid()
        RETURNS uuid
        LANGUAGE sql
        STABLE
        AS $$
            SELECT COALESCE(
                NULLIF(current_setting('request.jwt.claim.sub', true), ''),
                NULLIF(current_setting('request.jwt.claims', true), '')::jsonb ->> 'sub'
            )::uuid
        $$;
        GRANT USAGE ON SCHEMA auth TO authenticated;
        GRANT EXECUTE ON FUNCTION auth.uid() TO authenticated;
        GRANT USAGE ON SCHEMA public TO authenticated;
    `)
	return err
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(testDBURL)
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func insertAuthUser(t *testing.T, pool *pgxpool.Pool, id, email string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO auth.users(id, email) VALUES ($1, $2)`,
		id, email)
	require.NoError(t, err)
}

func cleanTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`TRUNCATE public.master_cards, public.master_cardgroups,
		          public.user_card_fsrs, public.cards, public.cardgroups,
		          auth.users CASCADE`)
	require.NoError(t, err)
}

func countRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM `+table).Scan(&n)
	require.NoError(t, err)
	return n
}

// seedOwnerDeck inserts a user plus one cardgroup with two cards, returning the
// owner email, cardgroup id and card ids. The personal deck is what runMigrate
// snapshots into the master tables.
func seedOwnerDeck(t *testing.T, pool *pgxpool.Pool, email string) (cgID string, cardIDs []string) {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New().String()
	insertAuthUser(t, pool, userID, email)

	cgID = uuid.New().String()
	_, err := pool.Exec(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name, created_at, updated_at)
		 VALUES ($1, $2, $3, now(), now())`,
		cgID, userID, "Owner Deck")
	require.NoError(t, err)

	for i, front := range []string{"Front A", "Front B"} {
		cardID := uuid.New().String()
		_, err = pool.Exec(ctx,
			`INSERT INTO public.cards (id, cardgroup_id, front, back, position, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, now(), now())`,
			cardID, cgID, front, "Back", i)
		require.NoError(t, err)
		cardIDs = append(cardIDs, cardID)
	}
	return cgID, cardIDs
}

func TestRunMigrate_CopiesDeckIntoMasterTables(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	email := "run@example.com"
	cgID, cardIDs := seedOwnerDeck(t, pool, email)

	require.NoError(t, runMigrate(testDBURL, email))

	// master_cardgroups carries the source id and the fixed catalog metadata.
	var (
		name             string
		source           string
		version          int
		status           string
		isDefaultStarter bool
		sortOrder        int
	)
	err := pool.QueryRow(context.Background(),
		`SELECT name, source, version, status, is_default_starter, sort_order
		   FROM public.master_cardgroups WHERE id = $1`, cgID).
		Scan(&name, &source, &version, &status, &isDefaultStarter, &sortOrder)
	require.NoError(t, err, "master_cardgroup should exist with the source id")
	assert.Equal(t, "Owner Deck", name)
	assert.Equal(t, "notion", source)
	assert.Equal(t, 1, version)
	assert.Equal(t, "published", status)
	assert.True(t, isDefaultStarter)
	assert.Equal(t, 0, sortOrder)

	assert.Equal(t, 2, countRows(t, pool, "public.master_cards"))
	for _, cardID := range cardIDs {
		var mcgID string
		err := pool.QueryRow(context.Background(),
			`SELECT master_cardgroup_id FROM public.master_cards WHERE id = $1`, cardID).Scan(&mcgID)
		require.NoError(t, err, "master_card should carry the source card id")
		assert.Equal(t, cgID, mcgID, "master_cardgroup_id should be the source cardgroup id")
	}
}

func TestRunMigrate_DoesNotDeletePersonalRows(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	email := "keep@example.com"
	seedOwnerDeck(t, pool, email)

	require.NoError(t, runMigrate(testDBURL, email))

	assert.Equal(t, 1, countRows(t, pool, "public.cardgroups"), "personal cardgroups must be kept")
	assert.Equal(t, 2, countRows(t, pool, "public.cards"), "personal cards must be kept")
}

func TestRunMigrate_Idempotent(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	email := "idempotent@example.com"
	seedOwnerDeck(t, pool, email)

	require.NoError(t, runMigrate(testDBURL, email))
	cgCount := countRows(t, pool, "public.master_cardgroups")
	cardCount := countRows(t, pool, "public.master_cards")

	require.NoError(t, runMigrate(testDBURL, email))
	assert.Equal(t, cgCount, countRows(t, pool, "public.master_cardgroups"), "re-run must not duplicate cardgroups")
	assert.Equal(t, cardCount, countRows(t, pool, "public.master_cards"), "re-run must not duplicate cards")
}

func TestRunMigrate_EmailCaseInsensitive(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	seedOwnerDeck(t, pool, "case@example.com")

	require.NoError(t, runMigrate(testDBURL, "CASE@Example.COM"))
	assert.Equal(t, 1, countRows(t, pool, "public.master_cardgroups"))
}

func TestRunVerify_ReportsParityAfterRun(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	email := "verify@example.com"
	seedOwnerDeck(t, pool, email)

	// Before the run, parity fails (master tables are empty).
	require.Error(t, runVerify(testDBURL, email), "verify must fail before the migration runs")

	require.NoError(t, runMigrate(testDBURL, email))
	require.NoError(t, runVerify(testDBURL, email), "verify must report parity after the migration runs")
}

func TestRunVerify_IsReadOnly(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	email := "readonly@example.com"
	seedOwnerDeck(t, pool, email)
	require.NoError(t, runMigrate(testDBURL, email))

	before := countRows(t, pool, "public.master_cards")
	require.NoError(t, runVerify(testDBURL, email))
	assert.Equal(t, before, countRows(t, pool, "public.master_cards"), "verify must not write")
}

func TestRunMigrate_MissingFlags(t *testing.T) {
	require.Error(t, runMigrate("", "owner@example.com"), "missing db-url must error")
	require.Error(t, runMigrate(testDBURL, ""), "missing owner-email must error")
}

func TestRunMigrate_UnknownEmail(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	err := runMigrate(testDBURL, "nobody@example.com")
	require.Error(t, err, "unknown owner email must error")
}
