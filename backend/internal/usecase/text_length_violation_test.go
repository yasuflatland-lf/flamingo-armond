package usecase

// White-box tests for translateTextLengthViolation, the usecase-side half of the
// 23514 CHECK backstop. The function is unexported so the tests live in the same
// package; no DB is required because the repository error is constructed
// directly.

import (
	"context"
	"errors"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

func TestTranslateTextLengthViolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantField string
	}{
		{
			name:      "front violation becomes a validation error on front",
			err:       &repository.TextLengthViolationError{Constraint: "cards_front_length", Field: "front"},
			wantField: "front",
		},
		{
			name:      "back violation becomes a validation error on back",
			err:       &repository.TextLengthViolationError{Constraint: "cards_back_length", Field: "back"},
			wantField: "back",
		},
		{
			name:      "name violation becomes a validation error on name",
			err:       &repository.TextLengthViolationError{Constraint: "master_cardgroups_name_length", Field: "name"},
			wantField: "name",
		},
		{
			name:      "eris-wrapped violation is still translated",
			err:       eris.Wrap(&repository.TextLengthViolationError{Constraint: "cards_back_length", Field: "back"}, "repository: card: create"),
			wantField: "back",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := translateTextLengthViolation(tt.err)
			require.NotNil(t, got)

			var ve *ucerr.ValidationError
			require.True(t, errors.As(got, &ve))
			assert.Equal(t, tt.wantField, ve.Field)
			assert.Contains(t, ve.Message, "too long")
		})
	}
}

// TestTranslateTextLengthViolation_PassesThroughUnrelatedErrors pins the
// pre-filter contract: a nil return means the caller falls through to its own
// eris.Wrap, so an unrelated infrastructure failure keeps classifying as
// INTERNAL rather than being mislabelled as bad user input.
func TestTranslateTextLengthViolation_PassesThroughUnrelatedErrors(t *testing.T) {
	t.Parallel()

	assert.Nil(t, translateTextLengthViolation(errors.New("connection reset")))
	assert.Nil(t, translateTextLengthViolation(repository.ErrNotFound))
	assert.Nil(t, translateTextLengthViolation(repository.ErrCardDuplicateFront))
}

// TestCardgroupUsecase_Create_TextLengthViolation_SurfacesAsValidationOutcome
// pins the deck-name half of the wiring: the repository classifies the 23514
// CHECK violation and the usecase must lift it into the outcome's Validation
// variant (BAD_USER_INPUT on "name"), not wrap it into an internal error.
func TestCardgroupUsecase_Create_TextLengthViolation_SurfacesAsValidationOutcome(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{
		createErr: eris.Wrap(
			&repository.TextLengthViolationError{Constraint: "cardgroups_name_length", Field: "name"},
			"repository: cardgroup: create",
		),
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "My Group"})

	require.NoError(t, err)
	require.Nil(t, outcome.Cardgroup)
	require.NotNil(t, outcome.Validation)
	assert.Equal(t, "name", outcome.Validation.Field)
	assert.Contains(t, outcome.Validation.Message, "too long")
}

// TestCopyMasterToUser_TextLengthViolation_BecomesValidationError covers the
// master-deck copy path, which reaches cardgroupRepo.CreateTx (the deck name is
// copied verbatim from the master) and cardRepo.UpsertManyTx (the card bodies).
// Both classified errors must survive the tx-level wrap as BAD_USER_INPUT.
func TestCopyMasterToUser_TextLengthViolation_BecomesValidationError(t *testing.T) {
	t.Parallel()

	const masterID = "m-textlen"
	newDeps := func() (*fakeMasterCGRepo, *fakeMasterCardRepo) {
		return &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{
				masterID: masterCG(masterID, "Deck"),
			}}, &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
				masterID: {masterCard("mc1", masterID, "f1", "b1", 0)},
			}}
	}

	t.Run("cardgroup name violation from CreateTx", func(t *testing.T) {
		t.Parallel()
		cg, card := newDeps()
		userCG := &fakeUserCG{createErr: &repository.TextLengthViolationError{
			Constraint: "cardgroups_name_length",
			Field:      "name",
		}}
		uc, _, _ := newSeedUsecase(t, cg, card, &fakeUserCardRepo{}, userCG)

		got, err := uc.CopyMasterToUser(context.Background(), masterID, "owner-textlen")
		assert.Nil(t, got)
		assertValidationError(t, err, "name", "name is too long")
	})

	t.Run("card body violation from UpsertManyTx", func(t *testing.T) {
		t.Parallel()
		cg, card := newDeps()
		user := &fakeUserCardRepo{upsertErr: &repository.TextLengthViolationError{
			Constraint: "cards_back_length",
			Field:      "back",
		}}
		uc, _, _ := newSeedUsecase(t, cg, card, user, &fakeUserCG{})

		got, err := uc.CopyMasterToUser(context.Background(), masterID, "owner-textlen-2")
		assert.Nil(t, got)
		assertValidationError(t, err, "back", "back is too long")
	})
}

// TestCardImportUsecase_TextLengthViolation_BecomesValidationError pins the bulk
// half: cardRepo.UpsertManyTx is the batch-import writer, and its classified
// 23514 error must reach the caller as BAD_USER_INPUT on the offending column
// rather than being swallowed by the tx-level eris.Wrap into an internal error.
func TestCardImportUsecase_TextLengthViolation_BecomesValidationError(t *testing.T) {
	t.Parallel()

	payload := buildPayload(t, [][2]string{{"apple", jpRunes(3)}})
	repo := &mockDictCardRepo{returnErr: &repository.TextLengthViolationError{
		Constraint: "cards_back_length",
		Field:      "back",
	}}
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})

	assertValidationError(t, err, "back", "back is too long")
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertManyTx call, got %d", repo.upsertCalls)
	}
}

// TestCardgroupUsecase_Update_TextLengthViolation_SurfacesAsValidationOutcome is
// the rename sibling: the GORM Updates statement wraps a distinct error branch,
// so it needs its own assertion.
func TestCardgroupUsecase_Update_TextLengthViolation_SurfacesAsValidationOutcome(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Original"},
		updateErr: eris.Wrap(
			&repository.TextLengthViolationError{Constraint: "cardgroups_name_length", Field: "name"},
			"repository: cardgroup: update",
		),
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Update(cgAuthedCtx("user-1"), "cg1", UpdateCardgroupInput{Name: ptr("New Name")})

	require.NoError(t, err)
	require.Nil(t, outcome.Cardgroup)
	require.NotNil(t, outcome.Validation)
	assert.Equal(t, "name", outcome.Validation.Field)
	assert.Contains(t, outcome.Validation.Message, "too long")
}
