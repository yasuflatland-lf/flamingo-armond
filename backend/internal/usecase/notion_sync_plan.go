package usecase

import (
	"time"

	"backend/internal/domain"
)

// masterRowSkip records one parsed row that domain.NewMasterCard rejected.
// It carries both halves of the diagnostic so the plan computation can stay
// pure: the caller-facing CardImportError (line, message, front) that feeds the
// whole-payload rejection error, and the log-facing detail (the row's position
// in the deduped slice and the raw constructor error) the structured skip warn
// is keyed on. The plan never logs; the sync iterates these and emits the warns.
type masterRowSkip struct {
	Position   int
	Reason     error
	Diagnostic CardImportError
}

// notionSyncPlan is the complete description of what a Notion sync would
// persist, computed without touching a repository, a transaction runner, a
// clock or a logger:
//
//   - Rows / ParseErrors: the deduped rows and the accumulated diagnostics that
//     the sync reports back to the caller.
//   - Cards: the master cards to upsert, in deduped document order.
//   - KeepFronts: the case-insensitive keep-set the diff-prune step compares the
//     currently stored fronts against.
//   - DomainSkips: one entry per row dropped by domain construction.
//
// Isolating the plan from its surroundings turns "never persist a plan that
// keeps nothing" into a checkable property of a value (NothingValidToPersist)
// instead of an ad-hoc condition wired into the sync routine.
type notionSyncPlan struct {
	Rows        []ParsedRow
	ParseErrors []CardImportError
	Cards       []*domain.MasterCard
	KeepFronts  map[string]struct{}
	DomainSkips []masterRowSkip
}

// NothingValidToPersist reports whether the plan carries rows but produced no
// card at all — every row parsed at the grammar level and then failed domain
// construction (e.g. an editor pushed every back side past domain.CardTextMax).
//
// Such a plan has an empty KeepFronts, so persisting it would make the
// diff-prune delete every existing card in the target master cardgroup, wiping
// a published deck. The sync must refuse it before opening the transaction.
// A plan with no rows at all is not "nothing valid to persist": the empty
// payload is handled earlier by the grammar-skip short-circuit, and a plan with
// no rows and no skips has nothing to complain about.
func (p notionSyncPlan) NothingValidToPersist() bool {
	return len(p.Cards) == 0 && len(p.Rows) > 0
}

// computeSyncPlan turns the grammar-parsed rows of a Notion sync into the plan
// the persistence step executes. It is pure: no repository, no transaction
// runner, no logger, and no clock — now is injected so the whole batch is
// pinned to a single timestamp and the function stays testable without fakes.
//
// masterCardgroupID is resolved by the caller (the cardgroup must exist before
// its id can be stamped onto the cards), but nothing else about the plan
// depends on the outside world.
//
// The steps run in the order the pipeline requires: dedupe first (it appends
// "duplicate front" diagnostics that must not be visible to the skip-only
// classifier, which the caller therefore runs beforehand), then master-card
// construction, then keep-set derivation from the surviving cards.
func computeSyncPlan(
	rows []ParsedRow,
	parseErrs []CardImportError,
	masterCardgroupID string,
	now time.Time,
) notionSyncPlan {
	dedupedRows, allErrs := dedupeParsedRows(rows, parseErrs)
	cards, skips := masterCardsFromParsedRows(masterCardgroupID, dedupedRows, now)
	// Derive the keep-set from the validated cards' (trimmed) fronts so the
	// diff-prune step stays consistent with what is actually upserted: rows
	// dropped by ParseCardText validation are absent here and so are pruned if
	// a stale card with the same front exists. Keyed by frontMatchKey so the
	// prune is case-insensitive, matching the citext master_cards.front column.
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

// masterCardsFromParsedRows builds master cards from deduped, document-order
// parsed rows. Position is the index in the deduped slice (0..n-1) so the
// catalog reflects the original Notion document position. Each row is built
// through domain.NewMasterCard, which validates and normalizes Front and Back;
// a row whose constructor fails (empty/whitespace-only or over-CardTextMax
// front/back, or an ID-generation failure) is skipped (not persisted) so the
// rest of the sync still imports the valid rows.
//
// The second return value collects one masterRowSkip per dropped row. It lets
// the caller distinguish "some rows dropped" from "every row dropped" — the
// latter being a whole-payload rejection rather than a per-row skip — and
// carries the detail the caller needs to emit the structured skip warn. See
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
		// NewMasterCard stamps per-card timestamps; pin the whole sync batch to
		// one now.
		card.CreatedAt = now
		card.UpdatedAt = now
		cards = append(cards, card)
	}
	return cards, skipped
}
