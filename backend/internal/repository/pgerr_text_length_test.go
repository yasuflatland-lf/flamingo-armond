package repository

// White-box tests for classifyTextLengthViolation and its constraint-name
// parser. Both are unexported so the tests live in the same package. Every case
// uses a fabricated *pgconn.PgError -- no live DB is required, mirroring
// pgerr_test.go and master_card_classify_fk_test.go.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rotisserie/eris"
)

func TestClassifyTextLengthViolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		err           error
		wantField     string
		wantClassify  bool
		wantConstrain string
	}{
		{
			name:          "cards front length violation yields front",
			err:           &pgconn.PgError{Code: "23514", ConstraintName: "cards_front_length"},
			wantField:     "front",
			wantClassify:  true,
			wantConstrain: "cards_front_length",
		},
		{
			name:          "cards back length violation yields back",
			err:           &pgconn.PgError{Code: "23514", ConstraintName: "cards_back_length"},
			wantField:     "back",
			wantClassify:  true,
			wantConstrain: "cards_back_length",
		},
		{
			name:          "cardgroups name length violation yields name",
			err:           &pgconn.PgError{Code: "23514", ConstraintName: "cardgroups_name_length"},
			wantField:     "name",
			wantClassify:  true,
			wantConstrain: "cardgroups_name_length",
		},
		{
			name:          "multi-word table prefix still yields the column segment",
			err:           &pgconn.PgError{Code: "23514", ConstraintName: "master_cardgroups_name_length"},
			wantField:     "name",
			wantClassify:  true,
			wantConstrain: "master_cardgroups_name_length",
		},
		{
			name:         "wrapped error is still classified",
			err:          eris.Wrap(&pgconn.PgError{Code: "23514", ConstraintName: "master_cards_back_length"}, "repository: master card: create"),
			wantField:    "back",
			wantClassify: true,
			// eris.Wrap preserves the cause, so errors.As reaches the PgError.
			wantConstrain: "master_cards_back_length",
		},
		{
			name:         "unique violation on the same table is not classified",
			err:          &pgconn.PgError{Code: "23505", ConstraintName: "uq_cards_cardgroup_front"},
			wantClassify: false,
		},
		{
			name:         "check violation on a non-length constraint is not classified",
			err:          &pgconn.PgError{Code: "23514", ConstraintName: "user_preferences_new_card_ratio_range"},
			wantClassify: false,
		},
		{
			name:         "check violation with no column segment is not classified",
			err:          &pgconn.PgError{Code: "23514", ConstraintName: "_length"},
			wantClassify: false,
		},
		{
			name:         "non-pg error is not classified",
			err:          errors.New("some error"),
			wantClassify: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyTextLengthViolation(tt.err)
			if !tt.wantClassify {
				if got != nil {
					t.Fatalf("classifyTextLengthViolation(%v) = %v, want nil", tt.err, got)
				}
				return
			}
			v, ok := errors.AsType[*TextLengthViolationError](got)
			if !ok {
				t.Fatalf("classifyTextLengthViolation(%v) = %v, want *TextLengthViolationError", tt.err, got)
			}
			if v.Field != tt.wantField {
				t.Fatalf("Field = %q, want %q", v.Field, tt.wantField)
			}
			if v.Constraint != tt.wantConstrain {
				t.Fatalf("Constraint = %q, want %q", v.Constraint, tt.wantConstrain)
			}
			if v.Error() == "" {
				t.Fatal("Error() returned an empty string")
			}
		})
	}
}

// TestTextLengthViolationError_SurvivesErisWrap pins the pointer-receiver
// contract: a caller that wraps the classified error must still recover the
// field with errors.AsType, because the usecase layer reads Field to build the
// BAD_USER_INPUT extensions payload.
func TestTextLengthViolationError_SurvivesErisWrap(t *testing.T) {
	t.Parallel()
	orig := &TextLengthViolationError{Constraint: "cards_front_length", Field: "front"}
	wrapped := eris.Wrap(orig, "usecase: card: create: repo create")

	v, ok := errors.AsType[*TextLengthViolationError](wrapped)
	if !ok {
		t.Fatalf("errors.AsType failed on %v", wrapped)
	}
	if v.Field != "front" {
		t.Fatalf("Field = %q, want %q", v.Field, "front")
	}
}
