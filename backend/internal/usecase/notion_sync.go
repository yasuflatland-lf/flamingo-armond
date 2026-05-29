package usecase

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
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
	CardgroupID string            `json:"cardgroupId"`
	Inserted    int64             `json:"inserted"`
	Updated     int64             `json:"updated"`
	Deleted     int64             `json:"deleted"`
	Parsed      []ParsedRow       `json:"parsed"`
	ParseErrors []CardImportError `json:"parseErrors"`
}

func NewNotionSyncUsecase(
	fetcher notion.Fetcher,
	cardgroupRepo NotionSyncCardgroupRepository,
	cardRepo NotionSyncCardRepository,
	db *gorm.DB,
	logger *slog.Logger,
) *NotionSyncUsecase {
	if logger == nil {
		panic("usecase: notion sync: logger is required")
	}
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
	if logger == nil {
		panic("usecase: notion sync: logger is required")
	}
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
	if len(pageIDs) == 0 {
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "page ids are required")
	}
	if ownerID == "" {
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "owner id is required")
	}
	cgName, cgNameErr := domain.ParseCardgroupName(input.CardgroupName)
	if cgNameErr != nil {
		// The typed *ucerr.ValidationError is preserved in the chain for log-structured
		// detail and any future GraphQL/CLI consumer; the REST handler intentionally
		// collapses it to a generic 422 body.
		return SyncFromNotionOutput{}, eris.Wrap(
			errors.Join(ErrNotionSyncInvalidInput, translateCardgroupNameErr(cgNameErr)),
			"cardgroup name is invalid",
		)
	}
	if u.fetcher == nil || u.cardgroupRepo == nil || u.cardRepo == nil || u.tx == nil {
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "dependencies are not configured")
	}

	pages, err := u.fetcher.FetchPages(ctx, pageIDs)
	if err != nil {
		return SyncFromNotionOutput{}, eris.Wrap(errors.Join(ErrNotionSyncFetch, err), "fetch pages")
	}

	rows, parseErrs, err := parseNotionPages(ctx, u.logger, pages)
	if err != nil {
		return SyncFromNotionOutput{}, eris.Wrap(errors.Join(ErrNotionSyncParse, err), "parse pages")
	}
	// Soft skip-only input: every non-blank line was intentionally skipped by
	// the grammar. Report the skipped rows, but do not persist an empty sync
	// that would delete existing cards from the target cardgroup.
	//
	// This short-circuit MUST run before dedupeParsedRows: dedupe appends
	// non-skip "duplicate front" warnings to parseErrs, which would make
	// allCardImportErrorsSkipped return false for a skip-only payload that
	// happens to also have duplicates added later in the pipeline.
	if len(rows) == 0 && len(parseErrs) > 0 {
		if allCardImportErrorsSkipped(parseErrs) {
			u.logger.InfoContext(ctx, "notion sync: skip-only payload, no persistence",
				"skipped_count", len(parseErrs),
				"first_line", parseErrs[0].Line,
				"first_kind", parseErrs[0].Kind,
				"first_snippet", parseErrs[0].Snippet,
			)
			return SyncFromNotionOutput{ParseErrors: parseErrs}, nil
		}
		u.logger.WarnContext(ctx, "notion sync: all rows failed to parse",
			"parse_error_count", len(parseErrs),
			"first_error_line", parseErrs[0].Line,
			"first_error_kind", parseErrs[0].Kind,
			"first_error_snippet", parseErrs[0].Snippet,
		)
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncParse, "all rows failed to parse")
	}
	if len(rows) > cardImportParsedRowCap {
		return SyncFromNotionOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "parsed rows exceed cap")
	}

	cardgroup, err := u.cardgroupRepo.EnsureByName(ctx, ownerID, cgName.String())
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

	u.logger.InfoContext(ctx, "notion sync complete",
		"cardgroup_id", out.CardgroupID,
		"inserted", out.Inserted,
		"updated", out.Updated,
		"deleted", out.Deleted,
		"parse_errors_count", len(out.ParseErrors),
	)
	return out, nil
}

// allCardImportErrorsSkipped reports whether every error in errs originated
// from a grammar skip production safe to drop silently (lone front / lone back).
//
// Only FRONT_ONLY and BACK_ONLY are considered "skipped" for the purposes of
// the no-persistence short-circuit. UNRECOGNIZED is intentionally hard-failed
// because it signals malformed payload the user likely didn't intend, and
// silently dropping it would mask real corruption. DUPLICATE and HARD are
// obviously hard. UNKNOWN indicates a bug and also falls into the hard branch.
func allCardImportErrorsSkipped(errs []CardImportError) bool {
	for _, e := range errs {
		switch e.Kind {
		case CardImportErrKindFrontOnly, CardImportErrKindBackOnly:
			continue
		default:
			return false
		}
	}
	return len(errs) > 0
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

func parseNotionPages(ctx context.Context, logger *slog.Logger, pages []notion.Page) ([]ParsedRow, []CardImportError, error) {
	rows := make([]ParsedRow, 0, len(pages))
	errs := make([]CardImportError, 0, len(pages))
	for i, page := range pages {
		words, parseErrs, err := textdic.Process(page.Text)
		if err != nil {
			// Earlier pages' parsed rows are about to be dropped on the caller
			// side (the function returns nil rows). Log the failing page so
			// operators can locate the breakage; the caller maps err into
			// ErrNotionSyncParse.
			logger.ErrorContext(ctx, "notion sync: page parse failed",
				"page_index", i,
				"page_id", page.ID,
				"error_name", reflect.TypeOf(err).String(),
				"error", err.Error(),
			)
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
			errs = append(errs, CardImportError{
				Line:    e.Line,
				Message: e.Message,
				Kind:    CardImportErrorKind(e.Kind.String()),
				Snippet: e.Snippet,
			})
		}
	}
	return rows, errs, nil
}

func dedupeParsedRows(rows []ParsedRow, errs []CardImportError) ([]ParsedRow, []CardImportError) {
	lastIndex := make(map[string]int, len(rows))
	for i, row := range rows {
		lastIndex[row.Front] = i
	}
	out := make([]ParsedRow, 0, len(rows))
	for i, row := range rows {
		if lastIndex[row.Front] != i {
			errs = append(errs, CardImportError{
				Line:    row.Line,
				Message: "duplicate front in Notion pages (later occurrence wins)",
				Kind:    CardImportErrKindDuplicate,
				Front:   row.Front,
				Back:    row.Back,
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
	// Position is the index in the deduped document-order slice (0..n-1), so the
	// learn query can order new cards by their original Notion document position.
	for i, row := range rows {
		cards = append(cards, &domain.Card{
			CardgroupID: cardgroupID,
			Front:       domain.CardText(row.Front),
			Back:        domain.CardText(row.Back),
			Position:    i,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	return cards
}

func frontsToDelete(current []string, notionFronts map[string]struct{}) []string {
	deleteFronts := make([]string, 0, len(current))
	for _, front := range current {
		if _, ok := notionFronts[front]; !ok {
			deleteFronts = append(deleteFronts, front)
		}
	}
	return deleteFronts
}
