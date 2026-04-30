package domain

import (
	"errors"
	"testing"
	"time"
)

// ptr helpers for test brevity.
func ptrTime(t time.Time) *time.Time   { return &t }
func ptrF64(f float64) *float64        { return &f }
func ptrInt(i int) *int                { return &i }

// fullOverride returns an FSRSStateOverride with all nine fields set using the
// supplied values.
func fullOverride(
	due time.Time,
	stability, difficulty float64,
	elapsedDays, scheduledDays, reps, lapses, state int,
	lastReview time.Time,
) FSRSStateOverride {
	return FSRSStateOverride{
		Due:           ptrTime(due),
		Stability:     ptrF64(stability),
		Difficulty:    ptrF64(difficulty),
		ElapsedDays:   ptrInt(elapsedDays),
		ScheduledDays: ptrInt(scheduledDays),
		Reps:          ptrInt(reps),
		Lapses:        ptrInt(lapses),
		State:         ptrInt(state),
		LastReview:    ptrTime(lastReview),
	}
}

func TestNewFSRSStateFromInput(t *testing.T) {
	t.Helper()

	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	due := now.Add(24 * time.Hour)
	lastReview := now.Add(-48 * time.Hour)

	tests := []struct {
		name        string
		override    FSRSStateOverride
		wantState   FSRSState
		wantErr     error
		checkFields bool // when true, compare individual fields
	}{
		{
			name:      "all nil returns new card state",
			override:  FSRSStateOverride{},
			wantState: NewFSRSStateForNewCard(now),
		},
		{
			name:        "all nine fields set returns override values",
			override:    fullOverride(due, 3.5, 6.0, 1, 2, 5, 0, int(FSRSStateReview), lastReview),
			checkFields: true,
			wantState: FSRSState{
				Due:           due,
				Stability:     3.5,
				Difficulty:    6.0,
				ElapsedDays:   1,
				ScheduledDays: 2,
				Reps:          5,
				Lapses:        0,
				State:         FSRSStateReview,
				LastReview:    lastReview,
			},
		},
		{
			name: "single field set returns ErrFSRSOverridePartial",
			override: FSRSStateOverride{
				Due: ptrTime(due),
			},
			wantErr: ErrFSRSOverridePartial,
		},
		{
			name: "all but one field set returns ErrFSRSOverridePartial",
			override: FSRSStateOverride{
				// Due is intentionally omitted.
				Stability:     ptrF64(3.5),
				Difficulty:    ptrF64(6.0),
				ElapsedDays:   ptrInt(1),
				ScheduledDays: ptrInt(2),
				Reps:          ptrInt(5),
				Lapses:        ptrInt(0),
				State:         ptrInt(int(FSRSStateReview)),
				LastReview:    ptrTime(lastReview),
			},
			wantErr: ErrFSRSOverridePartial,
		},
		{
			name:     "state 0 (New) accepted",
			override: fullOverride(due, 2.5, 5.0, 0, 0, 0, 0, 0, lastReview),
			wantState: FSRSState{
				Due: due, Stability: 2.5, Difficulty: 5.0,
				State: FSRSStateNew, LastReview: lastReview,
			},
			checkFields: true,
		},
		{
			name:     "state 1 (Learning) accepted",
			override: fullOverride(due, 2.5, 5.0, 0, 0, 0, 0, 1, lastReview),
			wantState: FSRSState{
				Due: due, Stability: 2.5, Difficulty: 5.0,
				State: FSRSStateLearning, LastReview: lastReview,
			},
			checkFields: true,
		},
		{
			name:     "state 2 (Review) accepted",
			override: fullOverride(due, 2.5, 5.0, 0, 0, 0, 0, 2, lastReview),
			wantState: FSRSState{
				Due: due, Stability: 2.5, Difficulty: 5.0,
				State: FSRSStateReview, LastReview: lastReview,
			},
			checkFields: true,
		},
		{
			name:     "state 3 (Relearning) accepted",
			override: fullOverride(due, 2.5, 5.0, 0, 0, 0, 0, 3, lastReview),
			wantState: FSRSState{
				Due: due, Stability: 2.5, Difficulty: 5.0,
				State: FSRSStateRelearning, LastReview: lastReview,
			},
			checkFields: true,
		},
		{
			name:     "state 4 returns ErrFSRSOverrideStateInvalid",
			override: fullOverride(due, 2.5, 5.0, 0, 0, 0, 0, 4, lastReview),
			wantErr:  ErrFSRSOverrideStateInvalid,
		},
		{
			name:     "state -1 returns ErrFSRSOverrideStateInvalid",
			override: fullOverride(due, 2.5, 5.0, 0, 0, 0, 0, -1, lastReview),
			wantErr:  ErrFSRSOverrideStateInvalid,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()

			got, err := NewFSRSStateFromInput(tc.override, now)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want error %v, got %v", tc.wantErr, err)
				}
				// Zero value expected on error.
				if got != (FSRSState{}) {
					t.Fatalf("want zero FSRSState on error, got %+v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.checkFields {
				assertFSRSState(t, tc.wantState, got)
			} else {
				// For the all-nil case, compare directly.
				if got != tc.wantState {
					t.Fatalf("want %+v, got %+v", tc.wantState, got)
				}
			}
		})
	}
}

// assertFSRSState compares two FSRSState values field-by-field.
func assertFSRSState(t *testing.T, want, got FSRSState) {
	t.Helper()
	if !want.Due.Equal(got.Due) {
		t.Errorf("Due: want %v, got %v", want.Due, got.Due)
	}
	if want.Stability != got.Stability {
		t.Errorf("Stability: want %v, got %v", want.Stability, got.Stability)
	}
	if want.Difficulty != got.Difficulty {
		t.Errorf("Difficulty: want %v, got %v", want.Difficulty, got.Difficulty)
	}
	if want.ElapsedDays != got.ElapsedDays {
		t.Errorf("ElapsedDays: want %v, got %v", want.ElapsedDays, got.ElapsedDays)
	}
	if want.ScheduledDays != got.ScheduledDays {
		t.Errorf("ScheduledDays: want %v, got %v", want.ScheduledDays, got.ScheduledDays)
	}
	if want.Reps != got.Reps {
		t.Errorf("Reps: want %v, got %v", want.Reps, got.Reps)
	}
	if want.Lapses != got.Lapses {
		t.Errorf("Lapses: want %v, got %v", want.Lapses, got.Lapses)
	}
	if want.State != got.State {
		t.Errorf("State: want %v, got %v", want.State, got.State)
	}
	if !want.LastReview.Equal(got.LastReview) {
		t.Errorf("LastReview: want %v, got %v", want.LastReview, got.LastReview)
	}
}
