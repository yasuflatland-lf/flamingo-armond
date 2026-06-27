package repository

// White-box tests for pgConstraintViolation. The predicate is unexported so the
// tests live in the same package. All assertions use fabricated *pgconn.PgError
// values — no live DB is required.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestPgConstraintViolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		err              error
		code             string
		constraintSubstr string
		want             bool
	}{
		{
			name:             "matching code and substr returns true",
			err:              &pgconn.PgError{Code: "23505", ConstraintName: "uq_cards_cardgroup_front"},
			code:             "23505",
			constraintSubstr: "uq_cards_cardgroup_front",
			want:             true,
		},
		{
			name:             "wrong code returns false",
			err:              &pgconn.PgError{Code: "23503", ConstraintName: "uq_cards_cardgroup_front"},
			code:             "23505",
			constraintSubstr: "uq_cards_cardgroup_front",
			want:             false,
		},
		{
			name:             "non-pg error returns false",
			err:              errors.New("some error"),
			code:             "23505",
			constraintSubstr: "uq_cards_cardgroup_front",
			want:             false,
		},
		{
			name:             "code match but substr miss returns false",
			err:              &pgconn.PgError{Code: "23505", ConstraintName: "cards_pkey"},
			code:             "23505",
			constraintSubstr: "uq_cards_cardgroup_front",
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := pgConstraintViolation(tt.err, tt.code, tt.constraintSubstr); got != tt.want {
				t.Fatalf("pgConstraintViolation(%v, %q, %q) = %v, want %v",
					tt.err, tt.code, tt.constraintSubstr, got, tt.want)
			}
		})
	}
}
