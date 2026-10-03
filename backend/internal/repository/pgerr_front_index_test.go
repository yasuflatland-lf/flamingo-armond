package repository

// White-box tests for classifyFrontIndexRowTooLarge. It is unexported, so the
// tests live in the same package; every case fabricates its error, mirroring
// pgerr_text_length_test.go, so no live DB is required.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rotisserie/eris"
)

func TestClassifyFrontIndexRowTooLarge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		frontIndex     string
		wantClassify   bool
		wantConstraint string
	}{
		{
			name:           "btree shape names the front index",
			err:            &pgconn.PgError{Code: "54000", ConstraintName: "uq_cards_cardgroup_front"},
			frontIndex:     "uq_cards_cardgroup_front",
			wantClassify:   true,
			wantConstraint: "uq_cards_cardgroup_front",
		},
		{
			name:           "index_form_tuple shape names no index",
			err:            &pgconn.PgError{Code: "54000", Routine: "index_form_tuple_context"},
			frontIndex:     "uq_cards_cardgroup_front",
			wantClassify:   true,
			wantConstraint: "uq_cards_cardgroup_front",
		},
		{
			name:           "index_form_tuple shape under its older routine name",
			err:            &pgconn.PgError{Code: "54000", Routine: "index_form_tuple"},
			frontIndex:     "uq_cards_cardgroup_front",
			wantClassify:   true,
			wantConstraint: "uq_cards_cardgroup_front",
		},
		{
			name:         "XID wraparound stop names no index and is not classified",
			err:          &pgconn.PgError{Code: "54000", Routine: "GetNewTransactionId"},
			frontIndex:   "uq_cards_cardgroup_front",
			wantClassify: false,
		},
		{
			name:         "MultiXact stop names no index and is not classified",
			err:          &pgconn.PgError{Code: "54000", Routine: "GetNewMultiXactId"},
			frontIndex:   "uq_cards_cardgroup_front",
			wantClassify: false,
		},
		{
			name:         "unnamed 54000 without a routine is not classified",
			err:          &pgconn.PgError{Code: "54000"},
			frontIndex:   "uq_cards_cardgroup_front",
			wantClassify: false,
		},
		{
			name:           "master front index",
			err:            &pgconn.PgError{Code: "54000", ConstraintName: "uq_master_cards_cg_front"},
			frontIndex:     "uq_master_cards_cg_front",
			wantClassify:   true,
			wantConstraint: "uq_master_cards_cg_front",
		},
		{
			name:         "54000 naming a different index is not classified",
			err:          &pgconn.PgError{Code: "54000", ConstraintName: "cards_pkey"},
			frontIndex:   "uq_cards_cardgroup_front",
			wantClassify: false,
		},
		{
			name:         "unique violation on the front index is not classified",
			err:          &pgconn.PgError{Code: "23505", ConstraintName: "uq_cards_cardgroup_front"},
			frontIndex:   "uq_cards_cardgroup_front",
			wantClassify: false,
		},
		{
			name:         "check violation is not classified",
			err:          &pgconn.PgError{Code: "23514", ConstraintName: "cards_front_length"},
			frontIndex:   "uq_cards_cardgroup_front",
			wantClassify: false,
		},
		{
			name:         "non-pg error is not classified",
			err:          errors.New("boom"),
			frontIndex:   "uq_cards_cardgroup_front",
			wantClassify: false,
		},
		{
			name:           "eris-wrapped btree shape is still classified",
			err:            eris.Wrap(&pgconn.PgError{Code: "54000", ConstraintName: "uq_cards_cardgroup_front"}, "x"),
			frontIndex:     "uq_cards_cardgroup_front",
			wantClassify:   true,
			wantConstraint: "uq_cards_cardgroup_front",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyFrontIndexRowTooLarge(tt.err, tt.frontIndex)
			if !tt.wantClassify {
				if got != nil {
					t.Fatalf("classifyFrontIndexRowTooLarge(%v, %q) = %v, want nil", tt.err, tt.frontIndex, got)
				}
				return
			}
			v, ok := errors.AsType[*TextLengthViolationError](got)
			if !ok {
				t.Fatalf("classifyFrontIndexRowTooLarge(%v, %q) = %v, want *TextLengthViolationError", tt.err, tt.frontIndex, got)
			}
			if v.Field != "front" {
				t.Fatalf("Field = %q, want %q", v.Field, "front")
			}
			if v.Constraint != tt.wantConstraint {
				t.Fatalf("Constraint = %q, want %q", v.Constraint, tt.wantConstraint)
			}
		})
	}
}
