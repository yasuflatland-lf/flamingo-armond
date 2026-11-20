package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// These tests pin the SQL strings emitted through the shared cursor builders,
// including each aggregate's aliases and tie-break columns.

func ptrTime(t time.Time) *time.Time { return &t }
func ptrInt(i int) *int              { return &i }

func TestCursorSpec_IDDirection(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	cases := []struct {
		name      string
		dir       SortOrder
		userSpec  bool
		wantOrder string
		wantWhere string
	}{
		{"default ASC", SortAsc, false, "created_at ASC, id ASC", "(created_at > ? OR (created_at = ? AND id > ?))"},
		{"default DESC", SortDesc, false, "created_at DESC, id DESC", "(created_at < ? OR (created_at = ? AND id < ?))"},
		{"user spec id ASC override", SortDesc, true, "created_at DESC, id ASC", "(created_at < ? OR (created_at = ? AND id > ?))"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec := cursorSpec{
				orderCol:   "created_at",
				fieldValue: func() (any, error) { return now, nil },
			}
			if c.userSpec {
				spec = userCursorSpec(gormUser{ID: "uid", CreatedAt: now})
			}
			require.Equal(t, c.wantOrder, buildOrderClause(spec, c.dir))
			clause, args, err := buildCursorWhere(spec, c.dir, "uid")
			require.NoError(t, err)
			require.Equal(t, c.wantWhere, clause)
			require.Equal(t, []any{now, now, "uid"}, args)
		})
	}
}

func TestOrderClause_Card(t *testing.T) {
	t.Parallel()
	require.Equal(t, "cards.id ASC", orderClause())
}

func TestCursorWhere_Card(t *testing.T) {
	t.Parallel()
	clause, args, err := cursorWhere(&CardCursor{ID: "cid"})
	require.NoError(t, err)
	require.Equal(t, "cards.id > ?", clause)
	require.Equal(t, []any{"cid"}, args)
}

func TestOrderClause_Cardgroup(t *testing.T) {
	t.Parallel()
	require.Equal(t, "updated_at DESC, id DESC", cardgroupOrderClause())
}

func TestCursorWhere_Cardgroup(t *testing.T) {
	t.Parallel()
	ua := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	cur := &CardgroupCursor{ID: "gid", UpdatedAt: ptrTime(ua)}

	// The tuple form is unaliased on the id tie-break.
	clause, args, err := cardgroupCursorWhere(cur)
	require.NoError(t, err)
	require.Equal(t, "(updated_at < ? OR (updated_at = ? AND id < ?))", clause)
	require.Equal(t, []any{ua, ua, "gid"}, args)

	_, _, err = cardgroupCursorWhere(&CardgroupCursor{ID: "gid"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository: cardgroup: cursor missing")
}

func TestOrderClause_MasterCard(t *testing.T) {
	t.Parallel()
	require.Equal(t, "master_cards.position ASC, master_cards.id ASC", masterCardOrderClause())
}

func TestCursorWhere_MasterCard(t *testing.T) {
	t.Parallel()
	cur := &MasterCardCursor{ID: "mid", Position: ptrInt(7)}

	clause, args, err := masterCardCursorWhere(cur)
	require.NoError(t, err)
	require.Equal(t,
		"(master_cards.position > ? OR (master_cards.position = ? AND master_cards.id > ?))",
		clause)
	require.Equal(t, []any{7, 7, "mid"}, args)

	_, _, err = masterCardCursorWhere(&MasterCardCursor{ID: "mid"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository: master card: cursor missing")
}

func TestOrderClause_MasterCatalog(t *testing.T) {
	t.Parallel()
	require.Equal(t, "mcg.sort_order ASC, mcg.id ASC", masterCatalogOrderClause())
}

func TestCursorWhere_MasterCatalog(t *testing.T) {
	t.Parallel()
	cur := &MasterCatalogCursor{ID: "cgid", SortOrder: ptrInt(3)}

	clause, args, err := masterCatalogCursorWhere(cur)
	require.NoError(t, err)
	require.Equal(t,
		"(mcg.sort_order > ? OR (mcg.sort_order = ? AND mcg.id > ?))",
		clause)
	require.Equal(t, []any{3, 3, "cgid"}, args)

	_, _, err = masterCatalogCursorWhere(&MasterCatalogCursor{ID: "cgid"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository: master cardgroup: cursor missing")
}
