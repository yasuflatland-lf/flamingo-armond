package resolver

import (
	"context"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

// ---------------------------------------------------------------------------
// AdminCreateMasterCard
// ---------------------------------------------------------------------------

func TestAdminCreateMasterCard_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{createOut: usecase.CreateMasterCardOutcome{
		Card: &domain.MasterCard{ID: "c1", MasterCardgroupID: "m1", Front: domain.CardText("front"), Back: domain.CardText("back")},
	}}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	got, err := mr.AdminCreateMasterCard(context.Background(), model.NewMasterCardInput{MasterCardgroupID: "m1", Front: "front", Back: "back"})
	require.NoError(t, err)

	// Input passes through to the usecase.
	assert.Equal(t, "m1", stub.gotCreateInput.MasterCardgroupID)
	assert.Equal(t, "front", stub.gotCreateInput.Front)
	assert.Equal(t, "back", stub.gotCreateInput.Back)

	succ, ok := got.(model.CreateMasterCardSuccess)
	require.True(t, ok, "want CreateMasterCardSuccess variant, got %T", got)
	require.NotNil(t, succ.MasterCard)
	assert.Equal(t, "c1", succ.MasterCard.ID)
	assert.Equal(t, "front", succ.MasterCard.Front)
}

func TestAdminCreateMasterCard_DuplicateAsData(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{createOut: usecase.CreateMasterCardOutcome{
		Duplicate: &usecase.DuplicateCardInfo{ExistingID: "existing-1", ExistingBack: "old-back"},
	}}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	got, err := mr.AdminCreateMasterCard(context.Background(), model.NewMasterCardInput{MasterCardgroupID: "m1", Front: "f", Back: "b"})
	require.NoError(t, err)

	dup, ok := got.(model.MasterCardDuplicateFrontError)
	require.True(t, ok, "want MasterCardDuplicateFrontError variant, got %T", got)
	assert.Equal(t, "existing-1", dup.ExistingCardID)
	assert.Equal(t, "old-back", dup.ExistingBack)
	assert.NotEmpty(t, dup.Message)
}

func TestAdminCreateMasterCard_WrapsForbidden(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{createErr: ucerr.NewForbiddenError("admin only")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	_, err := mr.AdminCreateMasterCard(context.Background(), model.NewMasterCardInput{MasterCardgroupID: "m1", Front: "f", Back: "b"})
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}

func TestAdminCreateMasterCard_WrapsValidation(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{createErr: ucerr.NewValidationError("front", "front is required")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	_, err := mr.AdminCreateMasterCard(context.Background(), model.NewMasterCardInput{MasterCardgroupID: "m1", Front: "", Back: "b"})
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeBadUserInput), "want BAD_USER_INPUT wire code")
}

func TestAdminCreateMasterCard_NoVariantIsInternal(t *testing.T) {
	t.Parallel()
	// Both Card and Duplicate nil — a programmer error the resolver maps to INTERNAL.
	stub := &stubMasterCardUC{createOut: usecase.CreateMasterCardOutcome{}}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	_, err := mr.AdminCreateMasterCard(context.Background(), model.NewMasterCardInput{MasterCardgroupID: "m1", Front: "f", Back: "b"})
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeInternal), "want INTERNAL wire code")
}

// ---------------------------------------------------------------------------
// AdminUpdateMasterCard
// ---------------------------------------------------------------------------

func TestAdminUpdateMasterCard_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{updateOut: usecase.UpdateMasterCardOutcome{
		Card: &domain.MasterCard{ID: "c1", MasterCardgroupID: "m1", Front: domain.CardText("new-front"), Back: domain.CardText("new-back")},
	}}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	front := "new-front"
	got, err := mr.AdminUpdateMasterCard(context.Background(), "c1", model.UpdateMasterCardInput{Front: &front})
	require.NoError(t, err)
	assert.Equal(t, "c1", stub.gotUpdateID)
	require.NotNil(t, stub.gotUpdateInput.Front)
	assert.Equal(t, "new-front", *stub.gotUpdateInput.Front)

	succ, ok := got.(model.UpdateMasterCardSuccess)
	require.True(t, ok, "want UpdateMasterCardSuccess variant, got %T", got)
	require.NotNil(t, succ.MasterCard)
	assert.Equal(t, "new-front", succ.MasterCard.Front)
}

func TestAdminUpdateMasterCard_ValidationAsData(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{updateOut: usecase.UpdateMasterCardOutcome{
		Validation: usecase.NewInputValidationInfo("front", "front is required"),
	}}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	empty := ""
	got, err := mr.AdminUpdateMasterCard(context.Background(), "c1", model.UpdateMasterCardInput{Front: &empty})
	require.NoError(t, err)

	ve, ok := got.(model.InputValidationError)
	require.True(t, ok, "want InputValidationError variant, got %T", got)
	assert.Equal(t, "front", ve.Field)
	assert.Equal(t, "front is required", ve.Message)
}

func TestAdminUpdateMasterCard_WrapsForbidden(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{updateErr: ucerr.NewForbiddenError("admin only")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	front := "f"
	_, err := mr.AdminUpdateMasterCard(context.Background(), "c1", model.UpdateMasterCardInput{Front: &front})
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}

func TestAdminUpdateMasterCard_WrapsValidation(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{updateErr: ucerr.NewValidationError("id", "master card not found")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	front := "f"
	_, err := mr.AdminUpdateMasterCard(context.Background(), "missing", model.UpdateMasterCardInput{Front: &front})
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeBadUserInput), "want BAD_USER_INPUT wire code")
}

func TestAdminUpdateMasterCard_WrapsInternal(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{updateErr: eris.New("usecase: master card: update: db timeout")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	front := "f"
	_, err := mr.AdminUpdateMasterCard(context.Background(), "id-1", model.UpdateMasterCardInput{Front: &front})
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeInternal), "want INTERNAL wire code")
}

func TestAdminUpdateMasterCard_NoVariantIsInternal(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{updateOut: usecase.UpdateMasterCardOutcome{}}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	front := "f"
	_, err := mr.AdminUpdateMasterCard(context.Background(), "c1", model.UpdateMasterCardInput{Front: &front})
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeInternal), "want INTERNAL wire code")
}

// ---------------------------------------------------------------------------
// AdminDeleteMasterCard / AdminDeleteMasterCards
// ---------------------------------------------------------------------------

func TestAdminDeleteMasterCard_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	ok, err := mr.AdminDeleteMasterCard(context.Background(), "c1")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "c1", stub.gotDeleteID)
}

func TestAdminDeleteMasterCard_WrapsValidation(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{deleteErr: ucerr.NewValidationError("id", "master card not found")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	got, err := mr.AdminDeleteMasterCard(context.Background(), "missing")
	require.Error(t, err)
	assert.False(t, got)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeBadUserInput), "want BAD_USER_INPUT wire code")
}

func TestAdminDeleteMasterCard_WrapsForbidden(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{deleteErr: ucerr.NewForbiddenError("admin only")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	got, err := mr.AdminDeleteMasterCard(context.Background(), "id-1")
	require.Error(t, err)
	assert.False(t, got)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}

func TestAdminDeleteMasterCard_WrapsInternal(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{deleteErr: eris.New("usecase: master card: delete: db timeout")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	got, err := mr.AdminDeleteMasterCard(context.Background(), "id-1")
	require.Error(t, err)
	assert.False(t, got)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeInternal), "want INTERNAL wire code")
}

func TestAdminDeleteMasterCards_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{bulkCount: 3}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	n, err := mr.AdminDeleteMasterCards(context.Background(), []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, []string{"a", "b", "c"}, stub.gotBulkIDs)
}

func TestAdminDeleteMasterCards_WrapsForbidden(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{bulkDeleteErr: ucerr.NewForbiddenError("admin only")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	n, err := mr.AdminDeleteMasterCards(context.Background(), []string{"a"})
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}

// ---------------------------------------------------------------------------
// AdminImportMasterCards
// ---------------------------------------------------------------------------

func TestAdminImportMasterCards_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{importOut: usecase.ImportMasterCardsOutput{
		Inserted: 2,
		Updated:  1,
		Errors: []usecase.CardImportError{
			{Line: 3, Message: "lone front", Kind: usecase.CardImportErrKindFrontOnly, Snippet: "Lone"},
		},
	}}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	payload, err := mr.AdminImportMasterCards(context.Background(), model.ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: "cGF5bG9hZA=="})
	require.NoError(t, err)
	require.NotNil(t, payload)
	assert.Equal(t, "m1", stub.gotImportInput.MasterCardgroupID)
	assert.Equal(t, "cGF5bG9hZA==", stub.gotImportInput.Payload)
	assert.Equal(t, 2, payload.Inserted)
	assert.Equal(t, 1, payload.Updated)
	require.Len(t, payload.Errors, 1)
	assert.Equal(t, 3, payload.Errors[0].Line)
	assert.Equal(t, model.CardImportErrorKindFrontOnly, payload.Errors[0].Kind)
}

func TestAdminImportMasterCards_WrapsForbidden(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{importErr: ucerr.NewForbiddenError("admin only")}
	mr := &mutationResolver{&Resolver{MasterCardUC: stub}}

	_, err := mr.AdminImportMasterCards(context.Background(), model.ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: "x"})
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}
