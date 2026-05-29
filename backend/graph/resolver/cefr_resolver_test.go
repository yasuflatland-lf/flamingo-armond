package resolver_test

import (
	"context"
	"testing"

	"backend/graph/model"
	"backend/graph/resolver"
	"backend/internal/cefr"
	"backend/internal/domain/service"
	"backend/internal/usecase"

	"github.com/stretchr/testify/require"
)

// newCEFRResolver wires the real embedded word list through the full chain.
func newCEFRResolver(t *testing.T) *resolver.Resolver {
	t.Helper()
	classifier := service.NewCEFRClassifier(cefr.NewWordList())
	cefrUC := usecase.NewCEFRUsecase(classifier)
	return resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, cefrUC)
}

func TestCardResolver_CefrLevel_KnownWord(t *testing.T) {
	t.Parallel()
	r := newCEFRResolver(t)
	lvl, err := r.Card().CefrLevel(context.Background(), &model.Card{Front: "water"})
	require.NoError(t, err)
	require.NotNil(t, lvl)
	require.Equal(t, model.CEFRLevelB1, *lvl) // "water" is B1 in oxford-3000
}

func TestCardResolver_CefrLevel_EmptyFront(t *testing.T) {
	t.Parallel()
	r := newCEFRResolver(t)
	lvl, err := r.Card().CefrLevel(context.Background(), &model.Card{Front: ""})
	require.NoError(t, err)
	require.Nil(t, lvl)
}

func TestCardResolver_CefrLevel_OffList(t *testing.T) {
	t.Parallel()
	r := newCEFRResolver(t)
	lvl, err := r.Card().CefrLevel(context.Background(), &model.Card{Front: "zzqxnotaword"})
	require.NoError(t, err)
	require.Nil(t, lvl)
}
