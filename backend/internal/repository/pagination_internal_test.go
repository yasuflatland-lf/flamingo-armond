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
	cases := []struct {
		orderBy CardgroupOrderBy
		dir     SortOrder
		want    string
	}{
		{CardgroupOrderByID, SortAsc, "id ASC"},
		{CardgroupOrderByID, SortDesc, "id DESC"},
		{CardgroupOrderByName, SortAsc, "name ASC, id ASC"},
		{CardgroupOrderByCreatedAt, SortDesc, "created_at DESC, id DESC"},
		{CardgroupOrderByUpdatedAt, SortAsc, "updated_at ASC, id ASC"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, cardgroupOrderClause(c.orderBy, c.dir))
	}
}

func TestCursorWhere_Cardgroup(t *testing.T) {
	t.Parallel()
	name := "deck"
	cur := &CardgroupCursor{ID: "gid", Name: &name}

	// id-order: unaliased bare id, no tuple.
	clause, args, err := cardgroupCursorWhere(CardgroupOrderByID, SortAsc, cur)
	require.NoError(t, err)
	require.Equal(t, "id > ?", clause)
	require.Equal(t, []any{"gid"}, args)

	// name tuple form is unaliased on the id tie-break.
	clause, args, err = cardgroupCursorWhere(CardgroupOrderByName, SortDesc, cur)
	require.NoError(t, err)
	require.Equal(t, "(name < ? OR (name = ? AND id < ?))", clause)
	require.Equal(t, []any{name, name, "gid"}, args)

	_, _, err = cardgroupCursorWhere(CardgroupOrderByName, SortAsc, &CardgroupCursor{ID: "gid"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository: cardgroup: cursor missing")
}

func TestOrderClause_MasterCard(t *testing.T) {
	t.Parallel()
	cases := []struct {
		orderBy MasterCardOrderBy
		dir     SortOrder
		want    string
	}{
		{MasterCardOrderByID, SortAsc, "master_cards.id ASC"},
		{MasterCardOrderByID, SortDesc, "master_cards.id DESC"},
		{MasterCardOrderByPosition, SortAsc, "master_cards.position ASC, master_cards.id ASC"},
		{MasterCardOrderByCreatedAt, SortDesc, "master_cards.created_at DESC, master_cards.id DESC"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, masterCardOrderClause(c.orderBy, c.dir))
	}
}

func TestCursorWhere_MasterCard(t *testing.T) {
	t.Parallel()
	cur := &MasterCardCursor{ID: "mid", Position: ptrInt(7)}

	clause, args, err := masterCardCursorWhere(MasterCardOrderByID, SortDesc, cur)
	require.NoError(t, err)
	require.Equal(t, "master_cards.id < ?", clause)
	require.Equal(t, []any{"mid"}, args)

	clause, args, err = masterCardCursorWhere(MasterCardOrderByPosition, SortAsc, cur)
	require.NoError(t, err)
	require.Equal(t,
		"(master_cards.position > ? OR (master_cards.position = ? AND master_cards.id > ?))",
		clause)
	require.Equal(t, []any{7, 7, "mid"}, args)

	_, _, err = masterCardCursorWhere(MasterCardOrderByPosition, SortAsc, &MasterCardCursor{ID: "mid"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository: master card: cursor missing")
}

func TestOrderClause_MasterCatalog(t *testing.T) {
	t.Parallel()
	// MasterCatalogOrderBy has no id member, so both columns are always emitted.
	cases := []struct {
		orderBy MasterCatalogOrderBy
		dir     SortOrder
		want    string
	}{
		{MasterCatalogOrderBySortOrder, SortAsc, "mcg.sort_order ASC, mcg.id ASC"},
		{MasterCatalogOrderBySortOrder, SortDesc, "mcg.sort_order DESC, mcg.id DESC"},
		{MasterCatalogOrderByName, SortAsc, "mcg.name ASC, mcg.id ASC"},
		{MasterCatalogOrderByCreatedAt, SortDesc, "mcg.created_at DESC, mcg.id DESC"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, masterCatalogOrderClause(c.orderBy, c.dir))
	}
}

func TestCursorWhere_MasterCatalog(t *testing.T) {
	t.Parallel()
	cur := &MasterCatalogCursor{ID: "cgid", SortOrder: ptrInt(3)}

	// Always a tuple — no id-order short-circuit for this aggregate.
	clause, args, err := masterCatalogCursorWhere(MasterCatalogOrderBySortOrder, SortAsc, cur)
	require.NoError(t, err)
	require.Equal(t,
		"(mcg.sort_order > ? OR (mcg.sort_order = ? AND mcg.id > ?))",
		clause)
	require.Equal(t, []any{3, 3, "cgid"}, args)

	_, _, err = masterCatalogCursorWhere(MasterCatalogOrderBySortOrder, SortAsc, &MasterCatalogCursor{ID: "cgid"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository: master cardgroup: cursor missing")
}
