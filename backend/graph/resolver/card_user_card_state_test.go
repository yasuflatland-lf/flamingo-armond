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
)

// emptyUserCardFSRSReader satisfies the loader's narrow userCardFSRSReader by
// returning an empty map, so the batched Load resolves to nil for every key —
// the DataLoader's documented "card the user has never seen" contract.
type emptyUserCardFSRSReader struct{}

func (emptyUserCardFSRSReader) FindByUserAndCardIDs(context.Context, string, []string) (map[string]*domain.UserCardFSRS, error) {
	return map[string]*domain.UserCardFSRS{}, nil
}

// TestCardResolver_UserCardState_UnseenCard verifies that a missing loader row
// renders the aggregate's default new-card state.
func TestCardResolver_UserCardState_UnseenCard(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	cr := &cardResolver{&Resolver{}}

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

	want := domain.NewUserCardFSRSForNewCard(domain.UserID("viewer-1"), "card-1", createdAt)
	require.Equal(t, toUserCardStateModel(want), out)
}

// TestCardResolver_UserCardState_PassThroughExistingState verifies the resolver
// renders the existing scheduling record returned by the loader.
func TestCardResolver_UserCardState_PassThroughExistingState(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	existing := domain.NewUserCardFSRSForNewCard(domain.UserID("viewer-1"), "card-1", createdAt.Add(-time.Hour))
	cr := &cardResolver{&Resolver{}}

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

	require.Equal(t, toUserCardStateModel(existing), out)
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
