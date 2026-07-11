package resolver

import (
	"context"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/usecase"
)

// toLearningStatsModel maps the usecase learning-stats result to the generated
// GraphQL model, hydrating each deck's Cardgroup and each struggling card's
// Card from their ids via the existing per-request DataLoader (loaders.Cardgroup
// and loaders.Card respectively), each via its own two-phase Load-then-resolve
// pass. A missing loader registry or a Load failure is surfaced as an
// INTERNAL/CANCELLED wire error. This is resolver field-hydration (DataLoader
// batching + error classification), not a pure mapping, so it lives here beside
// its resolver rather than in mapper.go.
func toLearningStatsModel(ctx context.Context, res *usecase.LearningStatsResult) (*model.LearningStats, error) {
	loaders, gqlErr := loadersOrInternal(ctx)
	if gqlErr != nil {
		return nil, gqlErr
	}
	// Two-phase Load-then-resolve: issue every Cardgroup.Load first so all N
	// keys land in the same DataLoader batch window, then invoke each captured
	// thunk. Invoking the thunk inside the first loop would block on a
	// single-key batch per iteration (this loop is one sequential goroutine, not
	// gqlgen's concurrent per-object field-resolver fan-out that batches every
	// other Load call site), defeating batching entirely.
	thunks := make([]func() (*domain.Cardgroup, error), len(res.Decks))
	for i, d := range res.Decks {
		thunks[i] = loaders.Cardgroup.Load(ctx, d.CardgroupID)
	}
	decks := make([]*model.DeckMastery, 0, len(res.Decks))
	for i, d := range res.Decks {
		cg, err := thunks[i]()
		if err != nil {
			return nil, classifyLoaderErr(ctx, err, "resolver: stats: cardgroup")
		}
		decks = append(decks, &model.DeckMastery{
			Cardgroup:    toCardgroupModel(cg),
			TotalCards:   d.TotalCards,
			LearnedCards: d.LearnedCards,
			MatureCards:  d.MatureCards,
		})
	}

	// Hydrate each struggling card from its id via the Card DataLoader, using the
	// same two-phase Load-then-resolve so every key lands in one batch window.
	cardThunks := make([]func() (*domain.Card, error), len(res.StrugglingCards))
	for i, sc := range res.StrugglingCards {
		cardThunks[i] = loaders.Card.Load(ctx, sc.CardID)
	}
	struggling := make([]*model.StrugglingCard, 0, len(res.StrugglingCards))
	for i, sc := range res.StrugglingCards {
		card, err := cardThunks[i]()
		if err != nil {
			return nil, classifyLoaderErr(ctx, err, "resolver: stats: struggling card")
		}
		struggling = append(struggling, &model.StrugglingCard{
			Card:      toCardModel(card),
			Lapses:    sc.Lapses,
			Stability: sc.Stability,
		})
	}

	return &model.LearningStats{
		Mastery: &model.MasteryBreakdown{
			InProgress:   res.Mastery.InProgress,
			Learned:      res.Mastery.Learned,
			Mature:       res.Mastery.Mature,
			TotalStudied: res.Mastery.TotalStudied,
		},
		Decks:           decks,
		OwnsAnyDeck:     res.OwnsAnyDeck,
		Performance:     toPerformanceMetricsModel(res.Performance),
		StrugglingCards: struggling,
	}, nil
}
