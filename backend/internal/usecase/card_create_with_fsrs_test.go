package usecase

import (
	"testing"
	"time"

	"backend/internal/domain"
)

// TestCardUsecase_Create_FSRS_AllNilUsesDefaults verifies that when CreateCardInput
// carries no FSRS override, the persisted card receives the default new-card
// FSRS state (state=New, stability=2.5, difficulty=5.0).
func TestCardUsecase_Create_FSRS_AllNilUsesDefaults(t *testing.T) {
	t.Parallel()
	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo)

	got, err := uc.Create(authedCtx("u1"), CreateCardInput{
		CardgroupID: "cg1",
		Front:       "front",
		Back:        "back",
		FSRS:        nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Card == nil {
		t.Fatalf("expected outcome.Card to be non-nil, got duplicate=%+v", got.Duplicate)
	}
	if got.Card.FSRS.State != domain.FSRSStateNew {
		t.Fatalf("expected default state=New, got %d", got.Card.FSRS.State)
	}
	if got.Card.FSRS.Stability != 2.5 || got.Card.FSRS.Difficulty != 5.0 {
		t.Fatalf("expected default stability=2.5, difficulty=5.0, got %+v", got.Card.FSRS)
	}
}

// TestCardUsecase_Create_FSRS_FullOverride verifies that when every override
// pointer is non-nil, the persisted card carries the supplied values.
func TestCardUsecase_Create_FSRS_FullOverride(t *testing.T) {
	t.Parallel()
	due := time.Date(2030, 6, 1, 12, 0, 0, 0, time.UTC)
	last := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	stability := 7.25
	difficulty := 3.5
	elapsed := 11
	scheduled := 22
	reps := 4
	lapses := 1
	state := int(domain.FSRSStateReview)

	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo)

	got, err := uc.Create(authedCtx("u1"), CreateCardInput{
		CardgroupID: "cg1",
		Front:       "front",
		Back:        "back",
		FSRS: &domain.FSRSStateOverride{
			Due:           &due,
			Stability:     &stability,
			Difficulty:    &difficulty,
			ElapsedDays:   &elapsed,
			ScheduledDays: &scheduled,
			Reps:          &reps,
			Lapses:        &lapses,
			State:         &state,
			LastReview:    &last,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Card == nil {
		t.Fatalf("expected outcome.Card to be non-nil, got duplicate=%+v", got.Duplicate)
	}
	if !got.Card.FSRS.Due.Equal(due) || !got.Card.FSRS.LastReview.Equal(last) {
		t.Fatalf("override timestamps not applied: %+v", got.Card.FSRS)
	}
	if got.Card.FSRS.Stability != stability || got.Card.FSRS.Difficulty != difficulty {
		t.Fatalf("override floats not applied: %+v", got.Card.FSRS)
	}
	if got.Card.FSRS.ElapsedDays != elapsed || got.Card.FSRS.ScheduledDays != scheduled {
		t.Fatalf("override day counters not applied: %+v", got.Card.FSRS)
	}
	if got.Card.FSRS.Reps != reps || got.Card.FSRS.Lapses != lapses {
		t.Fatalf("override rep/lapse counters not applied: %+v", got.Card.FSRS)
	}
	if got.Card.FSRS.State != domain.FSRSStateReview {
		t.Fatalf("override state not applied: got %d", got.Card.FSRS.State)
	}
	if cardRepo.capturedCreate == nil || cardRepo.capturedCreate.FSRS.Stability != stability {
		t.Fatalf("repo did not receive overridden FSRS state: %+v", cardRepo.capturedCreate)
	}
}

// TestCardUsecase_Create_FSRS_PartialOverride verifies that when some override
// pointers are nil and others non-nil, the call is rejected with BAD_USER_INPUT
// carrying field=input.fsrs.
func TestCardUsecase_Create_FSRS_PartialOverride(t *testing.T) {
	t.Parallel()
	stability := 7.25 // only one of nine fields populated -> partial.
	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo)

	_, err := uc.Create(authedCtx("u1"), CreateCardInput{
		CardgroupID: "cg1",
		Front:       "front",
		Back:        "back",
		FSRS: &domain.FSRSStateOverride{
			Stability: &stability,
		},
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "input.fsrs")
	if cardRepo.capturedCreate != nil {
		t.Fatal("expected repo.Create not to be called on partial override")
	}
}

// TestCardUsecase_Create_FSRS_StateOutOfRange verifies that an out-of-range
// state value with all other fields populated yields BAD_USER_INPUT
// field=input.state.
func TestCardUsecase_Create_FSRS_StateOutOfRange(t *testing.T) {
	t.Parallel()
	due := time.Date(2030, 6, 1, 12, 0, 0, 0, time.UTC)
	last := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	stability := 7.25
	difficulty := 3.5
	elapsed := 11
	scheduled := 22
	reps := 4
	lapses := 1
	state := 99

	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}}
	uc := NewCardUsecase(nil, cardRepo, cgRepo)

	_, err := uc.Create(authedCtx("u1"), CreateCardInput{
		CardgroupID: "cg1",
		Front:       "front",
		Back:        "back",
		FSRS: &domain.FSRSStateOverride{
			Due:           &due,
			Stability:     &stability,
			Difficulty:    &difficulty,
			ElapsedDays:   &elapsed,
			ScheduledDays: &scheduled,
			Reps:          &reps,
			Lapses:        &lapses,
			State:         &state,
			LastReview:    &last,
		},
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "input.state")
	if cardRepo.capturedCreate != nil {
		t.Fatal("expected repo.Create not to be called on out-of-range state")
	}
}
