package resolver

import (
	"context"
	"testing"

	"github.com/rotisserie/eris"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

func TestMutationResolver_ImportMasterCardgroup_Success(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{
		importOut: usecase.ImportMasterOutcome{
			Cardgroup: &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "u1", Name: domain.CardgroupName("Deck")},
		},
	}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().ImportMasterCardgroup(context.Background(), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotImport != "m1" {
		t.Fatalf("usecase called with %q, want m1", stub.gotImport)
	}
	ok, isOK := res.(model.ImportMasterCardgroupSuccess)
	if !isOK {
		t.Fatalf("expected ImportMasterCardgroupSuccess, got %T", res)
	}
	if ok.Cardgroup == nil || ok.Cardgroup.ID != "cg1" {
		t.Fatalf("expected mapped cardgroup cg1, got %+v", ok.Cardgroup)
	}
}

func TestMutationResolver_ImportMasterCardgroup_NotFound_ReturnsTypedError(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{importOut: usecase.ImportMasterOutcome{NotFound: true}}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().ImportMasterCardgroup(context.Background(), "missing")
	if err != nil {
		t.Fatalf("not-found must be data, not error; got %v", err)
	}
	notFound, ok := res.(model.MasterNotFoundError)
	if !ok {
		t.Fatalf("expected MasterNotFoundError, got %T", res)
	}
	if notFound.Message == "" {
		t.Fatal("expected a non-empty message")
	}
}

func TestMutationResolver_ImportMasterCardgroup_Unauthenticated_WrapsError(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{importErr: ucerr.ErrUnauthenticated}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().ImportMasterCardgroup(context.Background(), "m1")
	if res != nil {
		t.Fatalf("expected nil union on error, got %T", res)
	}
	if !gqlerr.IsCode(err, gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected UNAUTHENTICATED wire error, got %v", err)
	}
}

func TestMutationResolver_ImportMasterCardgroup_InfraError_WrapsInternal(t *testing.T) {
	t.Parallel()

	// A non-auth usecase error (e.g. a DB outage surfaced by ImportMaster) must
	// transit gqlerr.FromUsecaseError to the INTERNAL wire code rather than
	// escaping uncoded — mirrors TestQueryResolver_MasterCatalog_WrapsUsecaseError.
	stub := &stubMasterCatalogUC{importErr: eris.New("db: connection reset")}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().ImportMasterCardgroup(context.Background(), "m1")
	if res != nil {
		t.Fatalf("expected nil union on error, got %T", res)
	}
	if !gqlerr.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL wire error, got %v", err)
	}
}

func TestMutationResolver_ImportMasterCardgroup_XORInvariantViolation(t *testing.T) {
	t.Parallel()

	// A degenerate {Cardgroup:nil, NotFound:false} outcome is a producer-contract
	// violation; the resolver's defensive guard must surface it as INTERNAL rather
	// than a nil union or a schema-null. Mirrors the sibling _XORInvariantViolation
	// tests on the admin role/user outcome-union resolvers.
	stub := &stubMasterCatalogUC{importOut: usecase.ImportMasterOutcome{}}
	r := &Resolver{MasterCatalogUC: stub}

	res, err := r.Mutation().ImportMasterCardgroup(context.Background(), "m1")
	if res != nil {
		t.Fatalf("expected nil union on invariant violation, got %T", res)
	}
	if !gqlerr.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL wire error, got %v", err)
	}
}
