package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/notion"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
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

type mockMasterCardgroupRepo struct {
	cg    *domain.MasterCardgroup
	err   error
	calls int
	name  string
}

func (m *mockMasterCardgroupRepo) EnsureByName(_ context.Context, name string) (*domain.MasterCardgroup, error) {
	m.calls++
	m.name = name
	if m.err != nil {
		return nil, m.err
	}
	if m.cg == nil {
		m.cg = &domain.MasterCardgroup{ID: "mcg-created", Name: domain.CardgroupName(name)}
	}
	return m.cg, nil
}

type mockMasterCardRepo struct {
	existingFronts []string
	upsertResult   repository.UpsertManyTxResult
	upsertErr      error
	listErr        error
	deleteErr      error

	upserted       []*domain.MasterCard
	deletedFronts  []string
	upsertCalls    int
	listCalls      int
	deleteCalls    int
	deleteAffected int64
}

func (m *mockMasterCardRepo) UpsertManyTx(_ context.Context, _ *gorm.DB, cards []*domain.MasterCard) (repository.UpsertManyTxResult, error) {
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

func (m *mockMasterCardRepo) ListFrontsByMasterCardgroupTx(_ context.Context, _ *gorm.DB, _ string) ([]string, error) {
	m.listCalls++
	if m.listErr != nil {
		return nil, m.listErr
	}
	return append([]string(nil), m.existingFronts...), nil
}

func (m *mockMasterCardRepo) DeleteByMasterCardgroupAndFrontsTx(_ context.Context, _ *gorm.DB, _ string, fronts []string) (int64, error) {
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

func TestMasterNotionSyncUsecase_DiffMerge(t *testing.T) {
	t.Parallel()

	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{
			ID:   "page-1",
			Text: "apple " + uniqueBack(1) + "\nbanana " + uniqueBack(2) + "\n",
		},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{
		existingFronts: []string{"apple", "banana", "stale"},
		upsertResult:   repository.UpsertManyTxResult{Inserted: 1, Updated: 1},
	}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	out, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{" page-1 ", "page-1"},
		MasterCardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if out.CardgroupID != "mcg-target" {
		t.Fatalf("CardgroupID = %q, want mcg-target", out.CardgroupID)
	}
	if out.Inserted != 1 || out.Updated != 1 || out.Deleted != 1 {
		t.Fatalf("counts = (%d,%d,%d), want (1,1,1)", out.Inserted, out.Updated, out.Deleted)
	}
	if len(out.Parsed) != 2 {
		t.Fatalf("Parsed len = %d, want 2", len(out.Parsed))
	}
	if cardgroups.name != "English" {
		t.Fatalf("EnsureByName name = %q, want English", cardgroups.name)
	}
	if len(fetcher.ids) != 1 || fetcher.ids[0] != "page-1" {
		t.Fatalf("fetch ids = %v, want [page-1]", fetcher.ids)
	}
	if len(cards.upserted) != 2 {
		t.Fatalf("upserted len = %d, want 2", len(cards.upserted))
	}
	// Cards land via the master upsert path, scoped to the master cardgroup id.
	if cards.upserted[0].MasterCardgroupID != "mcg-target" {
		t.Fatalf("upserted master cardgroup = %q, want mcg-target", cards.upserted[0].MasterCardgroupID)
	}
	if len(cards.deletedFronts) != 1 || cards.deletedFronts[0] != "stale" {
		t.Fatalf("deleted fronts = %v, want [stale]", cards.deletedFronts)
	}
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
}

func TestMasterNotionSyncUsecase_DuplicateFrontLastWins(t *testing.T) {
	t.Parallel()

	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"},
		{ID: "page-2", Text: "apple " + uniqueBack(2) + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1}}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	out, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1", "page-2"},
		MasterCardgroupName: "English",
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
	if len(cards.upserted) != 1 || string(cards.upserted[0].Back) != uniqueBack(2) {
		t.Fatalf("upserted = %+v, want latest back", cards.upserted)
	}
	// After deduplication the surviving slice has exactly one card; its Position
	// must be 0 (the first — and only — index in the deduped document-order slice).
	if cards.upserted[0].Position != 0 {
		t.Fatalf("upserted[0].Position = %d, want 0 (single surviving card after dedupe)", cards.upserted[0].Position)
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

func TestMasterNotionSyncUsecase_DuplicateFrontSamePageLastWins(t *testing.T) {
	t.Parallel()

	// Two duplicates inside a single page: the second occurrence wins, the
	// first is reported as a parse error whose Line points at the discarded
	// (earlier) row.
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\napple " + uniqueBack(2) + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1}}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	out, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
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
	// The dedupe step MUST anchor the error to the *discarded* row's line.
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

func TestMasterNotionSyncUsecase_CaseInsensitiveDedupe(t *testing.T) {
	t.Parallel()

	// Two case-variant fronts in one sync ("Drive" then "drive") must collapse to
	// a single row. Without case-insensitive dedupe both rows would reach the
	// citext upsert and a single multi-row INSERT would hit the same ON CONFLICT
	// target twice ("cannot affect row a second time"). The later occurrence wins.
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "Drive " + uniqueBack(1) + "\ndrive " + uniqueBack(2) + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1}}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	out, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(out.Parsed) != 1 {
		t.Fatalf("Parsed len = %d, want 1 (case variants collapse)", len(out.Parsed))
	}
	if out.Parsed[0].Front != "drive" {
		t.Fatalf("kept Front = %q, want %q (later occurrence)", out.Parsed[0].Front, "drive")
	}
	if len(cards.upserted) != 1 {
		t.Fatalf("upserted len = %d, want 1", len(cards.upserted))
	}
	if string(cards.upserted[0].Front) != "drive" || string(cards.upserted[0].Back) != uniqueBack(2) {
		t.Fatalf("upserted = (%q,%q), want (drive,%q)",
			cards.upserted[0].Front, cards.upserted[0].Back, uniqueBack(2))
	}
	if len(out.ParseErrors) != 1 {
		t.Fatalf("ParseErrors len = %d, want 1 duplicate warning", len(out.ParseErrors))
	}
	if out.ParseErrors[0].Front != "Drive" {
		t.Fatalf("ParseErrors[0].Front = %q, want %q (the discarded variant's original case)",
			out.ParseErrors[0].Front, "Drive")
	}
	if *txCalls != 1 {
		t.Fatalf("txCalls = %d, want 1", *txCalls)
	}
}

func TestMasterNotionSyncUsecase_CaseInsensitivePrune(t *testing.T) {
	t.Parallel()

	// The DB stored "Apple" (capitalized). Notion now has "apple" (lowercase),
	// which the citext upsert reconciles onto the same row. The diff-prune must
	// NOT treat "Apple" as stale just because its stored case differs from the
	// Notion line. A genuinely absent front ("stale") is still pruned.
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{
		existingFronts: []string{"Apple", "stale"},
		upsertResult:   repository.UpsertManyTxResult{Updated: 1},
	}
	tx, _ := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(cards.deletedFronts) != 1 || cards.deletedFronts[0] != "stale" {
		t.Fatalf("deleted fronts = %v, want [stale] (Apple matched case-insensitively)",
			cards.deletedFronts)
	}
}

func TestMasterNotionSyncUsecase_InputValidation(t *testing.T) {
	t.Parallel()

	// Each case proves a specific failure branch in Sync's input-validation
	// preamble. All must surface as ErrNotionSyncInvalidInput so the handler
	// can map them to a 4xx response.
	t.Run("empty page ids", func(t *testing.T) {
		t.Parallel()
		uc := newMasterValidationUsecase()
		_, err := uc.Sync(context.Background(), SyncToMasterInput{
			PageIDs:             []string{"", "  "},
			MasterCardgroupName: "English",
		})
		if !errors.Is(err, ErrNotionSyncInvalidInput) {
			t.Fatalf("err = %v, want ErrNotionSyncInvalidInput", err)
		}
	})

	t.Run("empty cardgroup name", func(t *testing.T) {
		t.Parallel()
		uc := newMasterValidationUsecase()
		_, err := uc.Sync(context.Background(), SyncToMasterInput{
			PageIDs:             []string{"page-1"},
			MasterCardgroupName: " ",
		})
		if !errors.Is(err, ErrNotionSyncInvalidInput) {
			t.Fatalf("err = %v, want ErrNotionSyncInvalidInput", err)
		}
		var v *ucerr.ValidationError
		if !errors.As(err, &v) {
			t.Fatalf("err chain has no *ucerr.ValidationError, got %v", err)
		}
		if v.Field != "name" {
			t.Fatalf("ValidationError.Field = %q, want %q", v.Field, "name")
		}
		if v.Message != "name is required" {
			t.Fatalf("ValidationError.Message = %q, want %q", v.Message, "name is required")
		}
	})

	t.Run("over-cap cardgroup name", func(t *testing.T) {
		t.Parallel()
		uc := newMasterValidationUsecase()
		overCap := strings.Repeat("a", domain.CardgroupNameMax+1)
		_, err := uc.Sync(context.Background(), SyncToMasterInput{
			PageIDs:             []string{"page-1"},
			MasterCardgroupName: overCap,
		})
		if !errors.Is(err, ErrNotionSyncInvalidInput) {
			t.Fatalf("err = %v, want ErrNotionSyncInvalidInput", err)
		}
		var v *ucerr.ValidationError
		if !errors.As(err, &v) {
			t.Fatalf("err chain has no *ucerr.ValidationError, got %v", err)
		}
		if v.Field != "name" {
			t.Fatalf("ValidationError.Field = %q, want %q", v.Field, "name")
		}
		wantSubstr := fmt.Sprintf("%d", domain.CardgroupNameMax)
		if !strings.Contains(v.Message, wantSubstr) {
			t.Fatalf("ValidationError.Message = %q, want it to contain cap %q", v.Message, wantSubstr)
		}
	})
}

func newMasterValidationUsecase() *MasterNotionSyncUsecase {
	tx, _ := dictTxRunner()
	return newMasterNotionSyncUsecaseWithTx(
		&stubNotionFetcher{},
		&mockMasterCardgroupRepo{},
		&mockMasterCardRepo{},
		tx,
		newTestLogger(),
	)
}

func TestMasterNotionSyncUsecase_SoftParseFailure(t *testing.T) {
	t.Parallel()

	// Page text where every line is a lexer-level failure. textdic.Process
	// returns err == nil with len(words) == 0 and len(errs) > 0, so Sync must
	// surface ErrNotionSyncParse (HTTP 422) rather than silently succeeding
	// with an empty persist.
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "@\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{}
	cards := &mockMasterCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
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

func TestMasterNotionSyncUsecase_MixedSkipAndLexerErrorIsNotSkipOnly(t *testing.T) {
	t.Parallel()

	// "orphan\n@\n" produces zero parsed rows, one skip (line 1: lone front),
	// and one lexer-level failure (line 2: '@'). The skip-only short-circuit
	// MUST NOT fire because not every error is a skip — the call must surface
	// ErrNotionSyncParse and skip persistence.
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "orphan\n@\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{}
	cards := &mockMasterCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncParse) {
		t.Fatalf("err = %v, want ErrNotionSyncParse (mixed skip + lexer error is not skip-only)", err)
	}
	// Persistence must be skipped entirely: no master cardgroup creation, no
	// repo calls, no tx open.
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

func TestMasterNotionSyncUsecase_LoneFrontDoesNotOverwriteExistingBack(t *testing.T) {
	t.Parallel()

	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{existingFronts: []string{"apple"}}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	out, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
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

func TestMasterNotionSyncUsecase_FetchErrorSkipsPersistence(t *testing.T) {
	t.Parallel()

	boom := errors.New("notion down")
	fetcher := &stubNotionFetcher{err: boom}
	cardgroups := &mockMasterCardgroupRepo{}
	cards := &mockMasterCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncFetch) || !errors.Is(err, boom) {
		t.Fatalf("err = %v, want fetch + boom", err)
	}
	if cardgroups.calls != 0 || cards.upsertCalls != 0 || *txCalls != 0 {
		t.Fatalf("persistence ran: cardgroups=%d upserts=%d tx=%d", cardgroups.calls, cards.upsertCalls, *txCalls)
	}
}

func TestMasterNotionSyncUsecase_PersistError(t *testing.T) {
	t.Parallel()

	boom := errors.New("db down")
	fetcher := &stubNotionFetcher{pages: []notion.Page{{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"}}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{upsertErr: boom}
	tx, _ := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncPersist) || !errors.Is(err, boom) {
		t.Fatalf("err = %v, want persist + boom", err)
	}
}

func TestMasterNotionSyncUsecase_CardgroupEnsureError(t *testing.T) {
	t.Parallel()

	// EnsureByName fails before the transaction is opened. Sync must surface
	// ErrNotionSyncPersist and the underlying db error, and must not enter the
	// tx or touch the card repository at all.
	dbErr := errors.New("db error")
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{err: dbErr}
	cards := &mockMasterCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
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

func TestMasterNotionSyncUsecase_ListFrontsError(t *testing.T) {
	t.Parallel()

	// ListFrontsByMasterCardgroupTx fails inside the transaction. Sync must
	// surface ErrNotionSyncPersist and the underlying list error so the caller
	// can distinguish a persistence failure from a fetch or parse failure.
	listErr := errors.New("list error")
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{listErr: listErr}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
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
	// UpsertManyTx runs before ListFrontsByMasterCardgroupTx; it must have been called.
	if cards.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", cards.upsertCalls)
	}
	// Delete must not have been reached after the list failure.
	if cards.deleteCalls != 0 {
		t.Fatalf("delete calls = %d, want 0 (must not be reached after list error)", cards.deleteCalls)
	}
}

// TestMasterNotionSyncUsecase_SkipOnlyLogFields verifies that the skip-only
// branch emits an InfoContext log record whose structured fields include
// first_line, first_kind, and first_snippet, pinned to the known values for a
// lone-front input ("apple\n" → line 1, kind "FRONT_ONLY", snippet "apple").
//
// Not parallel: injects a logger directly into the usecase, so it does not
// mutate the global slog default.
func TestMasterNotionSyncUsecase_SkipOnlyLogFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{}
	cards := &mockMasterCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, logger)

	out, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(out.ParseErrors) != 1 {
		t.Fatalf("ParseErrors len = %d, want 1", len(out.ParseErrors))
	}
	// Skip-only: persistence must be bypassed.
	if cardgroups.calls != 0 || cards.upsertCalls != 0 || *txCalls != 0 {
		t.Fatalf("persistence ran: cardgroups=%d upserts=%d tx=%d, want all zero",
			cardgroups.calls, cards.upsertCalls, *txCalls)
	}

	records := decodeJSONRecords(t, buf.Bytes())
	var skipRec map[string]any
	for _, rec := range records {
		if rec["msg"] == "notion sync: skip-only payload, no persistence" {
			skipRec = rec
			break
		}
	}
	if skipRec == nil {
		t.Fatalf("no InfoContext record with msg %q found in log output:\n%s",
			"notion sync: skip-only payload, no persistence", buf.String())
	}

	// first_line: JSON numbers decode as float64 in map[string]any.
	if fl, ok := skipRec["first_line"].(float64); !ok || int(fl) != 1 {
		t.Errorf("first_line = %v (%T), want 1", skipRec["first_line"], skipRec["first_line"])
	}
	if skipRec["first_kind"] != "FRONT_ONLY" {
		t.Errorf("first_kind = %v, want %q", skipRec["first_kind"], "FRONT_ONLY")
	}
	if skipRec["first_snippet"] != "apple" {
		t.Errorf("first_snippet = %v, want %q", skipRec["first_snippet"], "apple")
	}
}

// TestAllCardImportErrorsSkipped verifies the allCardImportErrorsSkipped
// predicate across the full domain of CardImportErrorKind values.
func TestAllCardImportErrorsSkipped(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input []CardImportError
		want  bool
	}{
		{
			name:  "empty slice",
			input: []CardImportError{},
			want:  false,
		},
		{
			name:  "FRONT_ONLY only",
			input: []CardImportError{{Kind: CardImportErrKindFrontOnly}},
			want:  true,
		},
		{
			name:  "BACK_ONLY only",
			input: []CardImportError{{Kind: CardImportErrKindBackOnly}},
			want:  true,
		},
		{
			name:  "FRONT_ONLY and BACK_ONLY",
			input: []CardImportError{{Kind: CardImportErrKindFrontOnly}, {Kind: CardImportErrKindBackOnly}},
			want:  true,
		},
		{
			name:  "FRONT_ONLY and UNRECOGNIZED",
			input: []CardImportError{{Kind: CardImportErrKindFrontOnly}, {Kind: CardImportErrKindUnrecognized}},
			want:  false,
		},
		{
			name:  "FRONT_ONLY and HARD",
			input: []CardImportError{{Kind: CardImportErrKindFrontOnly}, {Kind: CardImportErrKindHard}},
			want:  false,
		},
		{
			name:  "DUPLICATE only",
			input: []CardImportError{{Kind: CardImportErrKindDuplicate}},
			want:  false,
		},
		{
			name:  "UNKNOWN only — programming-error sentinel is not a skip",
			input: []CardImportError{{Kind: CardImportErrKindUnknown}},
			want:  false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := allCardImportErrorsSkipped(tc.input); got != tc.want {
				t.Errorf("allCardImportErrorsSkipped(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestMasterNotionSyncUsecase_WarnBranchLogFields verifies that the warn branch
// (rows == 0, parseErrs > 0, not skip-only) emits a WarnContext log record
// with structured fields anchored to the first error.
//
// Input "@broken\n" yields one UNRECOGNIZED parse error, which is not a
// skip-only kind, so the warn branch fires and persistence is skipped.
//
// Not parallel: injects a logger directly into the usecase, so it does not
// mutate the global slog default.
func TestMasterNotionSyncUsecase_WarnBranchLogFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "@broken\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{}
	cards := &mockMasterCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, logger)

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncParse) {
		t.Fatalf("expected ErrNotionSyncParse, got %v", err)
	}
	// Persistence must be bypassed.
	if cardgroups.calls != 0 || cards.upsertCalls != 0 || *txCalls != 0 {
		t.Fatalf("persistence ran: cardgroups=%d upserts=%d tx=%d, want all zero",
			cardgroups.calls, cards.upsertCalls, *txCalls)
	}

	records := decodeJSONRecords(t, buf.Bytes())
	var warnRec map[string]any
	for _, rec := range records {
		if rec["msg"] == "notion sync: all rows failed to parse" {
			warnRec = rec
			break
		}
	}
	if warnRec == nil {
		t.Fatalf("no WarnContext record with msg %q found in log output:\n%s",
			"notion sync: all rows failed to parse", buf.String())
	}

	// parse_error_count: JSON numbers decode as float64 in map[string]any.
	if v, ok := warnRec["parse_error_count"].(float64); !ok || int(v) != 1 {
		t.Errorf("parse_error_count = %v (%T), want 1", warnRec["parse_error_count"], warnRec["parse_error_count"])
	}
	if v, ok := warnRec["first_error_line"].(float64); !ok || int(v) != 1 {
		t.Errorf("first_error_line = %v (%T), want 1", warnRec["first_error_line"], warnRec["first_error_line"])
	}
	if warnRec["first_error_kind"] != "UNRECOGNIZED" {
		t.Errorf("first_error_kind = %v, want %q", warnRec["first_error_kind"], "UNRECOGNIZED")
	}
	if warnRec["first_error_snippet"] != "@broken" {
		t.Errorf("first_error_snippet = %v, want %q", warnRec["first_error_snippet"], "@broken")
	}
}

// TestMasterCardsFromParsedRows_AssignsContiguousPositions verifies that each
// master card receives a Position equal to its index (0..n-1) in the deduped
// document-order slice, including rows that originate from more than one source
// page.
func TestMasterCardsFromParsedRows_AssignsContiguousPositions(t *testing.T) {
	t.Parallel()
	// Rows come from two different source pages, simulating a multi-page Notion
	// sync.  After dedupe (already done before this call) these are the survivors.
	rows := []ParsedRow{
		{Front: "apple", Back: "fruit", SourcePageID: "page-1", Line: 1},
		{Front: "banana", Back: "fruit", SourcePageID: "page-1", Line: 2},
		{Front: "carrot", Back: "vegetable", SourcePageID: "page-2", Line: 1},
		{Front: "daikon", Back: "vegetable", SourcePageID: "page-2", Line: 2},
	}

	const masterCardgroupID = "mcg-test"
	cards, skipped := masterCardsFromParsedRows(masterCardgroupID, rows, testSyncNow)

	if len(skipped) != 0 {
		t.Fatalf("skipped = %+v, want none (all rows valid)", skipped)
	}
	if len(cards) != len(rows) {
		t.Fatalf("len(cards) = %d, want %d", len(cards), len(rows))
	}

	for i, card := range cards {
		// Position must equal the index in the deduped slice.
		if card.Position != i {
			t.Errorf("cards[%d].Position = %d, want %d", i, card.Position, i)
		}
		// Front order must be preserved.
		if string(card.Front) != rows[i].Front {
			t.Errorf("cards[%d].Front = %q, want %q", i, card.Front, rows[i].Front)
		}
		// MasterCardgroupID must be propagated.
		if card.MasterCardgroupID != masterCardgroupID {
			t.Errorf("cards[%d].MasterCardgroupID = %q, want %q", i, card.MasterCardgroupID, masterCardgroupID)
		}
	}
}

// TestMasterCardsFromParsedRows_SkipsInvalidRows verifies that rows failing
// CardText validation are dropped as skip diagnostics while the valid rows still
// produce cards. Position stays the index in the original deduped slice, so
// survivors keep gaps rather than being renumbered.
func TestMasterCardsFromParsedRows_SkipsInvalidRows(t *testing.T) {
	t.Parallel()

	rows := []ParsedRow{
		{Front: "apple", Back: "fruit", SourcePageID: "page-1", Line: 1},  // valid
		{Front: "   ", Back: "fruit", SourcePageID: "page-1", Line: 2},    // empty front -> skip
		{Front: "carrot", Back: "", SourcePageID: "page-1", Line: 3},      // empty back -> skip
		{Front: strings.Repeat("x", 501), Back: "ok", Line: 4},            // oversized front -> skip
		{Front: "banana", Back: "fruit", SourcePageID: "page-1", Line: 5}, // valid
	}

	const masterCardgroupID = "mcg-skip"
	cards, skipped := masterCardsFromParsedRows(masterCardgroupID, rows, testSyncNow)

	// Only the two valid rows survive, keeping their original deduped-slice
	// indices as Position (0 and 4).
	if len(cards) != 2 {
		t.Fatalf("len(cards) = %d, want 2", len(cards))
	}
	// One diagnostic per dropped row, keyed by the source line, so the caller can
	// tell "some rows dropped" from "every row dropped".
	if len(skipped) != 3 {
		t.Fatalf("len(skipped) = %d, want 3", len(skipped))
	}
	// Reason is carried rather than pre-formatted: the plan must not touch a logger.
	wantSkips := []struct {
		line     int
		position int
		field    string
	}{
		{line: 2, position: 1, field: "front"},
		{line: 3, position: 2, field: "back"},
		{line: 4, position: 3, field: "front"},
	}
	for i, want := range wantSkips {
		if skipped[i].Diagnostic.Line != want.line {
			t.Errorf("skipped[%d].Diagnostic.Line = %d, want %d", i, skipped[i].Diagnostic.Line, want.line)
		}
		if skipped[i].Diagnostic.Kind != CardImportErrKindHard {
			t.Errorf("skipped[%d].Diagnostic.Kind = %q, want %q", i, skipped[i].Diagnostic.Kind, CardImportErrKindHard)
		}
		if skipped[i].Diagnostic.Message == "" {
			t.Errorf("skipped[%d].Diagnostic.Message is empty, want the constructor error text", i)
		}
		if skipped[i].Position != want.position {
			t.Errorf("skipped[%d].Position = %d, want %d", i, skipped[i].Position, want.position)
		}
		if skipped[i].Reason == nil {
			t.Fatalf("skipped[%d].Reason = nil, want the constructor error", i)
		}
		if got := masterRowErrorField(skipped[i].Reason); got != want.field {
			t.Errorf("masterRowErrorField(skipped[%d].Reason) = %q, want %q", i, got, want.field)
		}
	}
	if string(cards[0].Front) != "apple" || cards[0].Position != 0 {
		t.Errorf("cards[0] = {Front:%q, Position:%d}, want {apple, 0}", cards[0].Front, cards[0].Position)
	}
	if string(cards[1].Front) != "banana" || cards[1].Position != 4 {
		t.Errorf("cards[1] = {Front:%q, Position:%d}, want {banana, 4}", cards[1].Front, cards[1].Position)
	}
}

// TestMasterNotionSyncUsecase_SkippedRowWarnFields pins the structured warn Sync
// emits per dropped row, keyed on (cardgroupID, position, field, reason). Two
// invalid rows, not one, so the multiplicity is pinned and not just the first
// record's shape. Not parallel: injects a logger into the usecase.
func TestMasterNotionSyncUsecase_SkippedRowWarnFields(t *testing.T) {
	overLongBack := jpRunes(domain.CardTextMax + 1)
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) +
			"\nbanana " + overLongBack +
			"\ncarrot " + overLongBack + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{}
	tx, _ := dictTxRunner()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, logger)

	if _, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Every warn, not just the first: a collapse of the fan-out must fail here.
	type skip struct {
		position int
		field    string
	}
	got := make(map[skip]bool)
	warnCount := 0
	for _, rec := range decodeJSONRecords(t, buf.Bytes()) {
		if rec["msg"] != "notion sync: skipping invalid master card row" {
			continue
		}
		warnCount++
		if rec["cardgroupID"] != "mcg-target" {
			t.Errorf("cardgroupID = %v, want %q", rec["cardgroupID"], "mcg-target")
		}
		if rec["reason"] == nil || rec["reason"] == "" {
			t.Errorf("reason is empty for record %v, want the constructor error text", rec)
		}
		// position: JSON numbers decode as float64 in map[string]any.
		pos, ok := rec["position"].(float64)
		if !ok {
			t.Errorf("position = %v (%T), want a number", rec["position"], rec["position"])
		}
		field, _ := rec["field"].(string)
		got[skip{int(pos), field}] = true
	}
	// Positions are 1 and 2, not 0 and 1: they index the deduped slice.
	want := []skip{{1, "back"}, {2, "back"}}
	if warnCount != len(want) {
		t.Fatalf("warn records = %d, want %d (log output:\n%s)", warnCount, len(want), buf.String())
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing warn for skipped row position=%d field=%q (got=%v)", w.position, w.field, got)
		}
	}
}

// TestMasterNotionSyncUsecase_DeleteError verifies that a failure in
// DeleteByMasterCardgroupAndFrontsTx (the prune-stale step) surfaces as
// ErrNotionSyncPersist with the underlying delete error preserved in the
// chain. UpsertManyTx runs before the delete, so it must have been called
// exactly once even though the transaction is rolled back on the delete
// failure.
func TestMasterNotionSyncUsecase_DeleteError(t *testing.T) {
	t.Parallel()

	deleteErr := errors.New("delete error")
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	// "stale" is present in the cardgroup but absent from the Notion payload, so
	// frontsToDelete yields ["stale"] and the prune-stale delete runs.
	cards := &mockMasterCardRepo{existingFronts: []string{"apple", "stale"}, deleteErr: deleteErr}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncPersist) {
		t.Fatalf("err = %v, want ErrNotionSyncPersist", err)
	}
	if !errors.Is(err, deleteErr) {
		t.Fatalf("err = %v, want wrapped delete error", err)
	}
	if !strings.Contains(err.Error(), "delete error") {
		t.Fatalf("err.Error() = %q, want it to contain \"delete error\"", err.Error())
	}
	// The tx was entered, but must have been rolled back on the delete failure.
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
	// UpsertManyTx runs before DeleteByMasterCardgroupAndFrontsTx; it must have
	// run exactly once before the delete failed.
	if cards.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", cards.upsertCalls)
	}
	if cards.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1 (the stale front triggers the delete)", cards.deleteCalls)
	}
}

// TestMasterNotionSyncUsecase_OverCapRejected covers the parsed-row cap gate at
// the usecase level: a Notion payload that parses to more than
// cardImportParsedRowCap rows is rejected with ErrNotionSyncInvalidInput before
// any persistence runs. Mirrors TestCardImportUsecase_PayloadOverCapBadInput on
// the card-import path.
func TestMasterNotionSyncUsecase_OverCapRejected(t *testing.T) {
	t.Parallel()

	const n = cardImportParsedRowCap + 1
	var b strings.Builder
	for i := range n {
		b.WriteString(stringFront("front", i))
		b.WriteString(" ")
		b.WriteString(uniqueBack(i))
		b.WriteString("\n")
	}
	fetcher := &stubNotionFetcher{pages: []notion.Page{{ID: "page-1", Text: b.String()}}}
	cardgroups := &mockMasterCardgroupRepo{}
	cards := &mockMasterCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if !errors.Is(err, ErrNotionSyncInvalidInput) {
		t.Fatalf("err = %v, want ErrNotionSyncInvalidInput (over-cap payload)", err)
	}
	// The cap gate runs before EnsureByName and the transaction, so no
	// persistence may have happened.
	if cardgroups.calls != 0 {
		t.Fatalf("EnsureByName calls = %d, want 0 (cap gate precedes persistence)", cardgroups.calls)
	}
	if cards.upsertCalls != 0 || cards.listCalls != 0 || cards.deleteCalls != 0 {
		t.Fatalf("card repo calls (upsert=%d list=%d delete=%d), want all zero",
			cards.upsertCalls, cards.listCalls, cards.deleteCalls)
	}
	if *txCalls != 0 {
		t.Fatalf("tx calls = %d, want 0 (no tx on over-cap rejection)", *txCalls)
	}
}

// TestMasterNotionSyncUsecase_NoDeletions pins the prune "nothing stale" branch:
// when the Notion fronts exactly match the cardgroup's existing fronts,
// frontsToDelete is empty, but DeleteByMasterCardgroupAndFrontsTx is still
// invoked once with an empty slice. The repository's empty-IN guard treats an
// empty slice as a no-op, so the destructive path stays harmless while the call
// itself remains unconditional.
func TestMasterNotionSyncUsecase_NoDeletions(t *testing.T) {
	t.Parallel()

	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\nbanana " + uniqueBack(2) + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	// existingFronts == notionFronts, so nothing is stale.
	cards := &mockMasterCardRepo{
		existingFronts: []string{"apple", "banana"},
		upsertResult:   repository.UpsertManyTxResult{Updated: 2},
	}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	out, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if out.Deleted != 0 {
		t.Fatalf("out.Deleted = %d, want 0 (nothing stale)", out.Deleted)
	}
	if cards.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", cards.upsertCalls)
	}
	// The delete is unconditional even with nothing to prune.
	if cards.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1 (delete is called unconditionally)", cards.deleteCalls)
	}
	if len(cards.deletedFronts) != 0 {
		t.Fatalf("deletedFronts = %v, want empty slice (nothing stale)", cards.deletedFronts)
	}
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
}

// TestMasterNotionSyncUsecase_AllRowsFailDomainValidation pins the whole-payload
// guard: every row parses at the grammar level but fails domain construction, so
// the keep-set would be empty and the diff-prune would wipe the master cardgroup.
// The payload is rejected before the persistence transaction, but the warns are
// not skipped along with it: they are emitted ahead of the guard's early return.
func TestMasterNotionSyncUsecase_AllRowsFailDomainValidation(t *testing.T) {
	t.Parallel()

	overLongBack := jpRunes(domain.CardTextMax + 1)
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + overLongBack + "\nbanana " + overLongBack + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{existingFronts: []string{"apple", "banana"}}
	tx, txCalls := dictTxRunner()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, logger)

	_, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if err == nil {
		t.Fatal("Sync error = nil, want ErrNotionSyncInvalidInput")
	}
	if !errors.Is(err, ErrNotionSyncInvalidInput) {
		t.Fatalf("Sync error = %v, want ErrNotionSyncInvalidInput", err)
	}
	if cards.upsertCalls != 0 || cards.listCalls != 0 || cards.deleteCalls != 0 || *txCalls != 0 {
		t.Fatalf("persistence ran: upserts=%d list=%d deletes=%d tx=%d, want all zero",
			cards.upsertCalls, cards.listCalls, cards.deleteCalls, *txCalls)
	}
	if len(cards.deletedFronts) != 0 {
		t.Fatalf("deletedFronts = %v, want none (existing master cards must survive)", cards.deletedFronts)
	}
	// One per-row warn per skipped row, emitted before the guard's early return.
	warnCount := 0
	for _, rec := range decodeJSONRecords(t, buf.Bytes()) {
		if rec["msg"] == "notion sync: skipping invalid master card row" {
			warnCount++
		}
	}
	if warnCount != 2 {
		t.Fatalf("per-row skip warns = %d, want 2 (log output:\n%s)", warnCount, buf.String())
	}
}

// TestMasterNotionSyncUsecase_PartiallyInvalidBatchStillPrunes pins the
// complement of the guard above: when at least one row survives domain
// construction, the per-row prune semantics are unchanged — a row dropped by
// NewMasterCard is absent from the keep-set and its stale card is pruned.
func TestMasterNotionSyncUsecase_PartiallyInvalidBatchStillPrunes(t *testing.T) {
	t.Parallel()

	overLongBack := jpRunes(domain.CardTextMax + 1)
	fetcher := &stubNotionFetcher{pages: []notion.Page{
		{ID: "page-1", Text: "apple " + uniqueBack(1) + "\nbanana " + overLongBack + "\n"},
	}}
	cardgroups := &mockMasterCardgroupRepo{cg: &domain.MasterCardgroup{ID: "mcg-target"}}
	cards := &mockMasterCardRepo{existingFronts: []string{"apple", "banana"}}
	tx, txCalls := dictTxRunner()
	uc := newMasterNotionSyncUsecaseWithTx(fetcher, cardgroups, cards, tx, newTestLogger())

	out, err := uc.Sync(context.Background(), SyncToMasterInput{
		PageIDs:             []string{"page-1"},
		MasterCardgroupName: "English",
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(cards.upserted) != 1 || cards.upserted[0].Front.String() != "apple" {
		t.Fatalf("upserted = %+v, want only the valid apple row", cards.upserted)
	}
	if len(cards.deletedFronts) != 1 || cards.deletedFronts[0] != "banana" {
		t.Fatalf("deletedFronts = %v, want [banana] (per-row prune is deliberate)", cards.deletedFronts)
	}
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
	if out.CardgroupID != "mcg-target" {
		t.Fatalf("CardgroupID = %q, want mcg-target", out.CardgroupID)
	}
}
