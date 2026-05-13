package usecase

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

const (
	defaultSwipeNextBatchSize   = 10
	swipePerformanceSampleLimit = 100
)

type CardRepoForSwipe interface {
	FindByIDTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error)
	UpdateFSRSStateTx(ctx context.Context, tx *gorm.DB, id string, state domain.FSRSState) error
	FindDueCardsTx(ctx context.Context, tx *gorm.DB, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error)
}

type CardgroupRepoForSwipe interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

type SwipeRecordRepoForSwipe interface {
	CreateTx(ctx context.Context, tx *gorm.DB, sr *domain.SwipeRecord) error
	ListRecentByUser(ctx context.Context, userID string, limit int) ([]*domain.SwipeRecord, error)
}

type SwipeUsecase struct {
	cardRepo      CardRepoForSwipe
	cardgroupRepo CardgroupRepoForSwipe
	swipeRepo     SwipeRecordRepoForSwipe
	scheduler     *service.FSRSScheduler
	ordering      *service.OrderingPolicy
	randSource    func() *rand.Rand
	tx            txRunner
	nextBatchSize int
}

type HandleSwipeInput struct {
	CardID      string
	CardgroupID string
	Mode        int
}

type SwipeOutput struct {
	NextCards       []*domain.Card
	PerformanceMode int
	Metrics         service.PerformanceMetrics
}

func NewSwipeUsecase(
	db *gorm.DB,
	cardRepo CardRepoForSwipe,
	cardgroupRepo CardgroupRepoForSwipe,
	swipeRepo SwipeRecordRepoForSwipe,
	scheduler *service.FSRSScheduler,
	nextBatchSize int,
) *SwipeUsecase {
	if scheduler == nil {
		scheduler = service.NewFSRSScheduler()
	}
	if nextBatchSize <= 0 {
		nextBatchSize = defaultSwipeNextBatchSize
	}
	uc := &SwipeUsecase{
		cardRepo:      cardRepo,
		cardgroupRepo: cardgroupRepo,
		swipeRepo:     swipeRepo,
		scheduler:     scheduler,
		ordering:      service.NewOrderingPolicy(),
		randSource: func() *rand.Rand {
			return rand.New(rand.NewSource(time.Now().UnixNano()))
		},
		nextBatchSize: nextBatchSize,
	}
	if db != nil {
		uc.tx = func(ctx context.Context, fn func(tx *gorm.DB) error) error {
			return db.WithContext(ctx).Transaction(fn)
		}
	}
	return uc
}

func NewSwipeUsecaseWithTx(
	cardRepo CardRepoForSwipe,
	cardgroupRepo CardgroupRepoForSwipe,
	swipeRepo SwipeRecordRepoForSwipe,
	scheduler *service.FSRSScheduler,
	nextBatchSize int,
	tx txRunner,
) *SwipeUsecase {
	uc := NewSwipeUsecase(nil, cardRepo, cardgroupRepo, swipeRepo, scheduler, nextBatchSize)
	uc.tx = tx
	return uc
}

func (u *SwipeUsecase) HandleSwipe(ctx context.Context, in HandleSwipeInput) (*SwipeOutput, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	rating, err := domain.RatingFromSwipeMode(in.Mode)
	if err != nil {
		return nil, gqlerr.BadUserInput("mode", err.Error())
	}
	if err := u.authorizeCardgroup(ctx, in.CardgroupID, user.Sub); err != nil {
		return nil, err
	}

	var nextCards []*domain.Card
	var now time.Time
	if u.tx == nil {
		return nil, gqlerr.Internal(ctx, errors.New("swipe usecase: transaction runner is not configured"))
	}
	err = u.tx(ctx, func(tx *gorm.DB) error {
		card, err := u.cardRepo.FindByIDTx(ctx, tx, in.CardID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return gqlerr.BadUserInput("cardId", "card not found")
			}
			return err
		}
		if card.CardgroupID != in.CardgroupID {
			return gqlerr.BadUserInput("cardId", "card not found")
		}

		now = time.Now().UTC()
		newState := u.scheduler.Apply(card.FSRS, rating, now)
		if err := u.cardRepo.UpdateFSRSStateTx(ctx, tx, card.ID, newState); err != nil {
			return err
		}
		sr, err := domain.NewSwipeRecord(user.Sub, card.ID, rating, now, newState)
		if err != nil {
			return err
		}
		if err := u.swipeRepo.CreateTx(ctx, tx, sr); err != nil {
			return err
		}
		nextCards, err = u.cardRepo.FindDueCardsTx(ctx, tx, in.CardgroupID, now, u.nextBatchSize)
		if err == nil {
			nextCards = u.ordering.Apply(nextCards, u.randSource())
		}
		return err
	})
	if err != nil {
		var ge *gqlerror.Error
		if errors.As(err, &ge) {
			return nil, ge
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	recentSwipes, err := u.swipeRepo.ListRecentByUser(ctx, user.Sub, swipePerformanceSampleLimit)
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	metrics := service.ComputeMetrics(swipeRecordsByValue(recentSwipes), now)
	return &SwipeOutput{
		NextCards:       nextCards,
		PerformanceMode: service.ModeFromMetrics(metrics),
		Metrics:         metrics,
	}, nil
}

func (u *SwipeUsecase) authorizeCardgroup(ctx context.Context, cardgroupID, userID string) error {
	cg, err := u.cardgroupRepo.FindByID(ctx, cardgroupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return gqlerr.BadUserInput("cardgroupId", "cardgroup not found")
		}
		return gqlerr.Internal(ctx, err)
	}
	if cg.OwnerID != userID {
		return gqlerr.Unauthenticated()
	}
	return nil
}

func swipeRecordsByValue(swipes []*domain.SwipeRecord) []domain.SwipeRecord {
	out := make([]domain.SwipeRecord, 0, len(swipes))
	for _, swipe := range swipes {
		if swipe != nil {
			out = append(out, *swipe)
		}
	}
	return out
}
