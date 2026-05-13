package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

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
        ALTER DEFAULT PRIVILEGES IN SCHEMA public
            GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public
            GRANT USAGE, SELECT ON SEQUENCES TO authenticated;
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
		`TRUNCATE public.user_card_fsrs, public.cards, public.cardgroups, auth.users CASCADE`)
	require.NoError(t, err)
}

func TestDump_WritesValidJSON(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	ctx := context.Background()
	userID := uuid.New().String()
	email := "dump@example.com"
	insertAuthUser(t, pool, userID, email)

	cgID := uuid.New().String()
	_, err := pool.Exec(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name, created_at, updated_at)
		 VALUES ($1, $2, $3, now(), now())`,
		cgID, userID, "Test Group")
	require.NoError(t, err)

	cardID := uuid.New().String()
	_, err = pool.Exec(ctx,
		`INSERT INTO public.cards (id, cardgroup_id, front, back, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, now(), now())`,
		cardID, cgID, "Front text", "Back text")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO public.user_card_fsrs
		(user_id, card_id, state, due, stability, difficulty, reps, lapses, last_review, elapsed_days, scheduled_days)
		VALUES ($1, $2, 0, now(), 1.0, 5.0, 0, 0, now(), 0, 0)`,
		userID, cardID)
	require.NoError(t, err)

	tmpFile := t.TempDir() + "/dump.json"
	err = runDump(testDBURL, tmpFile)
	require.NoError(t, err)

	data, err := os.ReadFile(tmpFile)
	require.NoError(t, err)

	var df DumpFile
	err = json.Unmarshal(data, &df)
	require.NoError(t, err)

	assert.Equal(t, 1, df.Version)
	assert.Len(t, df.Cardgroups, 1)
	assert.Len(t, df.Cards, 1)
	assert.Len(t, df.UserCardFSRS, 1)
	require.Len(t, df.UserMap, 1)
	assert.Equal(t, email, df.UserMap[0].Email)
}

func TestImport_Idempotent(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	ctx := context.Background()
	userID := uuid.New().String()
	insertAuthUser(t, pool, userID, "idempotent@example.com")

	cgID := uuid.New().String()
	_, err := pool.Exec(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name, created_at, updated_at)
		 VALUES ($1, $2, $3, now(), now())`,
		cgID, userID, "Idempotent Group")
	require.NoError(t, err)

	cardID := uuid.New().String()
	_, err = pool.Exec(ctx,
		`INSERT INTO public.cards (id, cardgroup_id, front, back, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, now(), now())`,
		cardID, cgID, "Front", "Back")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO public.user_card_fsrs
		(user_id, card_id, state, due, stability, difficulty, reps, lapses, last_review, elapsed_days, scheduled_days)
		VALUES ($1, $2, 0, now(), 1.0, 5.0, 0, 0, now(), 0, 0)`,
		userID, cardID)
	require.NoError(t, err)

	tmpFile := t.TempDir() + "/dump.json"
	err = runDump(testDBURL, tmpFile)
	require.NoError(t, err)

	err = runImport(testDBURL, tmpFile)
	require.NoError(t, err)

	var cgCount, cardCount, fsrsCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.cardgroups`).Scan(&cgCount)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.cards`).Scan(&cardCount)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.user_card_fsrs`).Scan(&fsrsCount)
	require.NoError(t, err)

	assert.Equal(t, 1, cgCount)
	assert.Equal(t, 1, cardCount)
	assert.Equal(t, 1, fsrsCount)

	err = runImport(testDBURL, tmpFile)
	require.NoError(t, err)

	var cgCount2, cardCount2, fsrsCount2 int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.cardgroups`).Scan(&cgCount2)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.cards`).Scan(&cardCount2)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.user_card_fsrs`).Scan(&fsrsCount2)
	require.NoError(t, err)

	assert.Equal(t, cgCount, cgCount2)
	assert.Equal(t, cardCount, cardCount2)
	assert.Equal(t, fsrsCount, fsrsCount2)
}

func TestImport_SkipsUnknownEmail(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	unknownUserID := uuid.New().String()
	cgID := uuid.New().String()
	now := time.Now().UTC()

	df := DumpFile{
		Version:  1,
		DumpedAt: now,
		UserMap: []UserEntry{
			{SourceUUID: unknownUserID, Email: "unknown@example.com"},
		},
		Cardgroups: []CGEntry{
			{
				ID:        cgID,
				OwnerID:   unknownUserID,
				Name:      "Should be skipped",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Cards:        []CardEntry{},
		UserCardFSRS: []FSRSEntry{},
	}

	data, err := json.MarshalIndent(df, "", "  ")
	require.NoError(t, err)

	tmpFile := t.TempDir() + "/dump.json"
	err = os.WriteFile(tmpFile, data, 0o644)
	require.NoError(t, err)

	err = runImport(testDBURL, tmpFile)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM public.cardgroups`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestImport_RemapsOwnerID(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	ctx := context.Background()

	// Build the dump file directly without writing to the live DB, so we avoid
	// truncate cascades that would also remove public.users rows (because
	// public.users.last_viewed_cardgroup_id references public.cardgroups, so
	// TRUNCATE public.cardgroups CASCADE transitively truncates public.users).
	sourceID := uuid.New().String()
	cgID := uuid.New().String()
	now := time.Now().UTC()

	df := DumpFile{
		Version:  1,
		DumpedAt: now,
		UserMap: []UserEntry{
			{SourceUUID: sourceID, Email: "remap@example.com"},
		},
		Cardgroups: []CGEntry{
			{
				ID:        cgID,
				OwnerID:   sourceID,
				Name:      "Remap Group",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Cards:        []CardEntry{},
		UserCardFSRS: []FSRSEntry{},
	}

	data, err := json.MarshalIndent(df, "", "  ")
	require.NoError(t, err)

	tmpFile := t.TempDir() + "/dump.json"
	err = os.WriteFile(tmpFile, data, 0o644)
	require.NoError(t, err)

	// Insert the target user (different UUID, same email as the dump's source user).
	targetID := uuid.New().String()
	insertAuthUser(t, pool, targetID, "remap@example.com")

	err = runImport(testDBURL, tmpFile)
	require.NoError(t, err)

	var ownerID string
	err = pool.QueryRow(ctx,
		`SELECT owner_id FROM public.cardgroups WHERE id = $1`, cgID).Scan(&ownerID)
	require.NoError(t, err, "cardgroup should have been inserted")

	assert.Equal(t, targetID, ownerID, "owner_id should be remapped to the target UUID")
	assert.NotEqual(t, sourceID, ownerID, "owner_id must not remain as the source UUID")
}

func TestImport_InvalidJSON(t *testing.T) {
	tmpFile := t.TempDir() + "/dump.json"
	err := os.WriteFile(tmpFile, []byte("{not valid json"), 0o644)
	require.NoError(t, err)

	err = runImport(testDBURL, tmpFile)
	require.Error(t, err)
}

func TestImport_VersionMismatch(t *testing.T) {
	for _, version := range []int{0, 2, 99} {
		df := DumpFile{Version: version, DumpedAt: time.Now().UTC()}
		data, err := json.MarshalIndent(df, "", "  ")
		require.NoError(t, err)

		tmpFile := t.TempDir() + "/dump.json"
		err = os.WriteFile(tmpFile, data, 0o644)
		require.NoError(t, err)

		err = runImport(testDBURL, tmpFile)
		require.Error(t, err)
	}
}

func TestImport_RemapsFSRSUserID(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	ctx := context.Background()
	sourceID := uuid.New().String()
	targetID := uuid.New().String()
	cgID := uuid.New().String()
	cardID := uuid.New().String()
	now := time.Now().UTC()

	insertAuthUser(t, pool, targetID, "fsrs-remap@example.com")

	df := DumpFile{
		Version:  1,
		DumpedAt: now,
		UserMap:  []UserEntry{{SourceUUID: sourceID, Email: "fsrs-remap@example.com"}},
		Cardgroups: []CGEntry{{
			ID: cgID, OwnerID: sourceID, Name: "FSRS Remap Group",
			CreatedAt: now, UpdatedAt: now,
		}},
		Cards: []CardEntry{{
			ID: cardID, CardgroupID: cgID, Front: "Q", Back: "A",
			CreatedAt: now, UpdatedAt: now,
		}},
		UserCardFSRS: []FSRSEntry{{
			UserID: sourceID, CardID: cardID, State: 0,
			Due: now, Stability: 1.0, Difficulty: 5.0,
			Reps: 0, Lapses: 0, LastReview: now,
			ElapsedDays: 0, ScheduledDays: 0,
			CreatedAt: now, UpdatedAt: now,
		}},
	}

	data, err := json.MarshalIndent(df, "", "  ")
	require.NoError(t, err)
	tmpFile := t.TempDir() + "/dump.json"
	err = os.WriteFile(tmpFile, data, 0o644)
	require.NoError(t, err)

	err = runImport(testDBURL, tmpFile)
	require.NoError(t, err)

	var fsrsUserID string
	err = pool.QueryRow(ctx,
		`SELECT user_id FROM public.user_card_fsrs WHERE card_id = $1`, cardID).Scan(&fsrsUserID)
	require.NoError(t, err, "fsrs row should have been inserted")
	assert.Equal(t, targetID, fsrsUserID, "user_id should be remapped to target UUID")
	assert.NotEqual(t, sourceID, fsrsUserID)
}

func TestImport_SkipsCascade(t *testing.T) {
	pool := openPool(t)
	cleanTables(t, pool)

	unknownUserID := uuid.New().String()
	knownUserID := uuid.New().String()
	knownTargetID := uuid.New().String()
	cgID := uuid.New().String()
	cardID := uuid.New().String()
	now := time.Now().UTC()

	insertAuthUser(t, pool, knownTargetID, "known@example.com")

	df := DumpFile{
		Version:  1,
		DumpedAt: now,
		UserMap: []UserEntry{
			{SourceUUID: unknownUserID, Email: "unknown@example.com"},
			{SourceUUID: knownUserID, Email: "known@example.com"},
		},
		Cardgroups: []CGEntry{{
			ID: cgID, OwnerID: unknownUserID, Name: "Skipped Group",
			CreatedAt: now, UpdatedAt: now,
		}},
		Cards: []CardEntry{{
			ID: cardID, CardgroupID: cgID, Front: "Q", Back: "A",
			CreatedAt: now, UpdatedAt: now,
		}},
		UserCardFSRS: []FSRSEntry{
			{
				UserID: unknownUserID, CardID: cardID, State: 0,
				Due: now, Stability: 1.0, Difficulty: 5.0,
				Reps: 0, Lapses: 0, LastReview: now,
				ElapsedDays: 0, ScheduledDays: 0,
				CreatedAt: now, UpdatedAt: now,
			},
			{
				UserID: knownUserID, CardID: cardID, State: 0,
				Due: now, Stability: 1.0, Difficulty: 5.0,
				Reps: 0, Lapses: 0, LastReview: now,
				ElapsedDays: 0, ScheduledDays: 0,
				CreatedAt: now, UpdatedAt: now,
			},
		},
	}

	data, err := json.MarshalIndent(df, "", "  ")
	require.NoError(t, err)
	tmpFile := t.TempDir() + "/dump.json"
	err = os.WriteFile(tmpFile, data, 0o644)
	require.NoError(t, err)

	err = runImport(testDBURL, tmpFile)
	require.NoError(t, err)

	ctx := context.Background()

	var cgCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.cardgroups`).Scan(&cgCount)
	require.NoError(t, err)
	assert.Equal(t, 0, cgCount, "cardgroup owned by unknown user should be skipped")

	var cardCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.cards`).Scan(&cardCount)
	require.NoError(t, err)
	assert.Equal(t, 0, cardCount, "card in skipped cardgroup should be cascaded-skipped")

	var fsrsCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.user_card_fsrs`).Scan(&fsrsCount)
	require.NoError(t, err)
	assert.Equal(t, 0, fsrsCount, "fsrs rows should be skipped (card not inserted + user skipped)")
}
