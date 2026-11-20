package repository_test

// Concurrency integration tests for the per-owner cardgroup quota. They run
// against the testcontainer Postgres; TestMain, testDB, insertAuthUser,
// seedMasterDeck, countCardgroupsForOwner and stubParityAdminChecker are defined
// in sibling files and shared across this package.

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase"
)

// seedOwnerAtOneBelowQuota creates an owner holding GeneralUserCardgroupLimit-1
// cardgroups and returns the owner id plus an authenticated context for them.
func seedOwnerAtOneBelowQuota(t *testing.T) (string, context.Context) {
	t.Helper()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cgRepo := repository.NewCardgroupRepository(testDB.GORM)
	now := time.Now().UTC()
	for i := 0; i < domain.GeneralUserCardgroupLimit-1; i++ {
		cg, err := domain.NewCardgroup(domain.UserID(ownerID), domain.CardgroupName(fmt.Sprintf("Existing %d", i)), now)
		require.NoError(t, err)
		require.NoError(t, cgRepo.Create(ctx, cg))
	}
	return ownerID, auth.ContextWithUser(ctx, &auth.AuthUser{Sub: ownerID})
}

func TestCardgroupCreate_Integration_ConcurrentCreatesRespectQuota(t *testing.T) {
	t.Parallel()
	ownerID, authedCtx := seedOwnerAtOneBelowQuota(t)
	ctx, cancel := context.WithTimeout(authedCtx, 15*time.Second)
	defer cancel()
	cgRepo := repository.NewCardgroupRepository(testDB.GORM)
	uc := usecase.NewCardgroupUsecase(testDB.GORM, cgRepo, stubParityAdminChecker{})

	const workers = 8
	outcomes := make([]usecase.CreateCardgroupOutcome, workers)
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			outcomes[i], errs[i] = uc.Create(ctx, usecase.CreateCardgroupInput{Name: fmt.Sprintf("Race %d", i)})
		}(i)
	}
	close(start)
	wg.Wait()

	created, limited := 0, 0
	for i := 0; i < workers; i++ {
		require.NoError(t, errs[i])
		if outcomes[i].Cardgroup != nil {
			created++
		}
		if outcomes[i].LimitReached != nil {
			limited++
		}
	}
	require.Equal(t, 1, created, "exactly one concurrent create fits under the quota")
	require.Equal(t, workers-1, limited)
	require.Equal(t, domain.GeneralUserCardgroupLimit, countCardgroupsForOwner(t, ctx, ownerID))
}

func TestCardgroupQuota_Integration_ConcurrentCreateAndImportRespectQuota(t *testing.T) {
	t.Parallel()
	ownerID, authedCtx := seedOwnerAtOneBelowQuota(t)
	ctx, cancel := context.WithTimeout(authedCtx, 15*time.Second)
	defer cancel()
	cgRepo := repository.NewCardgroupRepository(testDB.GORM)
	cgUC := usecase.NewCardgroupUsecase(testDB.GORM, cgRepo, stubParityAdminChecker{})
	catalogUC := newMasterCatalogUsecaseForParityTest(t)
	masterID := seedMasterDeck(t, context.Background(), "Quota Race Master "+uuid.NewString(), []*domain.MasterCard{
		masterCardFixture("front", "back", 0),
	})

	const perKind = 4
	succeeded := make([]bool, 2*perKind)
	errs := make([]error, 2*perKind)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2 * perKind)
	for i := 0; i < perKind; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			out, err := cgUC.Create(ctx, usecase.CreateCardgroupInput{Name: fmt.Sprintf("Race %d", i)})
			succeeded[i], errs[i] = out.Cardgroup != nil, err
		}(i)
		go func(i int) {
			defer wg.Done()
			<-start
			out, err := catalogUC.ImportMaster(ctx, masterID)
			succeeded[perKind+i], errs[perKind+i] = out.Cardgroup != nil, err
		}(i)
	}
	close(start)
	wg.Wait()

	total := 0
	for i := range errs {
		require.NoError(t, errs[i])
		if succeeded[i] {
			total++
		}
	}
	require.Equal(t, 1, total, "exactly one concurrent create or import fits under the quota")
	require.Equal(t, domain.GeneralUserCardgroupLimit, countCardgroupsForOwner(t, ctx, ownerID))
}

func TestCardgroupQuota_Integration_ConcurrentImportsRespectQuota(t *testing.T) {
	t.Parallel()
	ownerID, authedCtx := seedOwnerAtOneBelowQuota(t)
	ctx, cancel := context.WithTimeout(authedCtx, 15*time.Second)
	defer cancel()
	catalogUC := newMasterCatalogUsecaseForParityTest(t)
	masterID := seedMasterDeck(t, ctx, "Quota Import Race Master "+uuid.NewString(), []*domain.MasterCard{
		masterCardFixture("front", "back", 0),
	})

	const workers = 8
	outcomes := make([]usecase.ImportMasterOutcome, workers)
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			outcomes[i], errs[i] = catalogUC.ImportMaster(ctx, masterID)
		}(i)
	}
	close(start)
	wg.Wait()

	imported, limited := 0, 0
	for i := 0; i < workers; i++ {
		require.NoError(t, errs[i])
		if outcomes[i].Cardgroup != nil {
			imported++
		}
		if outcomes[i].LimitReached != nil {
			limited++
		}
	}
	require.Equal(t, 1, imported, "exactly one concurrent import fits under the quota")
	require.Equal(t, workers-1, limited)
	require.Equal(t, domain.GeneralUserCardgroupLimit, countCardgroupsForOwner(t, ctx, ownerID))
}
