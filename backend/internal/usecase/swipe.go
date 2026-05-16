package usecase

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

const (
	defaultSwipeNextBatchSize   = 10
	swipePerformanceSampleLimit = 100
)

// SwipeNextBatchSize reads SWIPE_NEXT_BATCH_SIZE from the environment and
// returns it as an int. Returns defaultSwipeNextBatchSize when the variable is
// absent, non-numeric, or non-positive.
func SwipeNextBatchSize(logger *slog.Logger) int {
	v := os.Getenv("SWIPE_NEXT_BATCH_SIZE")
	if v == "" {
		return defaultSwipeNextBatchSize
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		logger.Warn("invalid SWIPE_NEXT_BATCH_SIZE, using default", "value", v, "default", defaultSwipeNextBatchSize)
		return defaultSwipeNextBatchSize
	}
	return n
}

type CardRepoForSwipe interface {
	FindByIDTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error)
	FindDueCardsForUserTx(ctx context.Context, tx *gorm.DB, userID, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error)
}

type CardgroupRepoForSwipe interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

type SwipeRecordRepoForSwipe interface {
	CreateTx(ctx context.Context, tx *gorm.DB, sr *domain.SwipeRecord) error
	ListRecentByUser(ctx context.Context, userID string, limit int) ([]*domain.SwipeRecord, error)
}

type UserCardFSRSRepoForSwipe interface {
	UpsertTx(ctx context.Context, tx *gorm.DB, u *domain.UserCardFSRS) error
	FindByUserAndCardIDs(ctx context.Context, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error)
	FindByUserAndCardIDsTx(ctx context.Context, tx *gorm.DB, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error)
}

type SwipeUsecase struct {
	cardRepo      CardRepoForSwipe
	cardgroupRepo CardgroupRepoForSwipe
	swipeRepo     SwipeRecordRepoForSwipe
	userFSRSRepo  UserCardFSRSRepoForSwipe
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
	userCardFSRSRepo UserCardFSRSRepoForSwipe,
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
		userFSRSRepo:  userCardFSRSRepo,
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
	userCardFSRSRepo UserCardFSRSRepoForSwipe,
) *SwipeUsecase {
	uc := NewSwipeUsecase(nil, cardRepo, cardgroupRepo, swipeRepo, scheduler, nextBatchSize, userCardFSRSRepo)
	uc.tx = tx
	return uc
}

func (u *SwipeUsecase) HandleSwipe(ctx context.Context, in HandleSwipeInput) (*SwipeOutput, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, ucerr.ErrUnauthenticated
	}
	rating, err := domain.RatingFromSwipeMode(in.Mode)
	if err != nil {
		return nil, &ucerr.ValidationError{Field: "mode", Message: err.Error()}
	}
	if err := u.authorizeCardgroup(ctx, in.CardgroupID, user.Sub); err != nil {
		return nil, err
	}

	var nextCards []*domain.Card
	var now time.Time
	if u.tx == nil {
		return nil, eris.New("usecase: swipe: transaction runner is not configured")
	}
	if u.userFSRSRepo == nil {
		return nil, eris.New("usecase: swipe: user card fsrs repository is not configured")
	}
	err = u.tx(ctx, func(tx *gorm.DB) error {
		card, err := u.cardRepo.FindByIDTx(ctx, tx, in.CardID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return &ucerr.ValidationError{Field: "cardId", Message: "card not found"}
			}
			return err
		}
		if card.CardgroupID != in.CardgroupID {
			return &ucerr.ValidationError{Field: "cardId", Message: "card not found"}
		}

		now = time.Now().UTC()
		byCardID, err := u.userFSRSRepo.FindByUserAndCardIDsTx(ctx, tx, user.Sub, []string{card.ID})
		if err != nil {
			return err
		}
		current := byCardID[card.ID]
		if current == nil {
			current = domain.NewUserCardFSRSForNewCard(user.Sub, card.ID, now)
		}
		newState := u.scheduler.Apply(current.State, rating, now)
		if err := u.userFSRSRepo.UpsertTx(ctx, tx, &domain.UserCardFSRS{
			UserID:    user.Sub,
			CardID:    card.ID,
			State:     newState,
			CreatedAt: current.CreatedAt,
			UpdatedAt: now,
		}); err != nil {
			return err
		}
		sr, err := domain.NewSwipeRecord(user.Sub, card.ID, rating, now, newState)
		if err != nil {
			return err
		}
		if err := u.swipeRepo.CreateTx(ctx, tx, sr); err != nil {
			return err
		}
		nextCards, err = u.cardRepo.FindDueCardsForUserTx(ctx, tx, user.Sub, in.CardgroupID, now, u.nextBatchSize)
		if err != nil {
			return err
		}
		nextCards = u.ordering.Apply(nextCards, u.randSource())
		return nil
	})
	if err != nil {
		return nil, err
	}
	recentSwipes, err := u.swipeRepo.ListRecentByUser(ctx, user.Sub, swipePerformanceSampleLimit)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: swipe: list recent swipes")
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
			return &ucerr.ValidationError{Field: "cardgroupId", Message: "cardgroup not found"}
		}
		return eris.Wrap(err, "usecase: swipe: find cardgroup")
	}
	if !cg.IsOwnedBy(userID) {
		return ucerr.ErrUnauthenticated
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
