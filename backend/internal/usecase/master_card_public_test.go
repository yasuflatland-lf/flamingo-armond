package usecase

import (
	"context"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/require"

	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
)

// ---------------------------------------------------------------------------
// ListPublicMasterCards
//
// The page-assembly body reuses the SAME shared helpers as ListMasterCards
// (resolveRelayPage / resolveMasterCardOrderBy / resolveMasterCardCursor /
// assemblePage), exhaustively tested for the admin path in master_card_test.go.
// These tests pin what is specific to the public method and NOT covered by the
// admin tests: the authentication check, the published-only visibility gate, the
// public-specific error-wrap prefixes ("usecase: master card: public list: ..."),
// and the in-method context-cancel pass-throughs.
// ---------------------------------------------------------------------------

// publishedMasterDeck returns a PUBLISHED *domain.MasterCardgroup fixture. The
// usecase trusts FindPublishedByID's status filter and never inspects Status, so
// the field is set only to make the fixture self-describing.
func publishedMasterDeck(id string) *domain.MasterCardgroup {
	return &domain.MasterCardgroup{
		ID:      id,
		Name:    domain.CardgroupName("Deck " + id),
		Status:  domain.MasterStatusPublished,
		Version: 1,
	}
}

func TestListPublicMasterCards_Unauthenticated(t *testing.T) {
	t.Parallel()
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, &mockMasterCardgroupReadRepo{}, true)
	_, err := uc.ListPublicMasterCards(anonCtx(), MasterCardConnectionInput{MasterCardgroupID: "id-1"})
	assertUnauthenticated(t, err)
}

// A DRAFT or unknown deck surfaces from FindPublishedByID as ErrNotFound, which
// the published gate maps to a BAD_USER_INPUT validation error on
// "masterCardgroupId" (non-disclosure: draft and unknown are indistinguishable).
// The page query must NOT run when the gate rejects the deck.
func TestListPublicMasterCards_DraftOrUnknownDeck_BadUserInput(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{}
	mcg := &mockMasterCardgroupReadRepo{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, repository.ErrNotFound },
	}
	uc := newMasterCardUC(t, mc, mcg, true)
	_, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{MasterCardgroupID: "missing"})
	assertValidationError(t, err, "masterCardgroupId", "master deck not found")
	if len(mc.findPageCalls) != 0 {
		t.Fatalf("page query must not run when the published gate rejects the deck, got %d calls", len(mc.findPageCalls))
	}
}

// A non-NotFound, non-context failure from the published gate is an internal
// error wrapped into the eris chain, not a validation/forbidden error.
func TestListPublicMasterCards_FindPublishedByID_InfraError_Wrapped(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{}
	mcg := &mockMasterCardgroupReadRepo{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, eris.New("db down") },
	}
	uc := newMasterCardUC(t, mc, mcg, true)
	_, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{MasterCardgroupID: "id-1"})
	assertInternalChain(t, err, "usecase: master card: public list: find published by id")
	if len(mc.findPageCalls) != 0 {
		t.Fatalf("page query must not run when the published gate errors, got %d calls", len(mc.findPageCalls))
	}
}

// context.Canceled from the published gate must propagate unwrapped so its
// identity survives errors.Is at the resolver boundary (FromUsecaseError →
// CANCELLED). An eris.Wrap here would break the == identity contract.
func TestListPublicMasterCards_FindPublishedByID_ContextCancelled_PassesThrough(t *testing.T) {
	t.Parallel()
	mcg := &mockMasterCardgroupReadRepo{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return nil, context.Canceled },
	}
	uc := newMasterCardUC(t, &mockMasterCardReadRepo{}, mcg, true)
	_, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{MasterCardgroupID: "id-1"})
	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// A published deck assembles a page: edges, search-aware totalCount, and RAW
// (un-encoded) StartCur/EndCur. The raw-cursor assertion mirrors the admin path
// (the resolver's connection layer encodes once; storing encoded values here
// would double-encode).
func TestListPublicMasterCards_Success(t *testing.T) {
	t.Parallel()
	rows := []*domain.MasterCard{
		masterCardFixture("a", "id-1", 1),
		masterCardFixture("b", "id-1", 2),
	}
	mc := &mockMasterCardReadRepo{findPageRows: rows, findPageTotal: 2}
	mcg := &mockMasterCardgroupReadRepo{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return publishedMasterDeck("id-1"), nil },
	}
	uc := newMasterCardUC(t, mc, mcg, true)
	out, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(10),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 2 {
		t.Fatalf("expected TotalCount 2, got %d", out.TotalCount)
	}
	if len(out.Cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(out.Cards))
	}
	// StartCur / EndCur MUST be the RAW node ids; the resolver encodes once.
	if out.StartCur != "a" {
		t.Fatalf("StartCur: want raw id %q, got %q", "a", out.StartCur)
	}
	if out.EndCur != "b" {
		t.Fatalf("EndCur: want raw id %q, got %q", "b", out.EndCur)
	}
	if out.StartCur == cursor.Encode("a") || out.EndCur == cursor.Encode("b") {
		t.Fatalf("cursors must not be cursor.Encode(...)-encoded; got StartCur=%q EndCur=%q", out.StartCur, out.EndCur)
	}
}

// A whitespace-only search normalizes to nil before reaching the repository
// (same invariant ListMasterCards relies on). The published gate must pass so
// the page query runs.
func TestListPublicMasterCards_SearchNormalizesWhitespace(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{}
	mcg := &mockMasterCardgroupReadRepo{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return publishedMasterDeck("id-1"), nil },
	}
	uc := newMasterCardUC(t, mc, mcg, true)
	blank := "   "
	_, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		Search:            &blank,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc.findPageCalls[0].Search != nil {
		t.Fatalf("whitespace-only search must normalize to nil, got %q", *mc.findPageCalls[0].Search)
	}
}

// A repository error from the page query (AFTER the published gate passes) wraps
// into the eris chain with the PUBLIC-specific prefix — distinct from the admin
// path's "list: find page", so the admin RepoErrorWrapped test does not cover it.
func TestListPublicMasterCards_FindPage_RepoError_Wrapped(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{findPageErr: eris.New("db boom")}
	mcg := &mockMasterCardgroupReadRepo{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return publishedMasterDeck("id-1"), nil },
	}
	uc := newMasterCardUC(t, mc, mcg, true)
	_, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
	})
	assertInternalChain(t, err, "usecase: master card: public list: find page")
}

// context.Canceled from the page query (the in-closure pass-through, distinct
// from the gate's pass-through) must propagate unwrapped so its identity survives
// errors.Is at the resolver boundary.
func TestListPublicMasterCards_FindPage_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{findPageErr: context.Canceled}
	mcg := &mockMasterCardgroupReadRepo{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return publishedMasterDeck("id-1"), nil },
	}
	uc := newMasterCardUC(t, mc, mcg, true)
	_, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
	})
	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// context.Canceled surfaced during cursor hydration (resolveMasterCardCursor →
// FindByID, reached when orderBy is non-ID) must propagate unwrapped end-to-end
// through the public method. resolveMasterCardCursor is shared with the admin
// path, but this pins that ListPublicMasterCards itself adds no wrap on that path
// for the security-sensitive public endpoint.
func TestListPublicMasterCards_CursorHydration_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardReadRepo{
		findByIDFn: func(string) (*domain.MasterCard, error) { return nil, context.Canceled },
	}
	mcg := &mockMasterCardgroupReadRepo{
		findPublishedByIDFn: func(string) (*domain.MasterCardgroup, error) { return publishedMasterDeck("id-1"), nil },
	}
	uc := newMasterCardUC(t, mc, mcg, true)
	ob := MasterCardOrderByPosition // non-ID ordering triggers FindByID in resolveMasterCardCursor
	after := cursor.Encode("cur-1")
	_, err := uc.ListPublicMasterCards(authedCtx("u1"), MasterCardConnectionInput{
		MasterCardgroupID: "id-1",
		First:             intPtr(5),
		After:             &after,
		OrderBy:           &ob,
	})
	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}
