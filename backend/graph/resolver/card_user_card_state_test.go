package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/graph/model"
	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/loader"
	"backend/internal/usecase"
)

// spyLearnUsecase records the DefaultIfNew call so the resolver test can assert
// the resolver delegates the new-card default decision rather than constructing
// the domain entity itself. NextDueCards / PracticeTodaysCards are not exercised
// by UserCardState; they panic to surface an unexpected call.
type spyLearnUsecase struct {
	gotUCS       *domain.UserCardFSRS
	gotUserID    domain.UserID
	gotCardID    string
	gotCreatedAt time.Time
	calls        int
	ret          *domain.UserCardFSRS
}

func (s *spyLearnUsecase) NextDueCards(context.Context, string, *int) ([]*domain.Card, error) {
	panic("spyLearnUsecase.NextDueCards must not be called by UserCardState")
}

func (s *spyLearnUsecase) PracticeTodaysCards(context.Context, string, *int) ([]*domain.Card, error) {
	panic("spyLearnUsecase.PracticeTodaysCards must not be called by UserCardState")
}

func (s *spyLearnUsecase) DefaultIfNew(ucs *domain.UserCardFSRS, userID domain.UserID, cardID string, createdAt time.Time) *domain.UserCardFSRS {
	s.calls++
	s.gotUCS = ucs
	s.gotUserID = userID
	s.gotCardID = cardID
	s.gotCreatedAt = createdAt
	return s.ret
}

var _ usecase.LearnUsecase = (*spyLearnUsecase)(nil)

// emptyUserCardFSRSReader satisfies the loader's narrow userCardFSRSReader by
// returning an empty map, so the batched Load resolves to nil for every key —
// the DataLoader's documented "card the user has never seen" contract.
type emptyUserCardFSRSReader struct{}

func (emptyUserCardFSRSReader) FindByUserAndCardIDs(context.Context, string, []string) (map[string]*domain.UserCardFSRS, error) {
	return map[string]*domain.UserCardFSRS{}, nil
}

// TestCardResolver_UserCardState_DelegatesDefaultDecision proves the resolver no
// longer authors the new-card FSRS default itself: the batched loader yields nil
// (unseen card), and the resolver hands that nil to LearnUC.DefaultIfNew with the
// viewer/card/createdAt, returning whatever the usecase decides. The "missing
// record means default state" policy lives in the application layer.
func TestCardResolver_UserCardState_DelegatesDefaultDecision(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	// The sentinel the spy returns; assert it round-trips through the mapper so
	// the resolver uses the usecase's decision verbatim.
	decided := domain.NewUserCardFSRSForNewCard(domain.UserID("viewer-1"), "card-1", createdAt)
	spy := &spyLearnUsecase{ret: decided}

	r := &Resolver{LearnUC: spy}
	cr := &cardResolver{r}

	loaders := loader.NewWithUserCardFSRS(
		nil, nil, nil, nil, nil, nil, nil,
		emptyUserCardFSRSReader{},
		"viewer-1",
	)
	ctx := loader.WithContext(context.Background(), loaders)
	ctx = auth.ContextWithUser(ctx, &auth.AuthUser{Sub: "viewer-1"})

	obj := &model.Card{ID: "card-1", CreatedAt: createdAt}
	out, err := cr.UserCardState(ctx, obj)
	require.NoError(t, err)
	require.NotNil(t, out)

	// The resolver delegated exactly once, passing the loader's nil and the
	// viewer/card/createdAt tuple — it did not construct the entity itself.
	require.Equal(t, 1, spy.calls)
	require.Nil(t, spy.gotUCS, "loader yields nil for an unseen card; resolver must forward it, not pre-default it")
	require.Equal(t, domain.UserID("viewer-1"), spy.gotUserID)
	require.Equal(t, "card-1", spy.gotCardID)
	require.True(t, spy.gotCreatedAt.Equal(createdAt))

	// The mapped output reflects the usecase's decision, not a resolver-authored value.
	require.Equal(t, decided.State.Due, out.Due)
}

// TestCardResolver_UserCardState_PassThroughExistingState verifies the resolver
// also delegates when the loader returns a non-nil record: the usecase receives
// the existing state and returns it unchanged.
func TestCardResolver_UserCardState_PassThroughExistingState(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	existing := domain.NewUserCardFSRSForNewCard(domain.UserID("viewer-1"), "card-1", createdAt.Add(-time.Hour))
	spy := &spyLearnUsecase{ret: existing}

	r := &Resolver{LearnUC: spy}
	cr := &cardResolver{r}

	loaders := loader.NewWithUserCardFSRS(
		nil, nil, nil, nil, nil, nil, nil,
		seededUserCardFSRSReader{state: existing},
		"viewer-1",
	)
	ctx := loader.WithContext(context.Background(), loaders)
	ctx = auth.ContextWithUser(ctx, &auth.AuthUser{Sub: "viewer-1"})

	obj := &model.Card{ID: "card-1", CreatedAt: createdAt}
	out, err := cr.UserCardState(ctx, obj)
	require.NoError(t, err)
	require.NotNil(t, out)

	require.Equal(t, 1, spy.calls)
	require.Same(t, existing, spy.gotUCS, "loader's non-nil record must be forwarded to the usecase")
}

// seededUserCardFSRSReader returns a single stored record keyed by card id.
type seededUserCardFSRSReader struct{ state *domain.UserCardFSRS }

func (s seededUserCardFSRSReader) FindByUserAndCardIDs(_ context.Context, _ string, cardIDs []string) (map[string]*domain.UserCardFSRS, error) {
	out := map[string]*domain.UserCardFSRS{}
	for _, id := range cardIDs {
		if id == s.state.CardID {
			out[id] = s.state
		}
	}
	return out, nil
}
