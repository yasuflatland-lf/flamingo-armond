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

// NotionSyncMasterCardgroupRepository is the narrow consumer interface the
// master-targeted sync needs from the master cardgroup repository. Master
// cardgroups are owner-less; EnsureByName looks up or creates the named
// catalog template under a 'master' advisory-lock namespace.
type NotionSyncMasterCardgroupRepository interface {
	EnsureByName(ctx context.Context, name string) (*domain.MasterCardgroup, error)
}

// NotionSyncMasterCardRepository is the narrow consumer interface the
// master-targeted sync needs from the master card repository: the bulk upsert
// plus the diff-prune pair (list current fronts, delete the stale ones).
type NotionSyncMasterCardRepository interface {
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.MasterCard) (repository.UpsertManyTxResult, error)
	ListFrontsByMasterCardgroupTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string) ([]string, error)
	DeleteByMasterCardgroupAndFrontsTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string, fronts []string) (int64, error)
}

// MasterNotionSyncUsecase syncs Notion pages into the admin-only master_*
// catalog tables. The destination master cardgroup is identified by name only
// (owner-less); EnsureByName resolves the name to an id under an advisory lock.
type MasterNotionSyncUsecase struct {
	fetcher             notion.Fetcher
	masterCardgroupRepo NotionSyncMasterCardgroupRepository
	masterCardRepo      NotionSyncMasterCardRepository
	tx                  txRunner
	logger              *slog.Logger
}

// SyncToMasterInput is the input for a master-targeted Notion sync. Unlike the
// user-targeted variant it carries no OwnerID: master cardgroups are owner-less
// and identified by MasterCardgroupName alone.
type SyncToMasterInput struct {
	PageIDs             []string
	MasterCardgroupName string
}

type ParsedRow struct {
	Front        string `json:"front"`
	Back         string `json:"back"`
	SourcePageID string `json:"sourcePageId"`
	Line         int    `json:"line"`
}

type MasterNotionSyncOutput struct {
	CardgroupID string            `json:"cardgroupId"`
	Inserted    int64             `json:"inserted"`
	Updated     int64             `json:"updated"`
	Deleted     int64             `json:"deleted"`
	Parsed      []ParsedRow       `json:"parsed"`
	ParseErrors []CardImportError `json:"parseErrors"`
}

func NewMasterNotionSyncUsecase(
	fetcher notion.Fetcher,
	masterCardgroupRepo NotionSyncMasterCardgroupRepository,
	masterCardRepo NotionSyncMasterCardRepository,
	db *gorm.DB,
	logger *slog.Logger,
) *MasterNotionSyncUsecase {
	if logger == nil {
		panic("usecase: notion sync: logger is required")
	}
	uc := &MasterNotionSyncUsecase{
		fetcher:             fetcher,
		masterCardgroupRepo: masterCardgroupRepo,
		masterCardRepo:      masterCardRepo,
		logger:              logger,
	}
	uc.tx = newTxRunner(db)
	return uc
}

func NewMasterNotionSyncUsecaseWithTx(
	fetcher notion.Fetcher,
	masterCardgroupRepo NotionSyncMasterCardgroupRepository,
	masterCardRepo NotionSyncMasterCardRepository,
	tx txRunner,
	logger *slog.Logger,
) *MasterNotionSyncUsecase {
	if logger == nil {
		panic("usecase: notion sync: logger is required")
	}
	return &MasterNotionSyncUsecase{
		fetcher:             fetcher,
		masterCardgroupRepo: masterCardgroupRepo,
		masterCardRepo:      masterCardRepo,
		tx:                  tx,
		logger:              logger,
	}
}

func (u *MasterNotionSyncUsecase) Sync(ctx context.Context, input SyncToMasterInput) (MasterNotionSyncOutput, error) {
	pageIDs := normalizePageIDs(input.PageIDs)
	if len(pageIDs) == 0 {
		return MasterNotionSyncOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "page ids are required")
	}
	cgName, cgNameErr := domain.ParseCardgroupName(input.MasterCardgroupName)
	if cgNameErr != nil {
		// The typed *ucerr.ValidationError is preserved in the chain for log-structured
		// detail and any future GraphQL/CLI consumer; the REST handler intentionally
		// collapses it to a generic 422 body.
		return MasterNotionSyncOutput{}, eris.Wrap(
			errors.Join(ErrNotionSyncInvalidInput, translateCardgroupNameErr(cgNameErr)),
			"cardgroup name is invalid",
		)
	}
	if u.fetcher == nil || u.masterCardgroupRepo == nil || u.masterCardRepo == nil || u.tx == nil {
		return MasterNotionSyncOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "dependencies are not configured")
	}

	pages, err := u.fetcher.FetchPages(ctx, pageIDs)
	if err != nil {
		return MasterNotionSyncOutput{}, eris.Wrap(errors.Join(ErrNotionSyncFetch, err), "fetch pages")
	}

	rows, parseErrs, err := parseNotionPages(ctx, u.logger, pages)
	if err != nil {
		return MasterNotionSyncOutput{}, eris.Wrap(errors.Join(ErrNotionSyncParse, err), "parse pages")
	}
	// Soft skip-only input: every non-blank line was intentionally skipped by
	// the grammar. Report the skipped rows, but do not persist an empty sync
	// that would delete existing cards from the target master cardgroup.
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
			return MasterNotionSyncOutput{ParseErrors: parseErrs}, nil
		}
		u.logger.WarnContext(ctx, "notion sync: all rows failed to parse",
			"parse_error_count", len(parseErrs),
			"first_error_line", parseErrs[0].Line,
			"first_error_kind", parseErrs[0].Kind,
			"first_error_snippet", parseErrs[0].Snippet,
		)
		return MasterNotionSyncOutput{}, eris.Wrap(ErrNotionSyncParse, "all rows failed to parse")
	}
	if len(rows) > cardImportParsedRowCap {
		return MasterNotionSyncOutput{}, eris.Wrap(ErrNotionSyncInvalidInput, "parsed rows exceed cap")
	}

	cardgroup, err := u.masterCardgroupRepo.EnsureByName(ctx, cgName.String())
	if err != nil {
		return MasterNotionSyncOutput{}, eris.Wrap(errors.Join(ErrNotionSyncPersist, err), "ensure master cardgroup")
	}

	rows, parseErrs = dedupeParsedRows(rows, parseErrs)
	cards := u.masterCardsFromParsedRows(ctx, cardgroup.ID, rows)
	// Derive the keep-set from the validated cards' (trimmed) fronts so the
	// diff-prune step stays consistent with what was actually upserted: rows
	// dropped by ParseCardText validation are absent here and so are pruned if
	// a stale card with the same front exists. Keyed by frontMatchKey so the
	// prune is case-insensitive, matching the citext master_cards.front column.
	notionFronts := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		notionFronts[frontMatchKey(card.Front.String())] = struct{}{}
	}

	out := MasterNotionSyncOutput{
		CardgroupID: cardgroup.ID,
		Parsed:      rows,
		ParseErrors: parseErrs,
	}
	err = u.tx(ctx, func(tx *gorm.DB) error {
		upserted, err := u.masterCardRepo.UpsertManyTx(ctx, tx, cards)
		if err != nil {
			return eris.Wrap(err, "upsert master cards")
		}
		currentFronts, err := u.masterCardRepo.ListFrontsByMasterCardgroupTx(ctx, tx, cardgroup.ID)
		if err != nil {
			return eris.Wrap(err, "list current fronts")
		}
		deleteFronts := frontsToDelete(currentFronts, notionFronts)
		deleted, err := u.masterCardRepo.DeleteByMasterCardgroupAndFrontsTx(ctx, tx, cardgroup.ID, deleteFronts)
		if err != nil {
			return eris.Wrap(err, "delete stale master cards")
		}
		out.Inserted = upserted.Inserted
		out.Updated = upserted.Updated
		out.Deleted = deleted
		return nil
	})
	if err != nil {
		return MasterNotionSyncOutput{}, eris.Wrap(errors.Join(ErrNotionSyncPersist, err), "persist master cards")
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

// frontMatchKey is the case-insensitive key used to match Notion fronts against
// each other (dedupe) and against existing master_cards rows (prune). It mirrors
// the citext semantics of the master_cards.front column, whose unique index and
// upsert conflict target compare case-insensitively. Fronts are ASCII by
// construction — the textdic lexer restricts the front token to ASCII letters —
// so strings.ToLower agrees with Postgres lower() with no locale ambiguity.
func frontMatchKey(front string) string {
	return strings.ToLower(front)
}

func dedupeParsedRows(rows []ParsedRow, errs []CardImportError) ([]ParsedRow, []CardImportError) {
	// Keyed by frontMatchKey so case-variant fronts (e.g. "Drive" / "drive")
	// collapse to one row. This is mandatory, not cosmetic: feeding two
	// case-variant rows into the citext upsert would make a single multi-row
	// INSERT hit the same ON CONFLICT target twice ("cannot affect row a second
	// time"). The last occurrence wins, keeping its original case for storage.
	lastIndex := make(map[string]int, len(rows))
	for i, row := range rows {
		lastIndex[frontMatchKey(row.Front)] = i
	}
	out := make([]ParsedRow, 0, len(rows))
	for i, row := range rows {
		if lastIndex[frontMatchKey(row.Front)] != i {
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

// masterCardsFromParsedRows builds master cards from deduped, document-order
// parsed rows. Position is the index in the deduped slice (0..n-1) so the
// catalog reflects the original Notion document position. Each row is built
// through domain.NewMasterCard, which validates and normalizes Front and Back;
// a row whose constructor fails (empty/whitespace-only or over-CardTextMax
// front/back, or an ID-generation failure) is skipped (not persisted) and a
// structured warn is emitted so the rest of the sync still imports the valid
// rows. See docs/backend/error-wrapping/log-structured-event-when-batch-item-fails.md.
func (u *MasterNotionSyncUsecase) masterCardsFromParsedRows(
	ctx context.Context, masterCardgroupID string, rows []ParsedRow,
) []*domain.MasterCard {
	now := time.Now().UTC()
	cards := make([]*domain.MasterCard, 0, len(rows))
	for i, row := range rows {
		card, err := domain.NewMasterCard(masterCardgroupID, row.Front, row.Back, i)
		if err != nil {
			u.warnSkippedMasterRow(ctx, masterCardgroupID, i, err)
			continue
		}
		// NewMasterCard stamps per-card timestamps; pin the whole sync batch to
		// one now.
		card.CreatedAt = now
		card.UpdatedAt = now
		cards = append(cards, card)
	}
	return cards
}

// warnSkippedMasterRow emits a structured warn for a master-card row whose
// constructor (domain.NewMasterCard) failed and was dropped from the import.
func (u *MasterNotionSyncUsecase) warnSkippedMasterRow(
	ctx context.Context, masterCardgroupID string, position int, reason error,
) {
	u.logger.WarnContext(ctx, "notion sync: skipping invalid master card row",
		"cardgroupID", masterCardgroupID,
		"position", position,
		"field", masterRowErrorField(reason),
		"reason", reason.Error(),
	)
}

// masterRowErrorField maps a NewMasterCard error to the field label used in the
// skip warning. Front and back CardText sentinels resolve to their field; any
// other error (e.g. an ID-generation failure) is attributed to "id".
func masterRowErrorField(err error) string {
	switch {
	case errors.Is(err, domain.ErrCardFrontRequired), errors.Is(err, domain.ErrCardFrontTooLong):
		return "front"
	case errors.Is(err, domain.ErrCardBackRequired), errors.Is(err, domain.ErrCardBackTooLong):
		return "back"
	default:
		return "id"
	}
}

func frontsToDelete(current []string, notionFronts map[string]struct{}) []string {
	// Compare case-insensitively (frontMatchKey) so an existing row whose stored
	// case differs from the current Notion line (e.g. DB "Drive" vs Notion
	// "drive", reconciled by the citext upsert) is not mistaken for stale and
	// pruned. The original stored front is passed to the delete so the
	// WHERE front IN (...) targets the exact row.
	deleteFronts := make([]string, 0, len(current))
	for _, front := range current {
		if _, ok := notionFronts[frontMatchKey(front)]; !ok {
			deleteFronts = append(deleteFronts, front)
		}
	}
	return deleteFronts
}
