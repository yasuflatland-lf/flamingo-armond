package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/notion"
	"backend/internal/repository"
	"backend/internal/textdic"
)

var (
	ErrNotionSyncInvalidInput = errors.New("usecase: notion sync invalid input")
	ErrNotionSyncFetch        = errors.New("usecase: notion sync fetch failed")
	ErrNotionSyncParse        = errors.New("usecase: notion sync parse failed")
	ErrNotionSyncPersist      = errors.New("usecase: notion sync persist failed")
)

type NotionSyncCardRepository interface {
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error)
	ListFrontsByCardgroupTx(ctx context.Context, tx *gorm.DB, cardgroupID string) ([]string, error)
	DeleteByCardgroupAndFrontsTx(ctx context.Context, tx *gorm.DB, cardgroupID string, fronts []string) (int64, error)
}

type NotionSyncCardgroupRepository interface {
	EnsureByName(ctx context.Context, ownerID, name string) (*domain.Cardgroup, error)
}

type NotionSyncUsecase struct {
	fetcher       notion.Fetcher
	cardRepo      NotionSyncCardRepository
	cardgroupRepo NotionSyncCardgroupRepository
	tx            txRunner
	logger        *slog.Logger
}

type SyncFromNotionInput struct {
	PageIDs       []string
	OwnerID       string
	CardgroupName string
}

type ParsedRow struct {
	Front        string `json:"front"`
	Back         string `json:"back"`
	SourcePageID string `json:"sourcePageId"`
	Line         int    `json:"line"`
}

type SyncFromNotionOutput struct {
	CardgroupID string                      `json:"cardgroupId"`
	Inserted    int64                       `json:"inserted"`
	Updated     int64                       `json:"updated"`
	Deleted     int64                       `json:"deleted"`
	Parsed      []ParsedRow                 `json:"parsed"`
	ParseErrors []DictionaryValidationError `json:"parseErrors"`
}

func NewNotionSyncUsecase(
	fetcher notion.Fetcher,
	cardgroupRepo NotionSyncCardgroupRepository,
	cardRepo NotionSyncCardRepository,
	db *gorm.DB,
	logger *slog.Logger,
) *NotionSyncUsecase {
	uc := &NotionSyncUsecase{
		fetcher:       fetcher,
		cardgroupRepo: cardgroupRepo,
		cardRepo:      cardRepo,
		logger:        logger,
	}
	if db != nil {
		uc.tx = func(ctx context.Context, fn func(tx *gorm.DB) error) error {
			return db.WithContext(ctx).Transaction(fn)
		}
	}
	return uc
}

func NewNotionSyncUsecaseWithTx(
	fetcher notion.Fetcher,
	cardgroupRepo NotionSyncCardgroupRepository,
	cardRepo NotionSyncCardRepository,
	tx txRunner,
	logger *slog.Logger,
) *NotionSyncUsecase {
	return &NotionSyncUsecase{
		fetcher:       fetcher,
		cardgroupRepo: cardgroupRepo,
		cardRepo:      cardRepo,
		tx:            tx,
		logger:        logger,
	}
}

func (u *NotionSyncUsecase) Sync(ctx context.Context, input SyncFromNotionInput) (SyncFromNotionOutput, error) {
	pageIDs := normalizePageIDs(input.PageIDs)
	ownerID := strings.TrimSpace(input.OwnerID)
	cardgroupName := strings.TrimSpace(input.CardgroupName)
	if len(pageIDs) == 0 {
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "page ids are required")
	}
	if ownerID == "" {
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "owner id is required")
	}
	if cardgroupName == "" {
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "cardgroup name is required")
	}
	if u == nil || u.fetcher == nil || u.cardgroupRepo == nil || u.cardRepo == nil || u.tx == nil {
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "dependencies are not configured")
	}

	pages, err := u.fetcher.FetchPages(ctx, pageIDs)
	if err != nil {
		return SyncFromNotionOutput{}, eris.Wrap(errors.Join(ErrNotionSyncFetch, err), "fetch pages")
	}

	rows, parseErrs, err := parseNotionPages(pages)
	if err != nil {
		return SyncFromNotionOutput{}, eris.Wrap(errors.Join(ErrNotionSyncParse, err), "parse pages")
	}
	if len(rows) > dictionaryParsedRowCap {
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "parsed rows exceed cap")
	}

	cardgroup, err := u.cardgroupRepo.EnsureByName(ctx, ownerID, cardgroupName)
	if err != nil {
		return SyncFromNotionOutput{}, eris.Wrap(errors.Join(ErrNotionSyncPersist, err), "ensure cardgroup")
	}

	rows, parseErrs = dedupeParsedRows(rows, parseErrs)
	cards := cardsFromParsedRows(cardgroup.ID, rows)
	notionFronts := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		notionFronts[row.Front] = struct{}{}
	}

	out := SyncFromNotionOutput{
		CardgroupID: cardgroup.ID,
		Parsed:      rows,
		ParseErrors: parseErrs,
	}
	err = u.tx(ctx, func(tx *gorm.DB) error {
		upserted, err := u.cardRepo.UpsertManyTx(ctx, tx, cards)
		if err != nil {
			return eris.Wrap(err, "upsert cards")
		}
		currentFronts, err := u.cardRepo.ListFrontsByCardgroupTx(ctx, tx, cardgroup.ID)
		if err != nil {
			return eris.Wrap(err, "list current fronts")
		}
		deleteFronts := frontsToDelete(currentFronts, notionFronts)
		deleted, err := u.cardRepo.DeleteByCardgroupAndFrontsTx(ctx, tx, cardgroup.ID, deleteFronts)
		if err != nil {
			return eris.Wrap(err, "delete stale cards")
		}
		out.Inserted = upserted.Inserted
		out.Updated = upserted.Updated
		out.Deleted = deleted
		return nil
	})
	if err != nil {
		return SyncFromNotionOutput{}, eris.Wrap(errors.Join(ErrNotionSyncPersist, err), "persist cards")
	}

	if u.logger != nil {
		u.logger.InfoContext(ctx, "notion sync complete",
			"cardgroup_id", out.CardgroupID,
			"inserted", out.Inserted,
			"updated", out.Updated,
			"deleted", out.Deleted,
			"parse_errors_count", len(out.ParseErrors),
		)
	}
	return out, nil
}

func normalizePageIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func parseNotionPages(pages []notion.Page) ([]ParsedRow, []DictionaryValidationError, error) {
	rows := make([]ParsedRow, 0)
	errs := make([]DictionaryValidationError, 0)
	for _, page := range pages {
		words, parseErrs, err := textdic.Process(page.Text)
		if err != nil {
			return nil, nil, err
		}
		for _, word := range words {
			rows = append(rows, ParsedRow{
				Front:        word.Front,
				Back:         word.Back,
				SourcePageID: page.ID,
				Line:         word.Line,
			})
		}
		for _, e := range parseErrs {
			errs = append(errs, DictionaryValidationError{Line: e.Line, Message: e.Message})
		}
	}
	return rows, errs, nil
}

func dedupeParsedRows(rows []ParsedRow, errs []DictionaryValidationError) ([]ParsedRow, []DictionaryValidationError) {
	lastIndex := make(map[string]int, len(rows))
	for i, row := range rows {
		lastIndex[row.Front] = i
	}
	out := make([]ParsedRow, 0, len(rows))
	for i, row := range rows {
		if lastIndex[row.Front] != i {
			errs = append(errs, DictionaryValidationError{
				Line:    row.Line,
				Message: "duplicate front in Notion pages (later occurrence wins)",
			})
			continue
		}
		out = append(out, row)
	}
	return out, errs
}

func cardsFromParsedRows(cardgroupID string, rows []ParsedRow) []*domain.Card {
	now := time.Now().UTC()
	cards := make([]*domain.Card, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, &domain.Card{
			CardgroupID: cardgroupID,
			Front:       row.Front,
			Back:        row.Back,
			FSRS:        domain.NewFSRSStateForNewCard(now),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	return cards
}

func frontsToDelete(current []string, notionFronts map[string]struct{}) []string {
	deleteFronts := make([]string, 0)
	for _, front := range current {
		if _, ok := notionFronts[front]; !ok {
			deleteFronts = append(deleteFronts, front)
		}
	}
	return deleteFronts
}
