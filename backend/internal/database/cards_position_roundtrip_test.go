package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"backend/internal/database"
)

// TestCardsPositionDownUpRoundtrip proves that the down migration for
// 20260529090000_add_position_to_cards correctly drops the column and that the
// up migration re-adds it, while leaving existing card data intact.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestCardsPositionDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	requireColumnExists(t, ctx, sqlDB, "cards", "position")

	cardID := insertRoundtripCard(t, ctx, sqlDB)
	assertCardContent(t, ctx, sqlDB, cardID, "front text", "back text")
	assertCardPosition(t, ctx, sqlDB, cardID, 0)

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// Step back past the fifteen migrations newer than add_position_to_cards
	// (enable_rls_schema_migrations, pin_trigger_function_search_path,
	// restrict_definer_function_exposure, add_master_tables,
	// add_learn_display_mode_to_user_preferences,
	// add_user_card_fsrs_card_id_index, master_cards_front_citext,
	// drop_master_cardgroup_metadata_columns,
	// index_hygiene_users_swipe_records,
	// add_new_card_ratio_to_user_preferences,
	// add_pre_swipe_snapshot_to_swipe_records,
	// add_last_rating_to_user_card_fsrs,
	// add_stability_before_to_swipe_records,
	// add_cardgroup_fk_to_swipe_records, widen_text_length_checks), then past
	// add_position_to_cards itself. Sixteen steps are required because
	// add_position_to_cards is no longer near the newest migration; bump this
	// count when adding migrations after it.
	if err := m.Steps(-16); err != nil {
		t.Fatalf("migrate down to before add_position_to_cards: %v", err)
	}

	requireColumnMissing(t, ctx, sqlDB, "cards", "position")
	assertCardContent(t, ctx, sqlDB, cardID, "front text", "back text")

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up one step: %v", err)
	}

	requireColumnExists(t, ctx, sqlDB, "cards", "position")
	assertCardContent(t, ctx, sqlDB, cardID, "front text", "back text")
	assertCardPosition(t, ctx, sqlDB, cardID, 0)
}

// insertRoundtripCard inserts the auth.users, public.cardgroups, and
// public.cards rows required to satisfy foreign-key constraints, then returns
// the new card's ID.
func insertRoundtripCard(t *testing.T, ctx context.Context, sqlDB *sql.DB) string {
	t.Helper()

	userID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO auth.users (id, email) VALUES ($1, $2)`,
		userID, fmt.Sprintf("%s@test", userID)); err != nil {
		t.Fatalf("insert auth.users: %v", err)
	}

	cardgroupID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name) VALUES ($1, $2, $3)`,
		cardgroupID, userID, "Roundtrip Cardgroup"); err != nil {
		t.Fatalf("insert public.cardgroups: %v", err)
	}

	cardID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.cards (id, cardgroup_id, front, back) VALUES ($1, $2, $3, $4)`,
		cardID, cardgroupID, "front text", "back text"); err != nil {
		t.Fatalf("insert public.cards: %v", err)
	}

	return cardID
}

func assertCardContent(t *testing.T, ctx context.Context, sqlDB *sql.DB, cardID, wantFront, wantBack string) {
	t.Helper()

	var front, back string
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT front, back
		FROM public.cards
		WHERE id = $1
	`, cardID).Scan(&front, &back); err != nil {
		t.Fatalf("query public.cards row: %v", err)
	}
	if front != wantFront {
		t.Fatalf("front: got %q, want %q", front, wantFront)
	}
	if back != wantBack {
		t.Fatalf("back: got %q, want %q", back, wantBack)
	}
}

func assertCardPosition(t *testing.T, ctx context.Context, sqlDB *sql.DB, cardID string, want int) {
	t.Helper()

	var got int
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT position
		FROM public.cards
		WHERE id = $1
	`, cardID).Scan(&got); err != nil {
		t.Fatalf("query public.cards.position: %v", err)
	}
	if got != want {
		t.Fatalf("position: got %d, want %d", got, want)
	}
}
