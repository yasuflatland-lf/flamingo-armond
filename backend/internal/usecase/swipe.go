package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

const defaultSwipeNextBatchSize = 10

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
}

type SwipeUsecase struct {
	cardRepo      CardRepoForSwipe
	cardgroupRepo CardgroupRepoForSwipe
	swipeRepo     SwipeRecordRepoForSwipe
	scheduler     *service.FSRSScheduler
	db            *gorm.DB
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
	return &SwipeUsecase{
		db:            db,
		cardRepo:      cardRepo,
		cardgroupRepo: cardgroupRepo,
		swipeRepo:     swipeRepo,
		scheduler:     scheduler,
		nextBatchSize: nextBatchSize,
	}
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
	err = u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

		now := time.Now().UTC()
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
		return err
	})
	if err != nil {
		var ge *gqlerror.Error
		if errors.As(err, &ge) {
			return nil, ge
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	return &SwipeOutput{NextCards: nextCards, PerformanceMode: 0}, nil
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
