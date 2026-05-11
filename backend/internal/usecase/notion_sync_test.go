package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/notion"
	"backend/internal/repository"
)

type stubNotionFetcher struct {
	pages []notion.Page
	err   error
	calls int
	ids   []string
}

func (s *stubNotionFetcher) FetchPages(_ context.Context, ids []string) ([]notion.Page, error) {
	s.calls++
	s.ids = append([]string(nil), ids...)
	if s.err != nil {
		return nil, s.err
	}
	return s.pages, nil
}

type mockNotionCardgroupRepo struct {
	cg      *domain.Cardgroup
	err     error
	calls   int
	ownerID string
	name    string
}

func (m *mockNotionCardgroupRepo) EnsureByName(_ context.Context, ownerID, name string) (*domain.Cardgroup, error) {
	m.calls++
	m.ownerID = ownerID
	m.name = name
	if m.err != nil {
		return nil, m.err
	}
	if m.cg == nil {
		m.cg = &domain.Cardgroup{ID: "cg-created", OwnerID: ownerID, Name: name}
	}
	return m.cg, nil
}

type mockNotionCardRepo struct {
	existingFronts []string
	upsertResult   repository.UpsertManyTxResult
	upsertErr      error
	listErr        error
	deleteErr      error

	upserted       []*domain.Card
	deletedFronts  []string
	upsertCalls    int
	listCalls      int
	deleteCalls    int
	deleteAffected int64
}

func (m *mockNotionCardRepo) UpsertManyTx(_ context.Context, _ *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error) {
	m.upsertCalls++
	for _, card := range cards {
		clone := *card
		m.upserted = append(m.upserted, &clone)
	}
	if m.upsertErr != nil {
		return repository.UpsertManyTxResult{}, m.upsertErr
	}
	return m.upsertResult, nil
}

func (m *mockNotionCardRepo) ListFrontsByCardgroupTx(_ context.Context, _ *gorm.DB, _ string) ([]string, error) {
	m.listCalls++
	if m.listErr != nil {
		return nil, m.listErr
	}
	return append([]string(nil), m.existingFronts...), nil
}

func (m *mockNotionCardRepo) DeleteByCardgroupAndFrontsTx(_ context.Context, _ *gorm.DB, _ string, fronts []string) (int64, error) {
	m.deleteCalls++
	m.deletedFronts = append([]string(nil), fronts...)
	if m.deleteErr != nil {
		return 0, m.deleteErr
	}
	if m.deleteAffected != 0 {
		return m.deleteAffected, nil
	}
	return int64(len(fronts)), nil
}

func TestNotionSyncUsecase_DiffMerge(t *testing.T) {
	t.Parallel()

	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{
			ID:   "page-1",
			Text: "apple " + uniqueBack(1) + "\nbanana " + uniqueBack(2) + "\n",
		},
	}}
	cardgroups := &mockNotionCardgroupRepo{cg: &domain.Cardgroup{ID: "cg-target"}}
	cards := &mockNotionCardRepo{
		existingFronts: []string{"apple", "banana", "stale"},
		upsertResult:   repository.UpsertManyTxResult{Inserted: 1, Updated: 1},
	}
	tx, txCalls := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	out, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{" page-1 ", "page-1"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if out.CardgroupID != "cg-target" {
		t.Fatalf("CardgroupID = %q, want cg-target", out.CardgroupID)
	}
	if out.Inserted != 1 || out.Updated != 1 || out.Deleted != 1 {
		t.Fatalf("counts = (%d,%d,%d), want (1,1,1)", out.Inserted, out.Updated, out.Deleted)
	}
	if len(out.Parsed) != 2 {
		t.Fatalf("Parsed len = %d, want 2", len(out.Parsed))
	}
	if cardgroups.ownerID != "owner-1" || cardgroups.name != "English" {
		t.Fatalf("EnsureByName args = (%q,%q)", cardgroups.ownerID, cardgroups.name)
	}
	if len(fetcher.ids) != 1 || fetcher.ids[0] != "page-1" {
		t.Fatalf("fetch ids = %v, want [page-1]", fetcher.ids)
	}
	if len(cards.upserted) != 2 {
		t.Fatalf("upserted len = %d, want 2", len(cards.upserted))
	}
	if cards.upserted[0].CardgroupID != "cg-target" {
		t.Fatalf("upserted cardgroup = %q, want cg-target", cards.upserted[0].CardgroupID)
	}
	if len(cards.deletedFronts) != 1 || cards.deletedFronts[0] != "stale" {
		t.Fatalf("deleted fronts = %v, want [stale]", cards.deletedFronts)
	}
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
}

func TestNotionSyncUsecase_DuplicateFrontLastWins(t *testing.T) {
	t.Parallel()

	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"},
		{ID: "page-2", Text: "apple " + uniqueBack(2) + "\n"},
	}}
	cardgroups := &mockNotionCardgroupRepo{cg: &domain.Cardgroup{ID: "cg-target"}}
	cards := &mockNotionCardRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1}}
	tx, txCalls := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	out, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{"page-1", "page-2"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(out.Parsed) != 1 {
		t.Fatalf("Parsed len = %d, want 1", len(out.Parsed))
	}
	if out.Parsed[0].SourcePageID != "page-2" {
		t.Fatalf("SourcePageID = %q, want page-2", out.Parsed[0].SourcePageID)
	}
	if len(out.ParseErrors) != 1 {
		t.Fatalf("ParseErrors len = %d, want duplicate warning", len(out.ParseErrors))
	}
	pe := out.ParseErrors[0]
	if pe.Front != "apple" {
		t.Fatalf("ParseErrors[0].Front = %q, want %q", pe.Front, "apple")
	}
	if pe.Back != uniqueBack(1) {
		t.Fatalf("ParseErrors[0].Back = %q, want %q (the dropped row's back)", pe.Back, uniqueBack(1))
	}
	if len(cards.upserted) != 1 || cards.upserted[0].Back != uniqueBack(2) {
		t.Fatalf("upserted = %+v, want latest back", cards.upserted)
	}
	// Partial success (rows>0, parseErrs>0): persistence must still run.
	if cardgroups.calls != 1 {
		t.Fatalf("EnsureByName calls = %d, want 1", cardgroups.calls)
	}
	if cards.upsertCalls != 1 {
		t.Fatalf("upsertCalls = %d, want 1", cards.upsertCalls)
	}
	if *txCalls != 1 {
		t.Fatalf("txCalls = %d, want 1", *txCalls)
	}
}

func TestNotionSyncUsecase_DuplicateFrontSamePageLastWins(t *testing.T) {
	t.Parallel()

	// Two duplicates inside a single page: the second occurrence wins, the
	// first is reported as a parse error whose Line points at the discarded
	// (earlier) row.
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\napple " + uniqueBack(2) + "\n"},
	}}
	cardgroups := &mockNotionCardgroupRepo{cg: &domain.Cardgroup{ID: "cg-target"}}
	cards := &mockNotionCardRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1}}
	tx, txCalls := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	out, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{"page-1"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(out.Parsed) != 1 {
		t.Fatalf("Parsed len = %d, want 1 (later occurrence wins)", len(out.Parsed))
	}
	if out.Parsed[0].Back != uniqueBack(2) {
		t.Fatalf("kept Back = %q, want %q (the later occurrence)", out.Parsed[0].Back, uniqueBack(2))
	}
	if len(out.ParseErrors) != 1 {
		t.Fatalf("ParseErrors len = %d, want 1", len(out.ParseErrors))
	}
	// The retained row is on line 2; the discarded (earlier) row is line 1.
	// dedupeParsedRows MUST anchor the error to the *discarded* row's line.
	if out.ParseErrors[0].Line != 1 {
		t.Fatalf("ParseErrors[0].Line = %d, want 1 (the discarded row's line)", out.ParseErrors[0].Line)
	}
	pe2 := out.ParseErrors[0]
	if pe2.Front != "apple" {
		t.Fatalf("ParseErrors[0].Front = %q, want %q", pe2.Front, "apple")
	}
	if pe2.Back != uniqueBack(1) {
		t.Fatalf("ParseErrors[0].Back = %q, want %q (the dropped row's back)", pe2.Back, uniqueBack(1))
	}
	// Partial success (rows>0, parseErrs>0): persistence must still run.
	if cardgroups.calls != 1 {
		t.Fatalf("EnsureByName calls = %d, want 1", cardgroups.calls)
	}
	if cards.upsertCalls != 1 {
		t.Fatalf("upsertCalls = %d, want 1", cards.upsertCalls)
	}
	if *txCalls != 1 {
		t.Fatalf("txCalls = %d, want 1", *txCalls)
	}
}

func TestNotionSyncUsecase_InputValidation(t *testing.T) {
	t.Parallel()

	// Each case proves a specific failure branch in Sync's input-validation
	// preamble. All must surface as ErrNotionSyncInvalidInput so the handler
	// can map them to a 4xx response.
	t.Run("empty page ids", func(t *testing.T) {
		t.Parallel()
		uc := newValidationUsecase()
		_, err := uc.Sync(context.Background(), SyncFromNotionInput{
			PageIDs:       []string{"", "  "},
			OwnerID:       "owner-1",
			CardgroupName: "English",
		})
		if !errors.Is(err, ErrNotionSyncInvalidInput) {
			t.Fatalf("err = %v, want ErrNotionSyncInvalidInput", err)
		}
	})

	t.Run("empty owner", func(t *testing.T) {
		t.Parallel()
		uc := newValidationUsecase()
		_, err := uc.Sync(context.Background(), SyncFromNotionInput{
			PageIDs:       []string{"page-1"},
			OwnerID:       "  ",
			CardgroupName: "English",
		})
		if !errors.Is(err, ErrNotionSyncInvalidInput) {
			t.Fatalf("err = %v, want ErrNotionSyncInvalidInput", err)
		}
	})

	t.Run("empty cardgroup name", func(t *testing.T) {
		t.Parallel()
		uc := newValidationUsecase()
		_, err := uc.Sync(context.Background(), SyncFromNotionInput{
			PageIDs:       []string{"page-1"},
			OwnerID:       "owner-1",
			CardgroupName: " ",
		})
		if !errors.Is(err, ErrNotionSyncInvalidInput) {
			t.Fatalf("err = %v, want ErrNotionSyncInvalidInput", err)
		}
	})
}

func newValidationUsecase() *NotionSyncUsecase {
	tx, _ := dictTxRunner()
	return NewNotionSyncUsecaseWithTx(
		&stubNotionFetcher{},
		&mockNotionCardgroupRepo{},
		&mockNotionCardRepo{},
		tx,
		nil,
	)
}

func TestNotionSyncUsecase_SoftParseFailure(t *testing.T) {
	t.Parallel()

	// Page text where every line is a lexer-level failure. textdic.Process
	// returns err == nil with len(words) == 0 and len(errs) > 0, so Sync must
	// surface ErrNotionSyncParse (HTTP 422) rather than silently succeeding
	// with an empty persist.
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "@\n"},
	}}
	cardgroups := &mockNotionCardgroupRepo{}
	cards := &mockNotionCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	_, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{"page-1"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncParse) {
		t.Fatalf("err = %v, want ErrNotionSyncParse", err)
	}
	// Persistence must be skipped entirely when all rows fail to parse.
	if cardgroups.calls != 0 {
		t.Fatalf("EnsureByName calls = %d, want 0 (persistence must be skipped)", cardgroups.calls)
	}
	if cards.upsertCalls != 0 || *txCalls != 0 {
		t.Fatalf("persistence ran: upserts=%d tx=%d, want all zero", cards.upsertCalls, *txCalls)
	}
}

func TestNotionSyncUsecase_MixedSkipAndLexerErrorIsNotSkipOnly(t *testing.T) {
	t.Parallel()

	// "orphan\n@\n" produces zero parsed rows, one skip (line 1: lone front),
	// and one lexer-level failure (line 2: '@'). The skip-only short-circuit
	// MUST NOT fire because not every error is a skip — the call must surface
	// ErrNotionSyncParse and skip persistence.
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "orphan\n@\n"},
	}}
	cardgroups := &mockNotionCardgroupRepo{}
	cards := &mockNotionCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	_, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{"page-1"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncParse) {
		t.Fatalf("err = %v, want ErrNotionSyncParse (mixed skip + lexer error is not skip-only)", err)
	}
	// Persistence must be skipped entirely: no cardgroup creation, no repo
	// calls, no tx open.
	if cardgroups.calls != 0 {
		t.Fatalf("EnsureByName calls = %d, want 0 (persistence must be skipped)", cardgroups.calls)
	}
	if cards.upsertCalls != 0 || cards.listCalls != 0 || cards.deleteCalls != 0 {
		t.Fatalf("card repo calls (upsert=%d list=%d delete=%d), want all zero",
			cards.upsertCalls, cards.listCalls, cards.deleteCalls)
	}
	if *txCalls != 0 {
		t.Fatalf("tx calls = %d, want 0", *txCalls)
	}
}

func TestNotionSyncUsecase_LoneFrontDoesNotOverwriteExistingBack(t *testing.T) {
	t.Parallel()

	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple\n"},
	}}
	cardgroups := &mockNotionCardgroupRepo{cg: &domain.Cardgroup{ID: "cg-target"}}
	cards := &mockNotionCardRepo{existingFronts: []string{"apple"}}
	tx, txCalls := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	out, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{"page-1"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(out.ParseErrors) != 1 {
		t.Fatalf("ParseErrors len = %d, want 1", len(out.ParseErrors))
	}
	if out.ParseErrors[0].Line != 1 {
		t.Fatalf("ParseErrors[0].Line = %d, want 1", out.ParseErrors[0].Line)
	}
	if out.ParseErrors[0].Message != "skipped: front-only line (no definition)" {
		t.Fatalf("ParseErrors[0].Message = %q, want skipped front-only line", out.ParseErrors[0].Message)
	}
	if len(cards.upserted) != 0 {
		t.Fatalf("upserted = %+v, want no cards for skipped lone front", cards.upserted)
	}
	if cards.upsertCalls != 0 || cards.listCalls != 0 || cards.deleteCalls != 0 || *txCalls != 0 {
		t.Fatalf("persistence ran: upserts=%d list=%d deletes=%d tx=%d, want all zero",
			cards.upsertCalls, cards.listCalls, cards.deleteCalls, *txCalls)
	}
	if cardgroups.calls != 0 {
		t.Fatalf("EnsureByName calls = %d, want 0 (skip-only sync must not create or mutate cardgroups)", cardgroups.calls)
	}
}

func TestNotionSyncUsecase_FetchErrorSkipsPersistence(t *testing.T) {
	t.Parallel()

	boom := errors.New("notion down")
	fetcher := &stubNotionFetcher{err: boom}
	cardgroups := &mockNotionCardgroupRepo{}
	cards := &mockNotionCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	_, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{"page-1"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncFetch) || !errors.Is(err, boom) {
		t.Fatalf("err = %v, want fetch + boom", err)
	}
	if cardgroups.calls != 0 || cards.upsertCalls != 0 || *txCalls != 0 {
		t.Fatalf("persistence ran: cardgroups=%d upserts=%d tx=%d", cardgroups.calls, cards.upsertCalls, *txCalls)
	}
}

func TestNotionSyncUsecase_PersistError(t *testing.T) {
	t.Parallel()

	boom := errors.New("db down")
	fetcher := &stubNotionFetcher{pages: []notion.Page{{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"}}}
	cardgroups := &mockNotionCardgroupRepo{cg: &domain.Cardgroup{ID: "cg-target"}}
	cards := &mockNotionCardRepo{upsertErr: boom}
	tx, _ := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	_, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{"page-1"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncPersist) || !errors.Is(err, boom) {
		t.Fatalf("err = %v, want persist + boom", err)
	}
}

func TestNotionSyncUsecase_CardgroupEnsureError(t *testing.T) {
	t.Parallel()

	// EnsureByName fails before the transaction is opened. Sync must surface
	// ErrNotionSyncPersist and the underlying db error, and must not enter the
	// tx or touch the card repository at all.
	dbErr := errors.New("db error")
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"},
	}}
	cardgroups := &mockNotionCardgroupRepo{err: dbErr}
	cards := &mockNotionCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	_, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{"page-1"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncPersist) {
		t.Fatalf("err = %v, want ErrNotionSyncPersist", err)
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want wrapped db error", err)
	}
	if !strings.Contains(err.Error(), "db error") {
		t.Fatalf("err.Error() = %q, want it to contain \"db error\"", err.Error())
	}
	// The transaction must not have been entered; no card repo method should run.
	if *txCalls != 0 {
		t.Fatalf("tx calls = %d, want 0 (tx must not be entered on EnsureByName failure)", *txCalls)
	}
	if cards.upsertCalls != 0 || cards.listCalls != 0 || cards.deleteCalls != 0 {
		t.Fatalf("card repo calls (upsert=%d list=%d delete=%d), want all zero",
			cards.upsertCalls, cards.listCalls, cards.deleteCalls)
	}
}

func TestNotionSyncUsecase_ListFrontsError(t *testing.T) {
	t.Parallel()

	// ListFrontsByCardgroupTx fails inside the transaction. Sync must surface
	// ErrNotionSyncPersist and the underlying list error so the caller can
	// distinguish a persistence failure from a fetch or parse failure.
	listErr := errors.New("list error")
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"},
	}}
	cardgroups := &mockNotionCardgroupRepo{cg: &domain.Cardgroup{ID: "cg-target"}}
	cards := &mockNotionCardRepo{listErr: listErr}
	tx, txCalls := dictTxRunner()
	uc := NewNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, nil)

	_, err := uc.Sync(context.Background(), SyncFromNotionInput{
		PageIDs:       []string{"page-1"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncPersist) {
		t.Fatalf("err = %v, want ErrNotionSyncPersist", err)
	}
	if !errors.Is(err, listErr) {
		t.Fatalf("err = %v, want wrapped list error", err)
	}
	if !strings.Contains(err.Error(), "list error") {
		t.Fatalf("err.Error() = %q, want it to contain \"list error\"", err.Error())
	}
	// The tx was entered (upsert ran before list), but must have been rolled back.
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
	// UpsertManyTx runs before ListFrontsByCardgroupTx; it must have been called.
	if cards.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", cards.upsertCalls)
	}
	// Delete must not have been reached after the list failure.
	if cards.deleteCalls != 0 {
		t.Fatalf("delete calls = %d, want 0 (must not be reached after list error)", cards.deleteCalls)
	}
}
