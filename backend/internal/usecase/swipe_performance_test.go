package usecase

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/domain/service"
)

type mockSwipeRecordRepoForSwipe struct {
	recent     []*domain.SwipeRecord
	createErr  error
	listErr    error
	listUserID string
	listLimit  int
	created    *domain.SwipeRecord
}

func (m *mockSwipeRecordRepoForSwipe) CreateTx(_ context.Context, _ *gorm.DB, sr *domain.SwipeRecord) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = sr
	m.recent = append([]*domain.SwipeRecord{sr}, m.recent...)
	return nil
}

func (m *mockSwipeRecordRepoForSwipe) ListRecentByUser(_ context.Context, userID string, limit int) ([]*domain.SwipeRecord, error) {
	m.listUserID = userID
	m.listLimit = limit
	return m.recent, m.listErr
}

func TestSwipeUsecase_HandleSwipePerformanceMode(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		recent     []*domain.SwipeRecord
		wantMode   int
		wantReview int
	}{
		{
			name:       "less than twenty reviews uses default mode",
			recent:     performanceSwipes(base, 9, 9, 5),
			wantMode:   service.ModeDefault,
			wantReview: 19,
		},
		{
			name:       "sixty percent success with high difficulty becomes difficult",
			recent:     performanceSwipes(base, 11, 8, 8),
			wantMode:   service.ModeDifficult,
			wantReview: 20,
		},
		{
			name:       "ninety five percent success becomes in while",
			recent:     performanceSwipes(base, 18, 1, 5),
			wantMode:   service.ModeInWhile,
			wantReview: 20,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cardRepo := &mockCardRepository{
				findResult: &domain.Card{
					ID:          "card-1",
					CardgroupID: "cg-1",
				},
				findDueRows: []*domain.Card{{ID: "next-1", CardgroupID: "cg-1"}},
			}
			cardgroupRepo := &mockCardgroupRepoForCard{
				findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
			}
			swipeRepo := &mockSwipeRecordRepoForSwipe{recent: append([]*domain.SwipeRecord(nil), tc.recent...)}
			userFSRSRepo := &mockUserCardFSRSRepository{
				byCardID: map[string]*domain.UserCardFSRS{
					"card-1": domain.NewUserCardFSRSForNewCard("user-1", "card-1", base),
				},
			}
			tx, _ := fakeTxRunner()
			uc := NewSwipeUsecaseWithTx(
				cardRepo,
				cardgroupRepo,
				swipeRepo,
				service.NewFSRSScheduler(),
				10,
				tx,
				userFSRSRepo,
			)

			got, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
				CardID:      "card-1",
				CardgroupID: "cg-1",
				Mode:        int(domain.RatingEasy),
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.PerformanceMode != tc.wantMode {
				t.Fatalf("performance mode=%d, want %d; metrics=%+v", got.PerformanceMode, tc.wantMode, got.Metrics)
			}
			if got.Metrics.ReviewCount != tc.wantReview {
				t.Fatalf("review count=%d, want %d", got.Metrics.ReviewCount, tc.wantReview)
			}
			if swipeRepo.created == nil {
				t.Fatal("expected swipe record to be created before metrics are listed")
			}
			if swipeRepo.listUserID != "user-1" {
				t.Fatalf("ListRecentByUser user=%q, want user-1", swipeRepo.listUserID)
			}
			if swipeRepo.listLimit != swipePerformanceSampleLimit {
				t.Fatalf("ListRecentByUser limit=%d, want %d", swipeRepo.listLimit, swipePerformanceSampleLimit)
			}
			if len(got.NextCards) != 1 || got.NextCards[0].ID != "next-1" {
				t.Fatalf("unexpected next cards: %+v", got.NextCards)
			}
		})
	}
}

func TestSwipeUsecase_HandleSwipeCreatesUserFSRSStateForFirstSwipe(t *testing.T) {
	t.Parallel()

	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: "cg-1",
		},
		findDueRows: []*domain.Card{{ID: "next-1", CardgroupID: "cg-1"}},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{}
	userFSRSRepo := &mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}}
	tx, _ := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		swipeRepo,
		service.NewFSRSScheduler(),
		10,
		tx,
		userFSRSRepo,
	)

	got, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Mode:        int(domain.RatingEasy),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userFSRSRepo.upserted == nil {
		t.Fatal("expected per-user FSRS row to be upserted")
	}
	if userFSRSRepo.upserted.UserID != "user-1" || userFSRSRepo.upserted.CardID != "card-1" {
		t.Fatalf("unexpected upserted identity: %+v", userFSRSRepo.upserted)
	}
	if userFSRSRepo.upserted.State.Reps != 1 {
		t.Fatalf("expected first swipe to produce reps=1, got %+v", userFSRSRepo.upserted.State)
	}
	if swipeRepo.created == nil || swipeRepo.created.StateAfter != userFSRSRepo.upserted.State {
		t.Fatalf("swipe snapshot must match upserted user state, swipe=%+v ucs=%+v", swipeRepo.created, userFSRSRepo.upserted)
	}
	if len(got.NextCards) != 1 || got.NextCards[0].ID != "next-1" {
		t.Fatalf("unexpected next cards: %+v", got.NextCards)
	}
}

func performanceSwipes(now time.Time, successes, failures int, difficulty float64) []*domain.SwipeRecord {
	swipes := make([]*domain.SwipeRecord, 0, successes+failures)
	for i := 0; i < successes; i++ {
		swipes = append(swipes, performanceSwipe(domain.RatingEasy, now.Add(-time.Duration(i+1)*time.Hour), difficulty))
	}
	for i := 0; i < failures; i++ {
		swipes = append(swipes, performanceSwipe(domain.RatingAgain, now.Add(-time.Duration(successes+i+1)*time.Hour), difficulty))
	}
	return swipes
}

func performanceSwipe(rating domain.Rating, reviewedAt time.Time, difficulty float64) *domain.SwipeRecord {
	return &domain.SwipeRecord{
		ID:         reviewedAt.Format("20060102150405"),
		UserID:     "user-1",
		CardID:     "card-1",
		Rating:     rating,
		ReviewedAt: reviewedAt,
		StateAfter: domain.FSRSState{
			Difficulty:    difficulty,
			ElapsedDays:   1,
			ScheduledDays: 1,
			State:         domain.FSRSStateReview,
		},
	}
}
