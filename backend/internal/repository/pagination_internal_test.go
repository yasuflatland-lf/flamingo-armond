package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// These white-box tests pin the exact SQL strings the per-aggregate cursor
// shims emit through the shared buildOrderClause / buildCursorWhere engine.
// They are the DB-free half of the SQL-equivalence regression net: every
// quirk the generalization had to preserve (card's COALESCE due branch + alias
// "cards", cardgroup's unaliased id, master_card's alias "master_cards",
// master_catalog's always-two-column form with alias "mcg") is asserted here so
// a future change to the generic builders cannot silently alter a clause.

func ptrTime(t time.Time) *time.Time { return &t }
func ptrInt(i int) *int              { return &i }

func TestCursorSpec_IDDirection(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	cases := []struct {
		name      string
		dir       SortOrder
		userSpec  bool
		invertID  bool
		wantOrder string
		wantWhere string
	}{
		{"default forward", SortAsc, false, false, "created_at ASC, id ASC", "(created_at > ? OR (created_at = ? AND id > ?))"},
		{"default backward", SortDesc, false, false, "created_at DESC, id DESC", "(created_at < ? OR (created_at = ? AND id < ?))"},
		{"overridden forward", SortDesc, true, false, "created_at DESC, id ASC", "(created_at < ? OR (created_at = ? AND id > ?))"},
		{"overridden backward", SortAsc, true, true, "created_at ASC, id DESC", "(created_at > ? OR (created_at = ? AND id < ?))"},
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
			if c.invertID {
				spec.idDir = InvertDir(spec.idDir)
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
	cases := []struct {
		orderBy CardOrderBy
		dir     SortOrder
		want    string
	}{
		{CardOrderByID, SortAsc, "cards.id ASC"},
		{CardOrderByID, SortDesc, "cards.id DESC"},
		{CardOrderByCreatedAt, SortAsc, "cards.created_at ASC, cards.id ASC"},
		{CardOrderByCreatedAt, SortDesc, "cards.created_at DESC, cards.id DESC"},
		{CardOrderByUpdatedAt, SortAsc, "cards.updated_at ASC, cards.id ASC"},
		{CardOrderByDue, SortAsc, "COALESCE(ucs.due, cards.created_at) ASC, cards.id ASC"},
		{CardOrderByDue, SortDesc, "COALESCE(ucs.due, cards.created_at) DESC, cards.id DESC"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, orderClause(c.orderBy, c.dir))
	}
}

func TestCursorWhere_Card(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	cur := &CardCursor{ID: "cid", Due: ptrTime(now), CreatedAt: ptrTime(now), UpdatedAt: ptrTime(now)}

	// id-order emits the single id comparison, no tuple.
	clause, args, err := cursorWhere(CardOrderByID, SortAsc, cur)
	require.NoError(t, err)
	require.Equal(t, "cards.id > ?", clause)
	require.Equal(t, []any{"cid"}, args)

	clause, args, err = cursorWhere(CardOrderByID, SortDesc, cur)
	require.NoError(t, err)
	require.Equal(t, "cards.id < ?", clause)
	require.Equal(t, []any{"cid"}, args)

	// created_at tuple form with alias "cards".
	clause, args, err = cursorWhere(CardOrderByCreatedAt, SortAsc, cur)
	require.NoError(t, err)
	require.Equal(t, "(cards.created_at > ? OR (cards.created_at = ? AND cards.id > ?))", clause)
	require.Equal(t, []any{now, now, "cid"}, args)

	// Due keeps the COALESCE primary column.
	clause, args, err = cursorWhere(CardOrderByDue, SortDesc, cur)
	require.NoError(t, err)
	require.Equal(t,
		"(COALESCE(ucs.due, cards.created_at) < ? OR (COALESCE(ucs.due, cards.created_at) = ? AND cards.id < ?))",
		clause)
	require.Equal(t, []any{now, now, "cid"}, args)

	// Missing hydrated column is a caller bug surfaced as an error.
	_, _, err = cursorWhere(CardOrderByCreatedAt, SortAsc, &CardCursor{ID: "cid"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository: card: cursor missing")
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
