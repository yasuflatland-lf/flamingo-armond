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

// ctxWithCardgroupLoader installs an in-memory Cardgroup loader for
// MyLearningStats resolver tests. A missing cardgroup_id returns
// loader.ErrNotFound, matching production behaviour. Other Loaders fields stay
// nil because toLearningStatsModel only reads loaders.Cardgroup.
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
			Mastery: usecase.MasteryBreakdown{InProgress: 4, Learned: 6, Mature: 10, TotalStudied: 20},
			Decks: []usecase.DeckMasteryResult{
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
			Mastery: usecase.MasteryBreakdown{TotalStudied: 0},
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
			Decks: []usecase.DeckMasteryResult{{CardgroupID: "cg-1", TotalCards: 1}},
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
			Decks: []usecase.DeckMasteryResult{{CardgroupID: "cg-1", TotalCards: 1}},
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
