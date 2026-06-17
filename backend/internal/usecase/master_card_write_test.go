package usecase

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/textdic"
)

// ---------------------------------------------------------------------------
// Write-capable test double
// ---------------------------------------------------------------------------

// mockMasterCardWriteRepo is a manual test double for the write surface of
// repository.MasterCardRepository. It embeds panicMasterCardRepo so any method
// the test under exercise does not call panics loudly.
type mockMasterCardWriteRepo struct {
	panicMasterCardRepo

	// Create
	createErr   error
	createCalls []*domain.MasterCard

	// FindByMasterCardgroupAndFront (duplicate-lookup)
	findByFrontResult *domain.MasterCard
	findByFrontErr    error
	findByFrontGroup  string
	findByFrontValue  string

	// Update
	updateResult *domain.MasterCard
	updateErr    error
	updateID     string
	updatePatch  repository.MasterCardUpdate

	// Delete (single)
	deleteErr    error
	deleteID     string
	deleteCalled bool

	// DeleteMany (bulk)
	deleteManyResult int64
	deleteManyErr    error
	deleteManyIDs    []string
	deleteManyCalled bool

	// UpsertManyTx (import)
	upsertResult   repository.UpsertManyTxResult
	upsertErr      error
	upsertCaptured []*domain.MasterCard
	upsertCalls    int
}

func (m *mockMasterCardWriteRepo) Create(_ context.Context, c *domain.MasterCard) error {
	m.createCalls = append(m.createCalls, c)
	return m.createErr
}

func (m *mockMasterCardWriteRepo) FindByMasterCardgroupAndFront(_ context.Context, masterCardgroupID, front string) (*domain.MasterCard, error) {
	m.findByFrontGroup = masterCardgroupID
	m.findByFrontValue = front
	if m.findByFrontErr != nil {
		return nil, m.findByFrontErr
	}
	return m.findByFrontResult, nil
}

func (m *mockMasterCardWriteRepo) Update(_ context.Context, id string, patch repository.MasterCardUpdate) (*domain.MasterCard, error) {
	m.updateID = id
	m.updatePatch = patch
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	return m.updateResult, nil
}

func (m *mockMasterCardWriteRepo) Delete(_ context.Context, id string) error {
	m.deleteCalled = true
	m.deleteID = id
	return m.deleteErr
}

func (m *mockMasterCardWriteRepo) DeleteMany(_ context.Context, ids []string) (int64, error) {
	m.deleteManyCalled = true
	m.deleteManyIDs = ids
	if m.deleteManyErr != nil {
		return 0, m.deleteManyErr
	}
	return m.deleteManyResult, nil
}

func (m *mockMasterCardWriteRepo) UpsertManyTx(_ context.Context, _ *gorm.DB, cards []*domain.MasterCard) (repository.UpsertManyTxResult, error) {
	m.upsertCalls++
	for _, c := range cards {
		clone := *c
		m.upsertCaptured = append(m.upsertCaptured, &clone)
	}
	if m.upsertErr != nil {
		return repository.UpsertManyTxResult{}, m.upsertErr
	}
	return m.upsertResult, nil
}

// newMasterCardWriteUC builds a MasterCardUsecase over the write mock with no tx
// runner (the non-import write methods never open a transaction).
func newMasterCardWriteUC(t *testing.T, mc repository.MasterCardRepository, isAdmin bool) MasterCardUsecase {
	t.Helper()
	return NewMasterCardUsecase(nil, mc, &mockMasterCardgroupReadRepo{}, newTestAdminGate(isAdmin), newTestLogger())
}

// newMasterCardImportUC builds a MasterCardUsecase with an explicit tx runner and
// an optional injected parser so import logic can be exercised deterministically
// without a real database or the textdic grammar.
func newMasterCardImportUC(
	t *testing.T,
	mc repository.MasterCardRepository,
	tx txRunner,
	isAdmin bool,
	process func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error),
) MasterCardUsecase {
	t.Helper()
	uc := NewMasterCardUsecaseWithTx(mc, &mockMasterCardgroupReadRepo{}, tx, newTestAdminGate(isAdmin), newTestLogger())
	if process != nil {
		uc.(*masterCardUsecase).processCardImport = process
	}
	return uc
}

// b64 encodes s as a standard base64 payload for the import path.
func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// ---------------------------------------------------------------------------
// CreateMasterCard
// ---------------------------------------------------------------------------

func TestMasterCard_CreateMasterCard_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := newMasterCardWriteUC(t, &mockMasterCardWriteRepo{}, false)
	_, err := uc.CreateMasterCard(authedCtx("u1"), CreateMasterCardInput{MasterCardgroupID: "m1", Front: "f", Back: "b"})
	assertForbidden(t, err, "admin only")
}

func TestMasterCard_CreateMasterCard_Anonymous(t *testing.T) {
	t.Parallel()
	uc := newMasterCardWriteUC(t, &mockMasterCardWriteRepo{}, true)
	_, err := uc.CreateMasterCard(anonCtx(), CreateMasterCardInput{MasterCardgroupID: "m1", Front: "f", Back: "b"})
	assertUnauthenticated(t, err)
}

func TestMasterCard_CreateMasterCard_Success(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{}
	uc := newMasterCardWriteUC(t, mc, true)
	out, err := uc.CreateMasterCard(authedCtx("admin1"), CreateMasterCardInput{MasterCardgroupID: "m1", Front: "  hello  ", Back: "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Duplicate != nil {
		t.Fatalf("expected no duplicate, got %+v", out.Duplicate)
	}
	if out.Card == nil {
		t.Fatal("expected created card")
	}
	// NewMasterCard trims the front through ParseCardText.
	if out.Card.Front != domain.CardText("hello") {
		t.Fatalf("front must be trimmed to %q, got %q", "hello", out.Card.Front)
	}
	if out.Card.MasterCardgroupID != "m1" {
		t.Fatalf("masterCardgroupID: want m1, got %q", out.Card.MasterCardgroupID)
	}
	if len(mc.createCalls) != 1 {
		t.Fatalf("expected 1 Create call, got %d", len(mc.createCalls))
	}
}

func TestMasterCard_CreateMasterCard_DuplicateAsData(t *testing.T) {
	t.Parallel()
	existing := &domain.MasterCard{ID: "existing-id", MasterCardgroupID: "m1", Front: domain.CardText("Cat"), Back: domain.CardText("existing-back")}
	mc := &mockMasterCardWriteRepo{
		createErr:         repository.ErrCardDuplicateFront,
		findByFrontResult: existing,
	}
	uc := newMasterCardWriteUC(t, mc, true)

	// Submit a case-only-differing front with surrounding whitespace.
	out, err := uc.CreateMasterCard(authedCtx("admin1"), CreateMasterCardInput{MasterCardgroupID: "m1", Front: "  cat  ", Back: "new-back"})
	if err != nil {
		t.Fatalf("expected nil error (duplicate is data), got %v", err)
	}
	if out.Card != nil {
		t.Fatalf("expected nil Card on duplicate, got %+v", out.Card)
	}
	if out.Duplicate == nil {
		t.Fatal("expected Duplicate outcome on duplicate-front collision")
	}
	if out.Duplicate.ExistingID != "existing-id" {
		t.Errorf("ExistingID: want existing-id, got %q", out.Duplicate.ExistingID)
	}
	if out.Duplicate.ExistingBack != "existing-back" {
		t.Errorf("ExistingBack: want existing-back, got %q", out.Duplicate.ExistingBack)
	}
	// The lookup must use the trimmed front and the supplied group; citext makes
	// the DB match case-insensitive.
	if mc.findByFrontValue != "cat" {
		t.Errorf("duplicate lookup front: want trimmed %q, got %q", "cat", mc.findByFrontValue)
	}
	if mc.findByFrontGroup != "m1" {
		t.Errorf("duplicate lookup group: want m1, got %q", mc.findByFrontGroup)
	}
}

func TestMasterCard_CreateMasterCard_DuplicateLookupRace(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{
		createErr:      repository.ErrCardDuplicateFront,
		findByFrontErr: eris.New("db: connection reset"),
	}
	uc := newMasterCardWriteUC(t, mc, true)
	_, err := uc.CreateMasterCard(authedCtx("admin1"), CreateMasterCardInput{MasterCardgroupID: "m1", Front: "hello", Back: "world"})
	assertInternalChain(t, err, "usecase: master card: lookup duplicate after 23505")
}

// A context cancellation during the duplicate re-lookup must pass through
// unwrapped so the caller's errors.Is identity check succeeds — mirroring the
// isContextDone pass-through convention used by every other repo-error path in
// master_card.go.
func TestMasterCard_CreateMasterCard_DuplicateLookup_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{
		createErr:      repository.ErrCardDuplicateFront,
		findByFrontErr: context.Canceled,
	}
	uc := newMasterCardWriteUC(t, mc, true)
	_, err := uc.CreateMasterCard(authedCtx("admin1"), CreateMasterCardInput{MasterCardgroupID: "m1", Front: "hello", Back: "world"})
	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

func TestMasterCard_CreateMasterCard_FrontEmptyValidation(t *testing.T) {
	t.Parallel()
	uc := newMasterCardWriteUC(t, &mockMasterCardWriteRepo{}, true)
	_, err := uc.CreateMasterCard(authedCtx("admin1"), CreateMasterCardInput{MasterCardgroupID: "m1", Front: "   ", Back: "b"})
	// Create surfaces field validation through the error channel (the union has
	// no InputValidationError variant).
	assertValidationError(t, err, "front", "")
}

func TestMasterCard_CreateMasterCard_BackTooLongValidation(t *testing.T) {
	t.Parallel()
	uc := newMasterCardWriteUC(t, &mockMasterCardWriteRepo{}, true)
	long := strings.Repeat("x", domain.CardTextMax+1)
	_, err := uc.CreateMasterCard(authedCtx("admin1"), CreateMasterCardInput{MasterCardgroupID: "m1", Front: "f", Back: long})
	assertValidationError(t, err, "back", "")
}

func TestMasterCard_CreateMasterCard_RepoErrorWrapped(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{createErr: eris.New("db boom")}
	uc := newMasterCardWriteUC(t, mc, true)
	_, err := uc.CreateMasterCard(authedCtx("admin1"), CreateMasterCardInput{MasterCardgroupID: "m1", Front: "f", Back: "b"})
	assertInternalChain(t, err, "usecase: master card: create")
}

// ---------------------------------------------------------------------------
// UpdateMasterCard
// ---------------------------------------------------------------------------

func TestMasterCard_UpdateMasterCard_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := newMasterCardWriteUC(t, &mockMasterCardWriteRepo{}, false)
	front := "f"
	_, err := uc.UpdateMasterCard(authedCtx("u1"), "id-1", UpdateMasterCardInput{Front: &front})
	assertForbidden(t, err, "admin only")
}

func TestMasterCard_UpdateMasterCard_Success(t *testing.T) {
	t.Parallel()
	updated := &domain.MasterCard{ID: "id-1", MasterCardgroupID: "m1", Front: domain.CardText("new-front"), Back: domain.CardText("new-back")}
	mc := &mockMasterCardWriteRepo{updateResult: updated}
	uc := newMasterCardWriteUC(t, mc, true)

	front := "  new-front  "
	back := "new-back"
	out, err := uc.UpdateMasterCard(authedCtx("admin1"), "id-1", UpdateMasterCardInput{Front: &front, Back: &back})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Validation != nil {
		t.Fatalf("expected no validation, got %+v", out.Validation)
	}
	if out.Card == nil || out.Card.ID != "id-1" {
		t.Fatalf("expected updated card id-1, got %+v", out.Card)
	}
	// The patch must carry the trimmed front.
	if mc.updatePatch.Front == nil || *mc.updatePatch.Front != "new-front" {
		t.Fatalf("patch front must be trimmed to %q, got %v", "new-front", mc.updatePatch.Front)
	}
	if mc.updatePatch.Back == nil || *mc.updatePatch.Back != "new-back" {
		t.Fatalf("patch back: want %q, got %v", "new-back", mc.updatePatch.Back)
	}
}

func TestMasterCard_UpdateMasterCard_EmptyFrontValidationVariant(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{}
	uc := newMasterCardWriteUC(t, mc, true)
	empty := "   "
	out, err := uc.UpdateMasterCard(authedCtx("admin1"), "id-1", UpdateMasterCardInput{Front: &empty})
	if err != nil {
		t.Fatalf("expected nil error (validation is data), got %v", err)
	}
	if out.Card != nil {
		t.Fatalf("expected nil Card on validation failure, got %+v", out.Card)
	}
	if out.Validation == nil || out.Validation.Field != "front" {
		t.Fatalf("expected Validation on field 'front', got %+v", out.Validation)
	}
	// The repo must not be touched when the patch fails validation.
	if mc.updateID != "" {
		t.Fatalf("repo.Update must not be called on validation failure, got id %q", mc.updateID)
	}
}

func TestMasterCard_UpdateMasterCard_BackTooLongValidationVariant(t *testing.T) {
	t.Parallel()
	uc := newMasterCardWriteUC(t, &mockMasterCardWriteRepo{}, true)
	long := strings.Repeat("y", domain.CardTextMax+1)
	out, err := uc.UpdateMasterCard(authedCtx("admin1"), "id-1", UpdateMasterCardInput{Back: &long})
	if err != nil {
		t.Fatalf("expected nil error (validation is data), got %v", err)
	}
	if out.Validation == nil || out.Validation.Field != "back" {
		t.Fatalf("expected Validation on field 'back', got %+v", out.Validation)
	}
}

func TestMasterCard_UpdateMasterCard_NotFound(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{updateErr: repository.ErrNotFound}
	uc := newMasterCardWriteUC(t, mc, true)
	front := "f"
	_, err := uc.UpdateMasterCard(authedCtx("admin1"), "missing", UpdateMasterCardInput{Front: &front})
	assertValidationError(t, err, "id", "")
}

func TestMasterCard_UpdateMasterCard_RepoErrorWrapped(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{updateErr: eris.New("db boom")}
	uc := newMasterCardWriteUC(t, mc, true)
	front := "f"
	_, err := uc.UpdateMasterCard(authedCtx("admin1"), "id-1", UpdateMasterCardInput{Front: &front})
	assertInternalChain(t, err, "usecase: master card: update")
}

// ---------------------------------------------------------------------------
// DeleteMasterCard
// ---------------------------------------------------------------------------

func TestMasterCard_DeleteMasterCard_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := newMasterCardWriteUC(t, &mockMasterCardWriteRepo{}, false)
	err := uc.DeleteMasterCard(authedCtx("u1"), "id-1")
	assertForbidden(t, err, "admin only")
}

func TestMasterCard_DeleteMasterCard_Success(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{}
	uc := newMasterCardWriteUC(t, mc, true)
	if err := uc.DeleteMasterCard(authedCtx("admin1"), "id-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mc.deleteCalled || mc.deleteID != "id-1" {
		t.Fatalf("expected Delete called with id-1, got called=%v id=%q", mc.deleteCalled, mc.deleteID)
	}
}

func TestMasterCard_DeleteMasterCard_NotFound(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{deleteErr: repository.ErrNotFound}
	uc := newMasterCardWriteUC(t, mc, true)
	err := uc.DeleteMasterCard(authedCtx("admin1"), "missing")
	assertValidationError(t, err, "id", "")
}

func TestMasterCard_DeleteMasterCard_RepoErrorWrapped(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{deleteErr: eris.New("db boom")}
	uc := newMasterCardWriteUC(t, mc, true)
	err := uc.DeleteMasterCard(authedCtx("admin1"), "id-1")
	assertInternalChain(t, err, "usecase: master card: delete")
}

// ---------------------------------------------------------------------------
// DeleteMasterCards (bulk)
// ---------------------------------------------------------------------------

func TestMasterCard_DeleteMasterCards_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := newMasterCardWriteUC(t, &mockMasterCardWriteRepo{}, false)
	_, err := uc.DeleteMasterCards(authedCtx("u1"), []string{"a", "b"})
	assertForbidden(t, err, "admin only")
}

func TestMasterCard_DeleteMasterCards_Success(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{deleteManyResult: 2}
	uc := newMasterCardWriteUC(t, mc, true)
	n, err := uc.DeleteMasterCards(authedCtx("admin1"), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected deleted count 2, got %d", n)
	}
	if !mc.deleteManyCalled {
		t.Fatal("expected DeleteMany to be called")
	}
}

func TestMasterCard_DeleteMasterCards_EmptyNoOp(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{}
	uc := newMasterCardWriteUC(t, mc, true)
	n, err := uc.DeleteMasterCards(authedCtx("admin1"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
	if mc.deleteManyCalled {
		t.Fatal("DeleteMany must not be called for an empty id list")
	}
}

func TestMasterCard_DeleteMasterCards_OverCap(t *testing.T) {
	t.Parallel()
	uc := newMasterCardWriteUC(t, &mockMasterCardWriteRepo{}, true)
	ids := make([]string, maxBulkDelete+1)
	for i := range ids {
		ids[i] = "id"
	}
	_, err := uc.DeleteMasterCards(authedCtx("admin1"), ids)
	assertValidationError(t, err, "ids", "")
}

func TestMasterCard_DeleteMasterCards_RepoErrorWrapped(t *testing.T) {
	t.Parallel()
	mc := &mockMasterCardWriteRepo{deleteManyErr: eris.New("db boom")}
	uc := newMasterCardWriteUC(t, mc, true)
	_, err := uc.DeleteMasterCards(authedCtx("admin1"), []string{"a"})
	assertInternalChain(t, err, "usecase: master card: bulk delete")
}

// ---------------------------------------------------------------------------
// ImportMasterCards
// ---------------------------------------------------------------------------

func TestMasterCard_ImportMasterCards_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, &mockMasterCardWriteRepo{}, tx, false, nil)
	_, err := uc.ImportMasterCards(authedCtx("u1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64("x")})
	assertForbidden(t, err, "admin only")
}

func TestMasterCard_ImportMasterCards_Anonymous(t *testing.T) {
	t.Parallel()
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, &mockMasterCardWriteRepo{}, tx, true, nil)
	_, err := uc.ImportMasterCards(anonCtx(), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64("x")})
	assertUnauthenticated(t, err)
}

func TestMasterCard_ImportMasterCards_EmptyGroupID(t *testing.T) {
	t.Parallel()
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, &mockMasterCardWriteRepo{}, tx, true, nil)
	_, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "", Payload: b64("x")})
	assertValidationError(t, err, "masterCardgroupId", "")
}

func TestMasterCard_ImportMasterCards_EmptyPayload(t *testing.T) {
	t.Parallel()
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, &mockMasterCardWriteRepo{}, tx, true, nil)
	_, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: ""})
	assertValidationError(t, err, "payload", "")
}

func TestMasterCard_ImportMasterCards_BadBase64(t *testing.T) {
	t.Parallel()
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, &mockMasterCardWriteRepo{}, tx, true, nil)
	_, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: "!!!not-base64!!!"})
	assertValidationError(t, err, "payload", "")
}

func TestMasterCard_ImportMasterCards_InsertsAndUpdates(t *testing.T) {
	t.Parallel()
	process := func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		return []textdic.ParsedWord{
			{Front: "Apple", Back: "back-a", Line: 1},
			{Front: "Banana", Back: "back-b", Line: 2},
		}, nil, nil
	}
	mc := &mockMasterCardWriteRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1, Updated: 1}}
	tx, calls := dictTxRunner()
	uc := newMasterCardImportUC(t, mc, tx, true, process)

	out, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64("ignored")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != 1 || out.Updated != 1 {
		t.Fatalf("expected Inserted=1 Updated=1, got %d/%d", out.Inserted, out.Updated)
	}
	if len(out.Errors) != 0 {
		t.Fatalf("expected no per-line errors, got %d", len(out.Errors))
	}
	if *calls != 1 {
		t.Fatalf("expected the tx runner to run once, got %d", *calls)
	}
	if mc.upsertCalls != 1 || len(mc.upsertCaptured) != 2 {
		t.Fatalf("expected UpsertManyTx called once with 2 cards, got calls=%d cards=%d", mc.upsertCalls, len(mc.upsertCaptured))
	}
	// Cards must be built for the target deck.
	for _, c := range mc.upsertCaptured {
		if c.MasterCardgroupID != "m1" {
			t.Fatalf("imported card must target deck m1, got %q", c.MasterCardgroupID)
		}
	}
}

func TestMasterCard_ImportMasterCards_PerLineErrors(t *testing.T) {
	t.Parallel()
	process := func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		return []textdic.ParsedWord{{Front: "Apple", Back: "back-a", Line: 1}},
			[]textdic.ValidationError{{Line: 2, Message: "lone front", Kind: textdic.SkipKindFrontOnly, Snippet: "Lone"}},
			nil
	}
	mc := &mockMasterCardWriteRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1}}
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, mc, tx, true, process)

	out, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64("ignored")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != 1 {
		t.Fatalf("expected Inserted=1, got %d", out.Inserted)
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected 1 per-line error, got %d", len(out.Errors))
	}
	if out.Errors[0].Kind != CardImportErrKindFrontOnly || out.Errors[0].Line != 2 {
		t.Fatalf("unexpected mapped error: %+v", out.Errors[0])
	}
}

func TestMasterCard_ImportMasterCards_DeduplicatesByFront(t *testing.T) {
	t.Parallel()
	process := func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		return []textdic.ParsedWord{
			{Front: "Dup", Back: "first", Line: 1},
			{Front: "Dup", Back: "second", Line: 2},
		}, nil, nil
	}
	mc := &mockMasterCardWriteRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1}}
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, mc, tx, true, process)

	out, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64("ignored")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Last occurrence wins; one card reaches the repo, the dropped row is reported.
	if len(mc.upsertCaptured) != 1 {
		t.Fatalf("expected 1 deduped card, got %d", len(mc.upsertCaptured))
	}
	if mc.upsertCaptured[0].Back != domain.CardText("second") {
		t.Fatalf("surviving card must carry the last back %q, got %q", "second", mc.upsertCaptured[0].Back)
	}
	if len(out.Errors) != 1 || out.Errors[0].Kind != CardImportErrKindDuplicate {
		t.Fatalf("expected 1 DUPLICATE error, got %+v", out.Errors)
	}
}

func TestMasterCard_ImportMasterCards_OverCap(t *testing.T) {
	t.Parallel()
	process := func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		words := make([]textdic.ParsedWord, cardImportParsedRowCap+1)
		for i := range words {
			words[i] = textdic.ParsedWord{Front: "f", Back: "b", Line: i + 1}
		}
		return words, nil, nil
	}
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, &mockMasterCardWriteRepo{}, tx, true, process)
	_, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64("ignored")})
	assertValidationError(t, err, "payload", "")
}

func TestMasterCard_ImportMasterCards_EmptyParseNoPersist(t *testing.T) {
	t.Parallel()
	process := func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		return nil, []textdic.ValidationError{{Line: 1, Message: "empty payload", Kind: textdic.SkipKindHard}}, nil
	}
	mc := &mockMasterCardWriteRepo{}
	tx, calls := dictTxRunner()
	uc := newMasterCardImportUC(t, mc, tx, true, process)

	out, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64("ignored")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != 0 || out.Updated != 0 {
		t.Fatalf("expected nothing persisted, got %d/%d", out.Inserted, out.Updated)
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected the parser diagnostic surfaced, got %d", len(out.Errors))
	}
	if *calls != 0 || mc.upsertCalls != 0 {
		t.Fatalf("empty parse must not open a tx or upsert, got calls=%d upserts=%d", *calls, mc.upsertCalls)
	}
}

func TestMasterCard_ImportMasterCards_RepoErrorWrapped(t *testing.T) {
	t.Parallel()
	process := func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		return []textdic.ParsedWord{{Front: "Apple", Back: "back-a", Line: 1}}, nil, nil
	}
	mc := &mockMasterCardWriteRepo{upsertErr: eris.New("db boom")}
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, mc, tx, true, process)
	_, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64("ignored")})
	assertInternalChain(t, err, "usecase: master card: import")
}
