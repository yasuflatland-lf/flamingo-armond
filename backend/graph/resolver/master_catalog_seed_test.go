package resolver

import (
	"context"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/usecase/ucerr"
)

// TestMutationResolver_SeedDefaultStarterCardgroups_Success verifies that the
// resolver maps seeded cardgroups to the wire payload and returns no error.
func TestMutationResolver_SeedDefaultStarterCardgroups_Success(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{
		seedOut: []*domain.Cardgroup{
			{ID: domain.CardgroupID("cg1"), OwnerID: "u1", Name: domain.CardgroupName("Deck A")},
			{ID: domain.CardgroupID("cg2"), OwnerID: "u1", Name: domain.CardgroupName("Deck B")},
		},
	}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().SeedDefaultStarterCardgroups(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Cardgroups, 2)
	assert.Equal(t, "cg1", res.Cardgroups[0].ID)
	assert.Equal(t, "cg2", res.Cardgroups[1].ID)
}

// TestMutationResolver_SeedDefaultStarterCardgroups_EmptySeed verifies that an
// already-seeded caller (idempotent no-op) returns an empty slice rather than an
// error — this is the normal path when the user already owns a cardgroup.
func TestMutationResolver_SeedDefaultStarterCardgroups_EmptySeed(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{seedOut: []*domain.Cardgroup{}}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().SeedDefaultStarterCardgroups(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Empty(t, res.Cardgroups)
}

// TestMutationResolver_SeedDefaultStarterCardgroups_Unauthenticated verifies that
// ucerr.ErrUnauthenticated from the usecase surfaces as UNAUTHENTICATED on the wire.
func TestMutationResolver_SeedDefaultStarterCardgroups_Unauthenticated(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{seedErr: ucerr.ErrUnauthenticated}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().SeedDefaultStarterCardgroups(context.Background())
	assert.Nil(t, res)
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeUnauthenticated), "want UNAUTHENTICATED wire code, got %v", err)
}

// TestMutationResolver_SeedDefaultStarterCardgroups_InfraError verifies that a
// non-auth usecase error (e.g. a DB failure) surfaces as INTERNAL on the wire.
func TestMutationResolver_SeedDefaultStarterCardgroups_InfraError(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{seedErr: eris.New("db: connection reset")}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().SeedDefaultStarterCardgroups(context.Background())
	assert.Nil(t, res)
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeInternal), "want INTERNAL wire code, got %v", err)
}
