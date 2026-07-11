package resolver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/graph-gophers/dataloader/v7"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/gqlerr"
	"backend/internal/loader"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

// mockStatsUsecase stubs usecase.StatsUsecase.
type mockStatsUsecase struct {
	result *usecase.LearningStatsResult
	err    error
}

func (m *mockStatsUsecase) MyLearningStats(_ context.Context) (*usecase.LearningStatsResult, error) {
	return m.result, m.err
}

// newStatsSrv builds a gqlgen Server wired to uc; other usecase fields are nil.
func newStatsSrv(uc usecase.StatsUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, uc)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// ctxWithCardgroupLoader installs ONLY an in-memory Cardgroup loader for
// MyLearningStats resolver tests with no struggling cards. A missing
// cardgroup_id returns loader.ErrNotFound, matching production behaviour.
// toLearningStatsModel also reads loaders.Card when StrugglingCards is
// non-empty; use ctxWithCardLoader (below) for those cases.
func ctxWithCardgroupLoader(base context.Context, cgs map[string]*domain.Cardgroup) context.Context {
	loaders := &loader.Loaders{
		Cardgroup: dataloader.NewBatchedLoader(
			func(_ context.Context, keys []string) []*dataloader.Result[*domain.Cardgroup] {
				out := make([]*dataloader.Result[*domain.Cardgroup], len(keys))
				for i, k := range keys {
					if cg, ok := cgs[k]; ok {
						out[i] = &dataloader.Result[*domain.Cardgroup]{Data: cg}
					} else {
						out[i] = &dataloader.Result[*domain.Cardgroup]{Error: loader.ErrNotFound}
					}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

// ctxWithCardgroupLoaderError installs a Cardgroup loader whose batch function
// fails every key with loadErr.
func ctxWithCardgroupLoaderError(base context.Context, loadErr error) context.Context {
	loaders := &loader.Loaders{
		Cardgroup: dataloader.NewBatchedLoader(
			func(_ context.Context, keys []string) []*dataloader.Result[*domain.Cardgroup] {
				out := make([]*dataloader.Result[*domain.Cardgroup], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*domain.Cardgroup]{Error: loadErr}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

const myLearningStatsQuery = `{"query":"{ myLearningStats { mastery { inProgress learned mature totalStudied } decks { cardgroup { id name } totalCards learnedCards matureCards } } }"}`

// TestMyLearningStats_HappyPath verifies the response maps the mastery
// breakdown fields and, crucially, that each deck's TotalCards/LearnedCards/
// MatureCards land in the correct GraphQL fields (a Learned/Mature swap would
// fail these assertions), and that each deck's Cardgroup is hydrated via the
// in-memory DataLoader.
func TestMyLearningStats_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockStatsUsecase{
		result: &usecase.LearningStatsResult{
			Mastery: service.MasteryBreakdown{InProgress: 4, Learned: 6, Mature: 10, TotalStudied: 20},
			Decks: []service.DeckMastery{
				{CardgroupID: "cg-1", TotalCards: 10, LearnedCards: 3, MatureCards: 7},
				{CardgroupID: "cg-2", TotalCards: 5, LearnedCards: 1, MatureCards: 2},
			},
		},
	}
	srv := newStatsSrv(mock)

	cgs := map[string]*domain.Cardgroup{
		"cg-1": {ID: domain.CardgroupID("cg-1"), OwnerID: "u-1", Name: "Deck One"},
		"cg-2": {ID: domain.CardgroupID("cg-2"), OwnerID: "u-1", Name: "Deck Two"},
	}
	ctx := ctxWithCardgroupLoader(authedCtx("u-1"), cgs)
	resp := gqlRequest(t, srv, ctx, myLearningStatsQuery)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	stats, _ := data["myLearningStats"].(map[string]any)
	if stats == nil {
		t.Fatalf("expected data.myLearningStats, got nil; response: %v", resp)
	}

	mastery, _ := stats["mastery"].(map[string]any)
	if mastery == nil {
		t.Fatalf("expected mastery, got nil; response: %v", resp)
	}
	if mastery["inProgress"] != float64(4) || mastery["learned"] != float64(6) ||
		mastery["mature"] != float64(10) || mastery["totalStudied"] != float64(20) {
		t.Fatalf("mastery breakdown mismatch: %v", mastery)
	}

	decks, _ := stats["decks"].([]any)
	if len(decks) != 2 {
		t.Fatalf("expected 2 decks, got %d; response: %v", len(decks), resp)
	}

	deck0, _ := decks[0].(map[string]any)
	if deck0["totalCards"] != float64(10) || deck0["learnedCards"] != float64(3) || deck0["matureCards"] != float64(7) {
		t.Fatalf("deck 0 counts mismatch (possible Learned/Mature swap): %v", deck0)
	}
	cg0, _ := deck0["cardgroup"].(map[string]any)
	if cg0 == nil {
		t.Fatalf("expected deck 0 cardgroup hydrated via DataLoader, got nil; response: %v", resp)
	}
	if cg0["id"] != "cg-1" || cg0["name"] != "Deck One" {
		t.Fatalf("expected deck 0 cardgroup {id: cg-1, name: Deck One}, got %v", cg0)
	}

	deck1, _ := decks[1].(map[string]any)
	if deck1["totalCards"] != float64(5) || deck1["learnedCards"] != float64(1) || deck1["matureCards"] != float64(2) {
		t.Fatalf("deck 1 counts mismatch (possible Learned/Mature swap): %v", deck1)
	}
	cg1, _ := deck1["cardgroup"].(map[string]any)
	if cg1 == nil {
		t.Fatalf("expected deck 1 cardgroup hydrated via DataLoader, got nil; response: %v", resp)
	}
	if cg1["id"] != "cg-2" || cg1["name"] != "Deck Two" {
		t.Fatalf("expected deck 1 cardgroup {id: cg-2, name: Deck Two}, got %v", cg1)
	}
}

// TestMyLearningStats_MissingLoaderMiddleware_ReturnsInternal drives
// MyLearningStats through a context WITHOUT the DataLoader middleware
// installed. toLearningStatsModel's loadersOrInternal guard must surface
// INTERNAL rather than panicking.
func TestMyLearningStats_MissingLoaderMiddleware_ReturnsInternal(t *testing.T) {
	t.Parallel()

	mock := &mockStatsUsecase{
		result: &usecase.LearningStatsResult{
			Mastery: service.MasteryBreakdown{TotalStudied: 0},
		},
	}
	srv := newStatsSrv(mock)

	resp := gqlRequest(t, srv, authedCtx("u-1"), myLearningStatsQuery)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL when loaders not installed, got %q", code)
	}
}

// TestMyLearningStats_CardgroupLoadError_ContextCancelledReturnsCancelled
// verifies that a context.Canceled error from the Cardgroup DataLoader maps to
// CANCELLED via classifyLoaderErr.
func TestMyLearningStats_CardgroupLoadError_ContextCancelledReturnsCancelled(t *testing.T) {
	t.Parallel()

	mock := &mockStatsUsecase{
		result: &usecase.LearningStatsResult{
			Decks: []service.DeckMastery{{CardgroupID: "cg-1", TotalCards: 1}},
		},
	}
	srv := newStatsSrv(mock)

	ctx := ctxWithCardgroupLoaderError(authedCtx("u-1"), context.Canceled)
	resp := gqlRequest(t, srv, ctx, myLearningStatsQuery)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeCancelled) {
		t.Fatalf("expected CANCELLED for context.Canceled loader error, got %q", code)
	}
}

// TestMyLearningStats_CardgroupLoadError_GenericReturnsInternal verifies that
// a non-sentinel Cardgroup DataLoader error maps to INTERNAL via
// classifyLoaderErr.
func TestMyLearningStats_CardgroupLoadError_GenericReturnsInternal(t *testing.T) {
	t.Parallel()

	mock := &mockStatsUsecase{
		result: &usecase.LearningStatsResult{
			Decks: []service.DeckMastery{{CardgroupID: "cg-1", TotalCards: 1}},
		},
	}
	srv := newStatsSrv(mock)

	ctx := ctxWithCardgroupLoaderError(authedCtx("u-1"), errors.New("db down"))
	resp := gqlRequest(t, srv, ctx, myLearningStatsQuery)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL for a generic loader error, got %q", code)
	}
}

// TestMyLearningStats_Unauthenticated verifies that a StatsUsecase returning
// ucerr.ErrUnauthenticated is translated by gqlerr.FromUsecaseError to the
// UNAUTHENTICATED wire code, before toLearningStatsModel (and its loader
// dependency) is ever invoked.
func TestMyLearningStats_Unauthenticated(t *testing.T) {
	t.Parallel()

	mock := &mockStatsUsecase{err: ucerr.ErrUnauthenticated}
	srv := newStatsSrv(mock)

	resp := gqlRequest(t, srv, context.Background(), myLearningStatsQuery)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// ctxWithCardLoader installs an in-memory Card loader (in addition to a
// Cardgroup loader) for the diagnostic-half myLearningStats tests. A missing
// card_id returns loader.ErrNotFound, matching production behaviour.
func ctxWithCardLoader(base context.Context, cards map[string]*domain.Card) context.Context {
	loaders := &loader.Loaders{
		Card: dataloader.NewBatchedLoader(
			func(_ context.Context, keys []string) []*dataloader.Result[*domain.Card] {
				out := make([]*dataloader.Result[*domain.Card], len(keys))
				for i, k := range keys {
					if card, ok := cards[k]; ok {
						out[i] = &dataloader.Result[*domain.Card]{Data: card}
					} else {
						out[i] = &dataloader.Result[*domain.Card]{Error: loader.ErrNotFound}
					}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

// ctxWithCardLoaderError installs a Card loader whose batch function fails every
// key with loadErr.
func ctxWithCardLoaderError(base context.Context, loadErr error) context.Context {
	loaders := &loader.Loaders{
		Card: dataloader.NewBatchedLoader(
			func(_ context.Context, keys []string) []*dataloader.Result[*domain.Card] {
				out := make([]*dataloader.Result[*domain.Card], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*domain.Card]{Error: loadErr}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

const myLearningStatsDiagnosticQuery = `{"query":"{ myLearningStats { mastery { totalStudied } performance { retentionRate successRate lapseRate studyStreak reviewCount avgDifficulty } strugglingCards { card { id front } lapses stability } } }"}`

// TestMyLearningStats_PerformanceAndStrugglingCards verifies the diagnostic half
// of the response: the performance snapshot maps every metric field, and the
// struggling-card list preserves the usecase order while hydrating each Card
// from its id via the in-memory Card DataLoader.
func TestMyLearningStats_PerformanceAndStrugglingCards(t *testing.T) {
	t.Parallel()

	mock := &mockStatsUsecase{
		result: &usecase.LearningStatsResult{
			Mastery: service.MasteryBreakdown{TotalStudied: 3},
			Performance: service.PerformanceMetrics{
				SuccessRate:   0.8,
				AvgDifficulty: 0.4,
				RetentionRate: 0.9,
				StudyStreak:   5,
				LapseRate:     0.1,
				ReviewCount:   42,
			},
			StrugglingCards: []service.StrugglingCard{
				{CardID: "card-1", Lapses: 5, Stability: 2.5},
				{CardID: "card-2", Lapses: 3, Stability: 8},
			},
		},
	}
	srv := newStatsSrv(mock)

	cards := map[string]*domain.Card{
		"card-1": {ID: "card-1", Front: "alpha", CardgroupID: domain.CardgroupID("cg-1")},
		"card-2": {ID: "card-2", Front: "beta", CardgroupID: domain.CardgroupID("cg-1")},
	}
	ctx := ctxWithCardLoader(authedCtx("u-1"), cards)
	resp := gqlRequest(t, srv, ctx, myLearningStatsDiagnosticQuery)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	stats, _ := data["myLearningStats"].(map[string]any)
	if stats == nil {
		t.Fatalf("expected data.myLearningStats, got nil; response: %v", resp)
	}

	perf, _ := stats["performance"].(map[string]any)
	if perf == nil {
		t.Fatalf("expected performance, got nil; response: %v", resp)
	}
	if perf["retentionRate"] != float64(0.9) || perf["successRate"] != float64(0.8) ||
		perf["lapseRate"] != float64(0.1) || perf["avgDifficulty"] != float64(0.4) ||
		perf["studyStreak"] != float64(5) || perf["reviewCount"] != float64(42) {
		t.Fatalf("performance metrics mismatch: %v", perf)
	}

	struggling, _ := stats["strugglingCards"].([]any)
	if len(struggling) != 2 {
		t.Fatalf("expected 2 struggling cards, got %d; response: %v", len(struggling), resp)
	}

	sc0, _ := struggling[0].(map[string]any)
	if sc0["lapses"] != float64(5) || sc0["stability"] != float64(2.5) {
		t.Fatalf("struggling card 0 mismatch: %v", sc0)
	}
	card0, _ := sc0["card"].(map[string]any)
	if card0 == nil || card0["id"] != "card-1" || card0["front"] != "alpha" {
		t.Fatalf("expected struggling card 0 hydrated {id: card-1, front: alpha}, got %v", card0)
	}

	sc1, _ := struggling[1].(map[string]any)
	if sc1["lapses"] != float64(3) || sc1["stability"] != float64(8) {
		t.Fatalf("struggling card 1 mismatch: %v", sc1)
	}
	card1, _ := sc1["card"].(map[string]any)
	if card1 == nil || card1["id"] != "card-2" || card1["front"] != "beta" {
		t.Fatalf("expected struggling card 1 hydrated {id: card-2, front: beta}, got %v", card1)
	}
}

// TestMyLearningStats_NoLapses_ReturnsEmptyStrugglingCards verifies that a
// learner with no struggling cards serializes strugglingCards as an empty array
// (not null), with no Card DataLoader invoked.
func TestMyLearningStats_NoLapses_ReturnsEmptyStrugglingCards(t *testing.T) {
	t.Parallel()

	mock := &mockStatsUsecase{
		result: &usecase.LearningStatsResult{
			Mastery:         service.MasteryBreakdown{TotalStudied: 2},
			StrugglingCards: []service.StrugglingCard{},
		},
	}
	srv := newStatsSrv(mock)

	ctx := ctxWithCardLoader(authedCtx("u-1"), map[string]*domain.Card{})
	resp := gqlRequest(t, srv, ctx, myLearningStatsDiagnosticQuery)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	stats, _ := data["myLearningStats"].(map[string]any)
	struggling, ok := stats["strugglingCards"].([]any)
	if !ok {
		t.Fatalf("expected strugglingCards array, got %v", stats["strugglingCards"])
	}
	if len(struggling) != 0 {
		t.Fatalf("expected empty strugglingCards, got %d entries", len(struggling))
	}
}

// TestMyLearningStats_StrugglingCardLoadError_ReturnsInternal drives the
// struggling-card Card hydration through a Card DataLoader that fails with a
// generic error; classifyLoaderErr must surface INTERNAL.
func TestMyLearningStats_StrugglingCardLoadError_ReturnsInternal(t *testing.T) {
	t.Parallel()

	mock := &mockStatsUsecase{
		result: &usecase.LearningStatsResult{
			StrugglingCards: []service.StrugglingCard{{CardID: "card-1", Lapses: 2, Stability: 1}},
		},
	}
	srv := newStatsSrv(mock)

	ctx := ctxWithCardLoaderError(authedCtx("u-1"), errors.New("db down"))
	resp := gqlRequest(t, srv, ctx, myLearningStatsDiagnosticQuery)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL for a generic Card loader error, got %q", code)
	}
}
