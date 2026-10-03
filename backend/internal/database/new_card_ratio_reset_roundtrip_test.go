package database_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"backend/internal/database"
)

// seedUpdatedAt is the updated_at every seeded row starts with; a row the reset
// rewrites moves past it, a row it leaves alone keeps it.
var seedUpdatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// TestResetLegacyNewCardRatio_DownUpRoundtrip pins reset_legacy_new_card_ratio
// against a real Postgres: the down is a no-op, and reapplying the up rewrites
// every stored 4/5 (reduced or not) to 1/5 while leaving other ratios untouched.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestResetLegacyNewCardRatio_DownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)

	type seeded struct {
		userID   string
		num, den int
		reset    bool
	}
	rows := []seeded{
		{num: 4, den: 5, reset: true},
		{num: 8, den: 10, reset: true},
		{num: 16, den: 20, reset: true},
		{num: 1, den: 5},
		{num: 3, den: 5},
		{num: 3, den: 4}, // close to 80% but not 4/5
		{num: 1, den: 2}, // neither 1/5 nor 4/5
	}
	for i := range rows {
		rows[i].userID = seedRatioRow(t, ctx, sqlDB, rows[i].num, rows[i].den)
	}

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// drop_ping_records_updated_at, cardgroups_owner_updated_at_index,
	// drop_redundant_fk_indexes, drop_user_card_fsrs_last_rating and
	// drop_swipe_records_user_cardgroup_index sit above
	// reset_legacy_new_card_ratio, so six steps reach it.
	if err := m.Steps(-6); err != nil {
		t.Fatalf("migrate down reset_legacy_new_card_ratio: %v", err)
	}
	for _, r := range rows {
		requireStoredRatio(t, ctx, sqlDB, r.userID, r.num, r.den, true)
	}

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up reset_legacy_new_card_ratio: %v", err)
	}
	for _, r := range rows {
		if r.reset {
			requireStoredRatio(t, ctx, sqlDB, r.userID, 1, 5, false)
			continue
		}
		requireStoredRatio(t, ctx, sqlDB, r.userID, r.num, r.den, true)
	}

	requireDefaultRatio(t, ctx, sqlDB, 1, 5)
}

// TestResetLegacyNewCardRatio_RewritesLegacyUnreducedRow seeds non-reduced 4/5
// rows that only the pre-tighten CHECK admits and proves the reset still
// rewrites them once the migrations are reapplied.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestResetLegacyNewCardRatio_RewritesLegacyUnreducedRow(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// Step back nine migrations newest-first:
	// drop_ping_records_updated_at, cardgroups_owner_updated_at_index,
	// drop_redundant_fk_indexes, drop_user_card_fsrs_last_rating,
	// drop_swipe_records_user_cardgroup_index, reset_legacy_new_card_ratio,
	// revoke_client_writes, lower_new_card_ratio_default, then
	// tighten_new_card_ratio_check, so the looser den <= 100 CHECK is in force.
	// Bump this count when adding migrations after tighten_new_card_ratio_check.
	if err := m.Steps(-9); err != nil {
		t.Fatalf("migrate down to before tighten_new_card_ratio_check: %v", err)
	}

	legacy12of15 := seedRatioRow(t, ctx, sqlDB, 12, 15)
	legacy40of50 := seedRatioRow(t, ctx, sqlDB, 40, 50)
	control19of20 := seedRatioRow(t, ctx, sqlDB, 19, 20)

	if err := database.Migrate(testDSN); err != nil {
		t.Fatalf("migrate up to latest: %v", err)
	}

	requireStoredRatio(t, ctx, sqlDB, legacy12of15, 1, 5, false)
	requireStoredRatio(t, ctx, sqlDB, legacy40of50, 1, 5, false)
	requireStoredRatio(t, ctx, sqlDB, control19of20, 19, 20, true)
}

// seedRatioRow inserts a user_preferences row carrying num/den and the
// seedUpdatedAt sentinel for a fresh user and returns that user's id.
func seedRatioRow(t *testing.T, ctx context.Context, sqlDB *sql.DB, num, den int) string {
	t.Helper()
	userID := insertRLSAuthUser(t, ctx, sqlDB)
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO public.user_preferences (user_id, new_card_ratio_num, new_card_ratio_den, updated_at)
		VALUES ($1, $2, $3, $4)
	`, userID, num, den, seedUpdatedAt); err != nil {
		t.Fatalf("seed user_preferences %d/%d: %v", num, den, err)
	}
	return userID
}

// requireStoredRatio asserts the stored ratio and whether updated_at still holds
// seedUpdatedAt (row untouched) or has moved past it (row rewritten).
func requireStoredRatio(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID string, wantNum, wantDen int, wantUpdatedAtSentinel bool) {
	t.Helper()
	var gotNum, gotDen int
	var updatedAt time.Time
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT new_card_ratio_num, new_card_ratio_den, updated_at
		FROM public.user_preferences
		WHERE user_id = $1
	`, userID).Scan(&gotNum, &gotDen, &updatedAt); err != nil {
		t.Fatalf("read user_preferences %s: %v", userID, err)
	}
	if gotNum != wantNum || gotDen != wantDen {
		t.Fatalf("stored ratio for %s: got %d/%d, want %d/%d", userID, gotNum, gotDen, wantNum, wantDen)
	}
	if wantUpdatedAtSentinel && !updatedAt.Equal(seedUpdatedAt) {
		t.Fatalf("updated_at for %s: got %s, want the untouched sentinel %s", userID, updatedAt, seedUpdatedAt)
	}
	if !wantUpdatedAtSentinel && !updatedAt.After(seedUpdatedAt) {
		t.Fatalf("updated_at for %s: got %s, want later than the sentinel %s", userID, updatedAt, seedUpdatedAt)
	}
}
