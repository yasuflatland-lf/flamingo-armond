package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/gqlerr/gqlerrtest"
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

// TestMergeMasterCardgroup_UsecaseError_Wrapped verifies the mandatory
// gqlerr.FromUsecaseError wrap: typed usecase errors surface with the correct
// wire code rather than escaping uncoded — mirrors
// TestQueryResolver_MasterCatalog_WrapsUsecaseError.
func TestMergeMasterCardgroup_UsecaseError_Wrapped(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want gqlerr.Code
	}{
		{"unauthenticated", ucerr.ErrUnauthenticated, gqlerr.CodeUnauthenticated},
		{"validation", ucerr.NewValidationError("cardgroupId", "cardgroup not found"), gqlerr.CodeBadUserInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubMasterCatalogUC{mergeErr: tc.err}
			r := &Resolver{MasterCatalogUC: stub}

			res, err := r.Mutation().MergeMasterCardgroup(context.Background(), model.MergeMasterCardgroupInput{
				MasterCardgroupID: "master-id",
				CardgroupID:       "cg-id",
			})
			if res != nil {
				t.Fatalf("expected nil union on error, got %T", res)
			}
			require.Error(t, err)
			assert.True(t, gqlerrtest.IsCode(err, tc.want), "want wire code %s", tc.want)
		})
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
	if !gqlerrtest.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL wire error, got %v", err)
	}
}

func TestMergeMasterCardgroupPreview_Success(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{
		previewOut: usecase.PreviewMergeOutcome{Added: 7, Updated: 2},
	}
	r := &Resolver{MasterCatalogUC: stub}

	got, err := r.Query().MergeMasterCardgroupPreview(context.Background(), model.MergeMasterCardgroupInput{
		MasterCardgroupID: "master-id",
		CardgroupID:       "cg-id",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pv, ok := got.(model.MergeMasterCardgroupPreview)
	if !ok {
		t.Fatalf("expected MergeMasterCardgroupPreview, got %T", got)
	}
	if pv.AddedCount != 7 || pv.UpdatedCount != 2 {
		t.Fatalf("expected 7/2, got %d/%d", pv.AddedCount, pv.UpdatedCount)
	}
}

func TestMergeMasterCardgroupPreview_NotFound(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{previewOut: usecase.PreviewMergeOutcome{NotFound: true}}
	r := &Resolver{MasterCatalogUC: stub}

	got, err := r.Query().MergeMasterCardgroupPreview(context.Background(), model.MergeMasterCardgroupInput{
		MasterCardgroupID: "missing",
		CardgroupID:       "cg-id",
	})
	if err != nil {
		t.Fatalf("not-found must be data, not error; got %v", err)
	}
	if _, ok := got.(model.MasterNotFoundError); !ok {
		t.Fatalf("expected MasterNotFoundError, got %T", got)
	}
}
