package database_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/database"
)

// TestMasterCardgroupsFieldChecks_DownUpRoundtrip proves the up migration adds
// the length CHECK constraints (an over-long description is rejected), the down
// migration removes them (the same insert succeeds), and re-applying up restores
// them. t.Parallel() is intentionally absent: it runs a global migration step.
func TestMasterCardgroupsFieldChecks_DownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	longDesc := strings.Repeat("a", 1001)

	// Constraint present: an over-long description is rejected.
	_, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.master_cardgroups (name, description) VALUES ($1, $2)`,
		"CheckDeck", longDesc)
	require.Error(t, err, "over-long description must violate the CHECK constraint")

	m, err := database.NewMigrateInstanceForTest(testDSN)
	require.NoError(t, err)
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// Step the newest migration down -> constraints dropped.
	require.NoError(t, m.Steps(-1))

	res, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.master_cardgroups (name, description) VALUES ($1, $2)`,
		"CheckDeckDown", longDesc)
	require.NoError(t, err, "after down the over-long description must be accepted")
	_ = res

	// Remove the violating row so re-applying up (ADD CONSTRAINT validates
	// existing rows) does not fail.
	_, err = sqlDB.ExecContext(ctx,
		`DELETE FROM public.master_cardgroups WHERE name = $1`, "CheckDeckDown")
	require.NoError(t, err)

	// Re-apply up -> constraints back.
	require.NoError(t, m.Steps(1))

	_, err = sqlDB.ExecContext(ctx,
		`INSERT INTO public.master_cardgroups (name, description) VALUES ($1, $2)`,
		"CheckDeckUp", longDesc)
	require.Error(t, err, "after re-applying up the CHECK constraint must be enforced again")
}
