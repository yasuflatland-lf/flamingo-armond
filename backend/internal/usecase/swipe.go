package usecase

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
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
	logger        *slog.Logger
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

// HandleSwipeOutcome is the result of SwipeUsecase.HandleSwipe. Exactly one of
// Swipe or Validation is non-nil on a nil-error return.
//   - Swipe holds the success result (next cards + performance metrics + completion state).
//   - Validation holds field-level user-input errors: invalid mode, an unknown card, or
//     an unknown cardgroup. Validation.Field will be one of "mode", "cardId", or "cardgroupId".
//   - Authorization failures (caller does not own the cardgroup) and infrastructure errors
//     travel on the error channel, not on Validation.
type HandleSwipeOutcome struct {
	Swipe      *SwipeOutput
	Validation *InputValidationInfo
}

func NewSwipeUsecase(
	db *gorm.DB,
	cardRepo CardRepoForSwipe,
	cardgroupRepo CardgroupRepoForSwipe,
	swipeRepo SwipeRecordRepoForSwipe,
	scheduler *service.FSRSScheduler,
	nextBatchSize int,
	userCardFSRSRepo UserCardFSRSRepoForSwipe,
	logger *slog.Logger,
) *SwipeUsecase {
	if logger == nil {
		panic("usecase: swipe: logger is required")
	}
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
		logger:        logger,
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
	logger *slog.Logger,
) *SwipeUsecase {
	if logger == nil {
		panic("usecase: swipe: logger is required")
	}
	uc := NewSwipeUsecase(nil, cardRepo, cardgroupRepo, swipeRepo, scheduler, nextBatchSize, userCardFSRSRepo, logger)
	uc.tx = tx
	return uc
}

func (u *SwipeUsecase) HandleSwipe(ctx context.Context, in HandleSwipeInput) (HandleSwipeOutcome, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return HandleSwipeOutcome{}, ucerr.ErrUnauthenticated
	}
	rating, err := domain.RatingFromSwipeMode(in.Mode)
	if err != nil {
		// Use a plain user-facing message; err.Error() carries an internal layer
		// prefix ("rating: unknown swipe mode N") that is not appropriate on the wire.
		return HandleSwipeOutcome{Validation: NewInputValidationInfo("mode", "unknown swipe mode")}, nil
	}
	if err := authorizeCardgroupOrBadInput(ctx, u.cardgroupRepo, in.CardgroupID, user.Sub); err != nil {
		// authorizeCardgroupOrBadInput returns ucerr.NewValidationError("cardgroupId", ...) for
		// not-found and ucerr.ErrUnauthenticated for non-owner. The not-found case
		// is a validation variant; the non-owner case stays on the error channel.
		info, err := liftValidationErr(err)
		if err != nil {
			return HandleSwipeOutcome{}, err
		}
		if info != nil {
			return HandleSwipeOutcome{Validation: info}, nil
		}
	}

	var nextCards []*domain.Card
	var now time.Time
	if u.tx == nil {
		return HandleSwipeOutcome{}, eris.New("usecase: swipe: transaction runner is not configured")
	}
	if u.userFSRSRepo == nil {
		return HandleSwipeOutcome{}, eris.New("usecase: swipe: user card fsrs repository is not configured")
	}
	err = u.tx(ctx, func(tx *gorm.DB) error {
		card, err := u.cardRepo.FindByIDTx(ctx, tx, in.CardID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ucerr.NewValidationError("cardId", "card not found")
			}
			return err
		}
		if !card.BelongsToCardgroup(in.CardgroupID) {
			return ucerr.NewValidationError("cardId", "card not found")
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
		if err := current.ApplyRating(u.scheduler, rating, now); err != nil {
			return eris.Wrap(err, "usecase: swipe: apply rating")
		}
		if err := u.userFSRSRepo.UpsertTx(ctx, tx, current); err != nil {
			return err
		}
		sr, err := domain.NewSwipeRecord(user.Sub, card.ID, rating, now, current.State)
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
		// Validation errors surfaced from within the transaction closure (e.g.
		// card not found, cardgroup mismatch) are promoted to the outcome's
		// Validation variant rather than returned on the error channel.
		info, err := liftValidationErr(err)
		if err != nil {
			return HandleSwipeOutcome{}, err
		}
		if info != nil {
			return HandleSwipeOutcome{Validation: info}, nil
		}
	}
	recentSwipes, err := u.swipeRepo.ListRecentByUser(ctx, user.Sub, swipePerformanceSampleLimit)
	if err != nil {
		return HandleSwipeOutcome{}, eris.Wrap(err, "usecase: swipe: list recent swipes")
	}
	metrics := service.ComputeMetrics(swipeRecordsByValue(recentSwipes), now)
	return HandleSwipeOutcome{Swipe: &SwipeOutput{
		NextCards:       nextCards,
		PerformanceMode: service.ModeFromMetrics(metrics),
		Metrics:         metrics,
	}}, nil
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
