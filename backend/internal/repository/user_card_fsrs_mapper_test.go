package repository

// White-box tests for userCardFSRSToDomain. The mapper is unexported and is a
// pure function of the row struct, so the tests live in the same package and
// need no live DB — the eight sibling guard cases in user_card_fsrs_test.go all
// go through a testcontainer, which leaves a contributor without Docker with no
// coverage of the guard at all. These pin the reject side and, crucially, the
// exact message each rejection produces: the message is the only signal an
// operator gets when a row edited outside the application reaches a read, so a
// silently reworded or dropped guard would leave a corrupt column to flow into
// the GraphQL Floats it feeds and break JSON marshalling of the whole response.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

// validUserCardFSRSRow returns a row every guard in userCardFSRSToDomain
// accepts. Each reject case mutates exactly one column so the failure is
// attributable to that column's guard. Every numeric and time column carries a
// distinct value: the mapper copies thirteen same-typed fields across in one
// struct literal, and equal fixture values would let a transposed pair
// (elapsed/scheduled days, created/updated timestamps) satisfy the accept-path
// assertion.
func validUserCardFSRSRow() gormUserCardFSRS {
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	good := int(domain.RatingGood)
	return gormUserCardFSRS{
		UserID:        "00000000-0000-0000-0000-000000000001",
		CardID:        "00000000-0000-0000-0000-000000000002",
		State:         int(domain.FSRSPhaseReview),
		Due:           now.Add(120 * time.Hour),
		Stability:     domain.NewCardStability,
		Difficulty:    domain.NewCardDifficulty,
		Reps:          3,
		Lapses:        1,
		LastReview:    now.Add(-48 * time.Hour),
		LastRating:    &good,
		ElapsedDays:   2,
		ScheduledDays: 5,
		CreatedAt:     now.Add(-72 * time.Hour),
		UpdatedAt:     now,
	}
}

func TestUserCardFSRSToDomain_RejectsCorruptColumns(t *testing.T) {
	t.Parallel()

	badRating := 7

	tests := []struct {
		name    string
		mutate  func(*gormUserCardFSRS)
		wantMsg string
	}{
		{
			name:    "phase below the lowest constant",
			mutate:  func(r *gormUserCardFSRS) { r.State = -1 },
			wantMsg: "repository: invalid FSRSPhase value -1 for card 00000000-0000-0000-0000-000000000002",
		},
		{
			name:    "phase past the highest constant",
			mutate:  func(r *gormUserCardFSRS) { r.State = 4 },
			wantMsg: "repository: invalid FSRSPhase value 4 for card 00000000-0000-0000-0000-000000000002",
		},
		{
			name:    "zero stability",
			mutate:  func(r *gormUserCardFSRS) { r.Stability = 0 },
			wantMsg: "repository: invalid stability value 0 for card 00000000-0000-0000-0000-000000000002",
		},
		{
			name:    "negative stability",
			mutate:  func(r *gormUserCardFSRS) { r.Stability = -1.5 },
			wantMsg: "repository: invalid stability value -1.5 for card 00000000-0000-0000-0000-000000000002",
		},
		{
			name:    "difficulty below the minimum",
			mutate:  func(r *gormUserCardFSRS) { r.Difficulty = domain.MinDifficulty - 0.5 },
			wantMsg: "repository: invalid difficulty value 0.5 for card 00000000-0000-0000-0000-000000000002",
		},
		{
			name:    "difficulty above the maximum",
			mutate:  func(r *gormUserCardFSRS) { r.Difficulty = domain.MaxDifficulty + 0.5 },
			wantMsg: "repository: invalid difficulty value 10.5 for card 00000000-0000-0000-0000-000000000002",
		},
		{
			name:    "last_rating outside the FSRS range",
			mutate:  func(r *gormUserCardFSRS) { r.LastRating = &badRating },
			wantMsg: "repository: invalid last_rating value 7 for card 00000000-0000-0000-0000-000000000002",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			row := validUserCardFSRSRow()
			tc.mutate(&row)

			got, err := userCardFSRSToDomain(row)
			require.Error(t, err)
			require.Nil(t, got)
			require.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

// TestUserCardFSRSToDomain_AcceptsValidRow guards the accept side. A narrowed
// predicate would hard-fail a whole read for a legitimately persisted row, so
// the reject cases above are only half the contract. The whole struct is
// compared rather than a field subset: an unasserted field is one a transposed
// or dropped assignment could corrupt with only the Docker-gated integration
// tests to catch it.
func TestUserCardFSRSToDomain_AcceptsValidRow(t *testing.T) {
	t.Parallel()

	row := validUserCardFSRSRow()

	got, err := userCardFSRSToDomain(row)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, &domain.UserCardFSRS{
		UserID: domain.UserID(row.UserID),
		CardID: row.CardID,
		State: domain.FSRSState{
			Due:           row.Due,
			Stability:     row.Stability,
			Difficulty:    row.Difficulty,
			ElapsedDays:   row.ElapsedDays,
			ScheduledDays: row.ScheduledDays,
			Reps:          row.Reps,
			Lapses:        row.Lapses,
			Phase:         domain.FSRSPhaseReview,
			LastReview:    row.LastReview,
			LastRating:    domain.RatingGood,
		},
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, got)
}

// TestUserCardFSRSToDomain_NullLastRatingMapsToZero pins the nullable column's
// mapping: last_rating IS NULL denotes a synthesized state no swipe has rated,
// which the domain spells as the zero Rating — not a guard violation.
func TestUserCardFSRSToDomain_NullLastRatingMapsToZero(t *testing.T) {
	t.Parallel()

	row := validUserCardFSRSRow()
	row.LastRating = nil

	got, err := userCardFSRSToDomain(row)
	require.NoError(t, err)
	require.Equal(t, domain.Rating(0), got.State.LastRating)
	require.False(t, got.State.LastRating.IsValid())
}
