package resolver

import (
	"context"
	"testing"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

func TestMergeMasterCardgroup_Success(t *testing.T) {
	t.Parallel()

	cg := &domain.Cardgroup{ID: domain.CardgroupID("cg-id"), Name: domain.CardgroupName("My Deck"), OwnerID: "owner"}
	stub := &stubMasterCatalogUC{
		mergeOut: usecase.MergeMasterOutcome{Cardgroup: cg, Added: 5, Updated: 2},
	}
	r := &Resolver{MasterCatalogUC: stub}

	got, err := r.Mutation().MergeMasterCardgroup(context.Background(), model.MergeMasterCardgroupInput{
		MasterCardgroupID: "master-id",
		CardgroupID:       "cg-id",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	succ, ok := got.(model.MergeMasterCardgroupSuccess)
	if !ok {
		t.Fatalf("expected MergeMasterCardgroupSuccess, got %T", got)
	}
	if succ.AddedCount != 5 {
		t.Fatalf("expected AddedCount 5, got %d", succ.AddedCount)
	}
	if succ.UpdatedCount != 2 {
		t.Fatalf("expected UpdatedCount 2, got %d", succ.UpdatedCount)
	}
	if succ.Cardgroup == nil || succ.Cardgroup.ID != "cg-id" {
		t.Fatalf("expected mapped cardgroup cg-id, got %+v", succ.Cardgroup)
	}
}

func TestMergeMasterCardgroup_NotFound_AsData(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{mergeOut: usecase.MergeMasterOutcome{NotFound: true}}
	r := &Resolver{MasterCatalogUC: stub}

	got, err := r.Mutation().MergeMasterCardgroup(context.Background(), model.MergeMasterCardgroupInput{
		MasterCardgroupID: "master-id",
		CardgroupID:       "cg-id",
	})
	if err != nil {
		t.Fatalf("not-found must be data, not error; got %v", err)
	}
	notFound, ok := got.(model.MasterNotFoundError)
	if !ok {
		t.Fatalf("expected MasterNotFoundError, got %T", got)
	}
	if notFound.Message == "" {
		t.Fatal("expected a non-empty message")
	}
}

func TestMergeMasterCardgroup_UsecaseError_Wrapped(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{mergeErr: ucerr.ErrUnauthenticated}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().MergeMasterCardgroup(context.Background(), model.MergeMasterCardgroupInput{
		MasterCardgroupID: "master-id",
		CardgroupID:       "cg-id",
	})
	if res != nil {
		t.Fatalf("expected nil union on error, got %T", res)
	}
	if !gqlerr.IsCode(err, gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected UNAUTHENTICATED wire error, got %v", err)
	}
}

func TestMergeMasterCardgroup_XORInvariantViolation(t *testing.T) {
	t.Parallel()

	// A degenerate {Cardgroup:nil, NotFound:false} outcome is a producer-contract
	// violation; the resolver's defensive guard must surface it as INTERNAL.
	stub := &stubMasterCatalogUC{mergeOut: usecase.MergeMasterOutcome{}}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().MergeMasterCardgroup(context.Background(), model.MergeMasterCardgroupInput{
		MasterCardgroupID: "master-id",
		CardgroupID:       "cg-id",
	})
	if res != nil {
		t.Fatalf("expected nil union on invariant violation, got %T", res)
	}
	if !gqlerr.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL wire error, got %v", err)
	}
}
