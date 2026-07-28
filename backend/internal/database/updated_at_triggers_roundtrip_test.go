package database_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/database"
)

var updatedAtTriggers = []struct {
	table    string
	trigger  string
	function string
}{
	{"users", "trg_users_set_updated_at", "set_users_updated_at"},
	{"cardgroups", "trg_cardgroups_set_updated_at", "set_cardgroups_updated_at"},
	{"cards", "trg_cards_set_updated_at", "set_cards_updated_at"},
	{"user_card_fsrs", "trg_user_card_fsrs_set_updated_at", "set_user_card_fsrs_updated_at"},
	{"master_cardgroups", "trg_master_cardgroups_set_updated_at", "set_master_cardgroups_updated_at"},
	{"master_cards", "trg_master_cards_set_updated_at", "set_master_cards_updated_at"},
}

// requireUpdatedAtTriggerEvents asserts, for every updated_at trigger, that it
// fires BEFORE ROW on the expected events and still executes its own table's
// trigger function. The tgfoid join is what makes the check meaningful: a
// CREATE OR REPLACE TRIGGER that names the right trigger but binds the wrong
// function would satisfy the event bitmask alone, and only the cardgroups path
// below is proven behaviorally.
func requireUpdatedAtTriggerEvents(t *testing.T, ctx context.Context, sqlDB *sql.DB, wantInsert bool) {
	t.Helper()
	for _, trigger := range updatedAtTriggers {
		var firesOnInsert, firesOnUpdate, firesBeforeRow bool
		var function string
		err := sqlDB.QueryRowContext(ctx, `
			SELECT (tg.tgtype::int & 4) <> 0,
			       (tg.tgtype::int & 16) <> 0,
			       (tg.tgtype::int & 1) <> 0 AND (tg.tgtype::int & 2) <> 0,
			       fn.proname
			FROM pg_trigger AS tg
			JOIN pg_class AS tbl ON tbl.oid = tg.tgrelid
			JOIN pg_namespace AS ns ON ns.oid = tbl.relnamespace
			JOIN pg_proc AS fn ON fn.oid = tg.tgfoid
			WHERE ns.nspname = 'public'
			  AND tbl.relname = $1
			  AND tg.tgname = $2
			  AND NOT tg.tgisinternal
		`, trigger.table, trigger.trigger).Scan(&firesOnInsert, &firesOnUpdate, &firesBeforeRow, &function)
		require.NoError(t, err, "%s trigger metadata", trigger.table)
		require.Equal(t, wantInsert, firesOnInsert, "%s INSERT event", trigger.table)
		require.True(t, firesOnUpdate, "%s UPDATE event", trigger.table)
		require.True(t, firesBeforeRow, "%s must stay BEFORE ROW", trigger.table)
		require.Equal(t, trigger.function, function, "%s trigger function", trigger.table)
	}
}

func databaseNow(t *testing.T, ctx context.Context, sqlDB *sql.DB) time.Time {
	t.Helper()
	var now time.Time
	require.NoError(t, sqlDB.QueryRowContext(ctx, `SELECT now()`).Scan(&now))
	return now
}

func insertCardgroupWithUpdatedAt(
	t *testing.T,
	ctx context.Context,
	sqlDB *sql.DB,
	ownerID string,
	updatedAt time.Time,
) (string, time.Time) {
	t.Helper()
	id := uuid.NewString()
	var persisted time.Time
	err := sqlDB.QueryRowContext(ctx, `
		INSERT INTO public.cardgroups (id, owner_id, name, updated_at)
		VALUES ($1, $2, $3, $4)
		RETURNING updated_at
	`, id, ownerID, "updated-at-roundtrip-"+id, updatedAt).Scan(&persisted)
	require.NoError(t, err)
	return id, persisted
}

func requireWithinDatabaseWindow(t *testing.T, got, before, after time.Time) {
	t.Helper()
	require.False(t, got.Before(before), "updated_at %v is before database baseline %v", got, before)
	require.False(t, got.After(after), "updated_at %v is after database ceiling %v", got, after)
}

// TestUpdatedAtTriggersDownUpRoundtrip proves the newest migration adds INSERT
// to all six updated_at triggers, preserves their UPDATE behavior, restores the
// old insert behavior on down, and reapplies the new behavior on up.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestUpdatedAtTriggersDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	ownerID := insertAuthUserForAdmin(t, ctx, db)
	sentinel := time.Unix(1, 0).UTC()

	requireUpdatedAtTriggerEvents(t, ctx, sqlDB, true)
	beforeInsert := databaseNow(t, ctx, sqlDB)
	cardgroupID, insertedAt := insertCardgroupWithUpdatedAt(t, ctx, sqlDB, ownerID, sentinel)
	afterInsert := databaseNow(t, ctx, sqlDB)
	requireWithinDatabaseWindow(t, insertedAt, beforeInsert, afterInsert)

	_, err := sqlDB.ExecContext(ctx, `SELECT pg_sleep(0.01)`)
	require.NoError(t, err)
	var updatedAt time.Time
	err = sqlDB.QueryRowContext(ctx, `
		UPDATE public.cardgroups
		SET name = name || '-updated'
		WHERE id = $1
		RETURNING updated_at
	`, cardgroupID).Scan(&updatedAt)
	require.NoError(t, err)
	require.True(t, updatedAt.After(insertedAt), "UPDATE must advance updated_at")

	m, err := database.NewMigrateInstanceForTest(testDSN)
	require.NoError(t, err)
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// realign_fsrs_snapshot_columns_to_v4 sits above the trigger migration. Bump
	// this count when adding later migrations.
	require.NoError(t, m.Steps(-2))
	requireUpdatedAtTriggerEvents(t, ctx, sqlDB, false)
	_, legacyInsertedAt := insertCardgroupWithUpdatedAt(t, ctx, sqlDB, ownerID, sentinel)
	require.True(t, sentinel.Equal(legacyInsertedAt), "down migration must preserve supplied updated_at")

	require.NoError(t, m.Steps(1))
	requireUpdatedAtTriggerEvents(t, ctx, sqlDB, true)
	beforeReappliedInsert := databaseNow(t, ctx, sqlDB)
	_, reappliedInsertedAt := insertCardgroupWithUpdatedAt(t, ctx, sqlDB, ownerID, sentinel)
	afterReappliedInsert := databaseNow(t, ctx, sqlDB)
	requireWithinDatabaseWindow(t, reappliedInsertedAt, beforeReappliedInsert, afterReappliedInsert)
}
