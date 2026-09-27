package repository_test

// Docker-gated integration tests proving the three layers agree on one
// user-visible length rule. TestMain, testDB, insertAuthUser, newCardgroup,
// newCard, and insertCardgroup are defined in sibling test files and shared
// across the package.

import (
	"context"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// zwjFamily is one grapheme cluster and seven code points
// (MAN ZWJ WOMAN ZWJ GIRL ZWJ BOY). It is the canonical case where Postgres
// char_length() and uniseg.GraphemeClusterCount() disagree by 7x, which is what
// made the pre-widening CHECK reject text the domain accepted.
const zwjFamily = "\U0001F468‍\U0001F469‍\U0001F467‍\U0001F466"

// maxGraphemeText returns n repetitions of zwjFamily: n grapheme clusters and
// 7*n code points.
func maxGraphemeText(n int) string { return strings.Repeat(zwjFamily, n) }

// TestZWJEmojiAtGraphemeCap_CodePointExpansion is the arithmetic precondition
// for the rest of this file: a card-cap-sized emoji string is 500 graphemes but
// 3500 code points, so any DB bound at or below 3500 would reject it.
func TestZWJEmojiAtGraphemeCap_CodePointExpansion(t *testing.T) {
	t.Parallel()
	s := maxGraphemeText(domain.CardTextMax)
	require.Equal(t, domain.CardTextMax, uniseg.GraphemeClusterCount(s))
	require.Equal(t, domain.CardTextMax*7, len([]rune(s)))
}

// TestCardRepository_ZWJEmojiAtGraphemeCap_RoundTrips proves the widened
// cards_front_length / cards_back_length CHECKs accept a value at exactly the
// domain grapheme cap through both create and update.
func TestCardRepository_ZWJEmojiAtGraphemeCap_RoundTrips(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	capText := maxGraphemeText(domain.CardTextMax)
	card := newCard(cg.ID, capText, capText)
	require.NoError(t, repo.Create(ctx, card))

	got, err := repo.FindByID(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, capText, got.Front.String())
	require.Equal(t, capText, got.Back.String())

	// The update path writes through a different statement (GORM Updates), so it
	// needs its own assertion against the same constraint.
	newFront := maxGraphemeText(domain.CardTextMax-1) + "a"
	updated, err := repo.Update(ctx, card.ID, repository.CardUpdate{Front: &newFront})
	require.NoError(t, err)
	require.Equal(t, newFront, updated.Front.String())
}

// TestCardgroupRepository_ZWJEmojiAtGraphemeCap_RoundTrips proves the same for
// the deck-name constraint, whose cap (100) and multiplier differ from cards.
func TestCardgroupRepository_ZWJEmojiAtGraphemeCap_RoundTrips(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	capName := maxGraphemeText(domain.CardgroupNameMax)
	cg := newCardgroup(ownerID, capName)
	require.NoError(t, repo.Create(ctx, cg))

	got, err := repo.FindByID(ctx, string(cg.ID))
	require.NoError(t, err)
	require.Equal(t, capName, got.Name.String())

	newName := maxGraphemeText(domain.CardgroupNameMax-1) + "a"
	updated, err := repo.Update(ctx, string(cg.ID), repository.CardgroupUpdate{Name: &newName})
	require.NoError(t, err)
	require.Equal(t, newName, updated.Name.String())
}

// TestDomainRejectsOverCapEmojiBeforeReachingDB pins the layer split the
// widening depends on: the domain, not the database, is the gate that rejects
// over-length text. One grapheme past the cap fails at ParseCardText /
// ParseCardgroupName, so no over-length value is ever handed to the repository.
func TestDomainRejectsOverCapEmojiBeforeReachingDB(t *testing.T) {
	t.Parallel()

	_, err := domain.ParseCardText(
		maxGraphemeText(domain.CardTextMax+1),
		domain.ErrCardFrontRequired,
		domain.ErrCardFrontTooLong,
	)
	require.ErrorIs(t, err, domain.ErrCardFrontTooLong)

	_, err = domain.ParseCardgroupName(maxGraphemeText(domain.CardgroupNameMax + 1))
	require.ErrorIs(t, err, domain.ErrCardgroupNameTooLong)
}

// TestCardRepository_CheckViolation_ClassifiesAsTextLengthError proves the 23514
// backstop is wired: a write that bypasses the domain gate and exceeds the DB
// bound surfaces as *repository.TextLengthViolationError (which the usecase maps
// to BAD_USER_INPUT) rather than an opaque wrapped driver error.
func TestCardRepository_CheckViolation_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	// 20001 ASCII code points: past the widened cards_front_length bound (20x the
	// 500-grapheme cap = 10000), and unreachable through domain.ParseCardText.
	overBound := strings.Repeat("a", 20001)
	card := newCard(cg.ID, overBound, "back")

	err := repo.Create(ctx, card)
	require.Error(t, err)

	var v *repository.TextLengthViolationError
	require.ErrorAs(t, err, &v)
	require.Equal(t, "front", v.Field)
	require.Equal(t, "cards_front_length", v.Constraint)
}

// TestCardgroupRepository_CheckViolation_ClassifiesAsTextLengthError is the
// deck-name sibling of the card test above. cardgroups_name_length is 20x the
// 100-grapheme cap, so only a pathological combining-mark name reaches it
// through the domain -- but the write path must still classify the violation as
// a field-scoped user-input error rather than an opaque internal one. Both the
// insert statement (Create) and the GORM Updates statement (Update) are covered
// because they wrap distinct error branches.
func TestCardgroupRepository_CheckViolation_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	// 2001 ASCII code points: past the widened cardgroups_name_length bound (20x
	// the 100-grapheme cap = 2000), and unreachable through ParseCardgroupName.
	overBound := strings.Repeat("a", 2001)

	err := repo.Create(ctx, newCardgroup(ownerID, overBound))
	require.Error(t, err)
	var created *repository.TextLengthViolationError
	require.ErrorAs(t, err, &created)
	require.Equal(t, "name", created.Field)
	require.Equal(t, "cardgroups_name_length", created.Constraint)

	cg := newCardgroup(ownerID, "Within Bounds")
	require.NoError(t, repo.Create(ctx, cg))

	_, err = repo.Update(ctx, string(cg.ID), repository.CardgroupUpdate{Name: &overBound})
	require.Error(t, err)
	var updated *repository.TextLengthViolationError
	require.ErrorAs(t, err, &updated)
	require.Equal(t, "name", updated.Field)
	require.Equal(t, "cardgroups_name_length", updated.Constraint)
}

// TestCardgroupRepository_CreateTx_CheckViolation_ClassifiesAsTextLengthError
// covers the transactional insert used by the master-deck copy path, which
// writes the master deck's name verbatim into a new user cardgroup.
func TestCardgroupRepository_CreateTx_CheckViolation_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	overBound := strings.Repeat("a", 2001)
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.CreateTx(ctx, tx, newCardgroup(ownerID, overBound))
	})
	require.Error(t, err)

	var v *repository.TextLengthViolationError
	require.ErrorAs(t, err, &v)
	require.Equal(t, "name", v.Field)
	require.Equal(t, "cardgroups_name_length", v.Constraint)
}

// TestCardRepository_UpsertManyTx_CheckViolation_ClassifiesAsTextLengthError
// covers the bulk writer behind batch import and merge-from-catalog. It shares
// the cards_back_length constraint with the single-row Create path but reaches
// it through a hand-built multi-row INSERT, so it needs its own classifier arm.
func TestCardRepository_UpsertManyTx_CheckViolation_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	// 10001 ASCII code points: past the widened cards_back_length bound (20x the
	// 500-grapheme cap = 10000), and unreachable through domain.ParseCardText.
	overBound := strings.Repeat("a", 10001)
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, []*domain.Card{
			newCard(cg.ID, "upsert-cap-front", overBound),
		})
		return txErr
	})
	require.Error(t, err)

	var v *repository.TextLengthViolationError
	require.ErrorAs(t, err, &v)
	require.Equal(t, "back", v.Field)
	require.Equal(t, "cards_back_length", v.Constraint)
}

// incompressibleFront returns domain.CardTextMax grapheme clusters, each a random
// CJK base (U+4E00..U+9FFF) followed by marks random combining marks
// (U+0300..U+036F). A fixed seed keeps it deterministic; the randomness keeps
// pglz from shrinking it under the btree limit.
func incompressibleFront(marks int) string {
	r := rand.New(rand.NewPCG(1, 2))
	var b strings.Builder
	for range domain.CardTextMax {
		b.WriteRune(rune(0x4E00 + r.IntN(0x9FFF-0x4E00+1)))
		for range marks {
			b.WriteRune(rune(0x0300 + r.IntN(0x036F-0x0300+1)))
		}
	}
	return b.String()
}

// TestIncompressibleFront_IsDomainValid is the precondition for the 54000 tests
// below: both inputs pass the domain grapheme gate and stay within the
// cards_front_length CHECK (10000 code points), so only the index limit can
// reject them.
func TestIncompressibleFront_IsDomainValid(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		marks     int
		wantRunes int
	}{
		{marks: 4, wantRunes: 2500},
		{marks: 19, wantRunes: 10000},
	} {
		s := incompressibleFront(tc.marks)
		_, err := domain.ParseCardText(s, domain.ErrCardFrontRequired, domain.ErrCardFrontTooLong)
		require.NoError(t, err, "marks=%d", tc.marks)
		require.Equal(t, domain.CardTextMax, uniseg.GraphemeClusterCount(s), "marks=%d", tc.marks)
		require.Equal(t, tc.wantRunes, len([]rune(s)), "marks=%d", tc.marks)
	}
}

// requireFrontIndexViolation asserts err is the 54000 backstop classified as a
// front length error against index.
func requireFrontIndexViolation(t *testing.T, err error, index string) {
	t.Helper()
	var v *repository.TextLengthViolationError
	require.ErrorAs(t, err, &v)
	require.Equal(t, "front", v.Field)
	require.Equal(t, index, v.Constraint)
}

// TestCardRepository_FrontIndexRowTooLarge_ClassifiesAsTextLengthError proves a
// domain-valid front too large for uq_cards_cardgroup_front surfaces as
// *repository.TextLengthViolationError in both 54000 shapes: the btree check
// (index named) and the earlier index-tuple check (no index named).
func TestCardRepository_FrontIndexRowTooLarge_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	for _, tc := range []struct {
		name  string
		marks int
	}{
		{name: "btree", marks: 4},
		{name: "index tuple", marks: 19},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := repo.Create(ctx, newCard(cg.ID, incompressibleFront(tc.marks), "back"))
			requireFrontIndexViolation(t, err, "uq_cards_cardgroup_front")
		})
	}
}

// TestCardRepository_UpsertManyTx_FrontIndexRowTooLarge_ClassifiesAsTextLengthError
// covers the bulk writer, which reaches the same index through a hand-built
// multi-row INSERT and so needs its own classifier arm.
func TestCardRepository_UpsertManyTx_FrontIndexRowTooLarge_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, []*domain.Card{
			newCard(cg.ID, incompressibleFront(19), "back"),
		})
		return txErr
	})
	requireFrontIndexViolation(t, err, "uq_cards_cardgroup_front")
}

// TestMasterCardRepository_FrontIndexRowTooLarge_ClassifiesAsTextLengthError is
// the master-catalog sibling: master_cards.front is citext with its own unique
// (master_cardgroup_id, front) index.
func TestMasterCardRepository_FrontIndexRowTooLarge_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "Front-Index-Row-Size-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	err := repo.Create(ctx, newMasterCard(mcg.ID, incompressibleFront(4), "back", 0))
	requireFrontIndexViolation(t, err, "uq_master_cards_cg_front")
}

// TestCardRepository_Update_FrontIndexRowTooLarge_ClassifiesAsTextLengthError
// pins the classifier arm on the single-card edit path in both 54000 shapes.
func TestCardRepository_Update_FrontIndexRowTooLarge_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	card := newCard(cg.ID, "small", "back")
	require.NoError(t, repo.Create(ctx, card))

	for _, tc := range []struct {
		name  string
		marks int
	}{
		{name: "btree", marks: 4},
		{name: "index tuple", marks: 19},
	} {
		t.Run(tc.name, func(t *testing.T) {
			big := incompressibleFront(tc.marks)
			_, err := repo.Update(ctx, card.ID, repository.CardUpdate{Front: &big})
			requireFrontIndexViolation(t, err, "uq_cards_cardgroup_front")
		})
	}
}

// TestMasterCardRepository_Update_FrontIndexRowTooLarge_ClassifiesAsTextLengthError
// pins the master edit-path arm. marks=4 hits the btree shape, which names the
// index, so a wrong index constant at the call site fails the test.
func TestMasterCardRepository_Update_FrontIndexRowTooLarge_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "Front-Index-Row-Size-Update-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)
	card := newMasterCard(mcg.ID, "small", "back", 0)
	require.NoError(t, repo.Create(ctx, card))

	big := incompressibleFront(4)
	_, err := repo.Update(ctx, card.ID, repository.MasterCardUpdate{Front: &big})
	requireFrontIndexViolation(t, err, "uq_master_cards_cg_front")
}

// TestMasterCardRepository_UpsertManyTx_FrontIndexRowTooLarge_ClassifiesAsTextLengthError
// pins the master bulk-import arm. marks=4 hits the btree shape, which names the
// index, so a wrong index constant at the call site fails the test.
func TestMasterCardRepository_UpsertManyTx_FrontIndexRowTooLarge_ClassifiesAsTextLengthError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mcg := insertMCGForCardTest(t, ctx, "Front-Index-Row-Size-Upsert-Group")
	repo := repository.NewMasterCardRepository(testDB.GORM)

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, []*domain.MasterCard{
			newMasterCard(mcg.ID, incompressibleFront(4), "back", 0),
		})
		return txErr
	})
	requireFrontIndexViolation(t, err, "uq_master_cards_cg_front")
}
