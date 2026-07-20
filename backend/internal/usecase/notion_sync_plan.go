package usecase

import (
	"time"

	"backend/internal/domain"
)

// masterRowSkip records one parsed row that domain.NewMasterCard rejected,
// carrying both the caller-facing diagnostic and the log-facing detail.
// The plan deliberately does not log: it stays pure, and the sync emits the
// structured warns from these entries.
type masterRowSkip struct {
	Position   int
	Reason     error
	Diagnostic CardImportError
}

// notionSyncPlan is everything a Notion sync would persist, computed without a
// repository, transaction runner, clock or logger. Keeping it a plain value —
// rather than wiring these steps into the sync routine — is what turns "never
// persist a plan that keeps nothing" into a checkable property of that value
// (NothingValidToPersist) instead of an ad-hoc condition.
type notionSyncPlan struct {
	Rows        []ParsedRow
	ParseErrors []CardImportError
	Cards       []*domain.MasterCard
	KeepFronts  map[string]struct{}
	DomainSkips []masterRowSkip
}

// NothingValidToPersist reports whether every row survived the grammar and then
// failed domain construction. Such a plan has an empty KeepFronts, so persisting
// it would let the diff-prune wipe a published deck. A plan with no rows at all
// deliberately does not qualify: the empty payload is caught earlier by the
// grammar-skip short-circuit.
func (p notionSyncPlan) NothingValidToPersist() bool {
	return len(p.Cards) == 0 && len(p.Rows) > 0
}

// computeSyncPlan turns grammar-parsed rows into the plan the persistence step
// executes. It takes no repository, transaction runner, logger or clock — now
// is injected rather than read here so the batch shares one timestamp and the
// function needs no fakes to test.
func computeSyncPlan(
	rows []ParsedRow,
	parseErrs []CardImportError,
	masterCardgroupID string,
	now time.Time,
) notionSyncPlan {
	// Dedupe cannot run before the caller's skip-only classifier: it appends
	// "duplicate front" diagnostics the classifier must not see.
	dedupedRows, allErrs := dedupeParsedRows(rows, parseErrs)
	cards, skips := masterCardsFromParsedRows(masterCardgroupID, dedupedRows, now)
	// Keyed off the validated cards, not the rows: a row dropped by validation
	// must not keep a stale card alive. frontMatchKey rather than the raw front,
	// because master_cards.front is citext.
	keepFronts := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		keepFronts[frontMatchKey(card.Front.String())] = struct{}{}
	}
	return notionSyncPlan{
		Rows:        dedupedRows,
		ParseErrors: allErrs,
		Cards:       cards,
		KeepFronts:  keepFronts,
		DomainSkips: skips,
	}
}

// masterCardsFromParsedRows builds master cards from deduped rows, positioned
// by index so the catalog mirrors Notion document order. A row the constructor
// rejects is skipped rather than failing the whole sync; the returned skips
// carry the caller diagnostic and the detail the warn is keyed on. See
// docs/backend/error-wrapping/log-structured-event-when-batch-item-fails.md.
func masterCardsFromParsedRows(
	masterCardgroupID string, rows []ParsedRow, now time.Time,
) ([]*domain.MasterCard, []masterRowSkip) {
	cards := make([]*domain.MasterCard, 0, len(rows))
	skipped := make([]masterRowSkip, 0)
	for i, row := range rows {
		card, err := domain.NewMasterCard(masterCardgroupID, row.Front, row.Back, i)
		if err != nil {
			skipped = append(skipped, masterRowSkip{
				Position: i,
				Reason:   err,
				Diagnostic: CardImportError{
					Line:    row.Line,
					Message: err.Error(),
					Kind:    CardImportErrKindHard,
					Front:   row.Front,
				},
			})
			continue
		}
		// updated_at is deliberately not pinned: the DB trigger owns it.
		card.CreatedAt = now
		cards = append(cards, card)
	}
	return cards, skipped
}
