package usecase

import (
	"context"
	"errors"
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
	tx, _ := dictTxRunner()
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
	if len(cards.upserted) != 1 || cards.upserted[0].Back != uniqueBack(2) {
		t.Fatalf("upserted = %+v, want latest back", cards.upserted)
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
