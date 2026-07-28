package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/logging"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

const (
	swipePerformanceSampleLimit = 100
)

type CardRepoForSwipe interface {
	FindByIDForUpdateTx(ctx context.Context, tx repository.Tx, id string) (*domain.Card, error)
}

type CardgroupRepoForSwipe interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

type SwipeRecordRepoForSwipe interface {
	CreateTx(ctx context.Context, tx repository.Tx, sr *domain.SwipeRecord) error
	ListRecentByUser(ctx context.Context, userID string, limit int) ([]*domain.SwipeRecord, error)
}

type UserCardFSRSRepoForSwipe interface {
	UpsertTx(ctx context.Context, tx repository.Tx, u *domain.UserCardFSRS) error
	FindByUserAndCardIDs(ctx context.Context, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error)
	FindByUserAndCardIDsTx(ctx context.Context, tx repository.Tx, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error)
}

// SwipeUsecase processes a single card swipe and advances the FSRS schedule.
// A repeat review within the same JST learn day or without FSRS scheduling
// credit is accepted and ignored: the schedule is left untouched, no second
// swipe record is written, and the normal success outcome is still returned.
type SwipeUsecase interface {
	HandleSwipe(ctx context.Context, in HandleSwipeInput) (HandleSwipeOutcome, error)
}

type swipeUsecase struct {
	cardRepo       CardRepoForSwipe
	cardgroupRepo  CardgroupRepoForSwipe
	swipeRepo      SwipeRecordRepoForSwipe
	userFSRSRepo   UserCardFSRSRepoForSwipe
	scheduler      *service.FSRSScheduler
	applyRating    func(current *domain.UserCardFSRS, scheduler domain.FSRSScheduler, rating domain.Rating, now time.Time) error
	newSwipeRecord func(userID domain.UserID, cardID string, cardgroupID domain.CardgroupID, rating domain.Rating, reviewedAt time.Time, stateBefore, stateAfter domain.FSRSState) (*domain.SwipeRecord, error)
	tx             txRunner
	clock          Clock
	logger         *slog.Logger
}

type HandleSwipeInput struct {
	CardID      string
	CardgroupID domain.CardgroupID
	Rating      int
}

type SwipeOutput struct {
	PerformanceMode int
	Metrics         service.PerformanceMetrics
}

// HandleSwipeOutcome is the result of SwipeUsecase.HandleSwipe. Exactly one of
// Swipe or Validation is non-nil on a nil-error return.
//   - Swipe holds the success result (performance metrics).
//   - Validation holds field-level user-input errors: invalid mode, an unknown card, or
//     an unknown cardgroup. Validation.Field will be one of "mode", "cardId", or "cardgroupId".
//   - Authorization failures (caller does not own the cardgroup) and infrastructure errors
//     travel on the error channel, not on Validation.
type HandleSwipeOutcome struct {
	Swipe      *SwipeOutput
	Validation *InputValidationInfo
}

func NewSwipeUsecase(
	db repository.Tx,
	cardRepo CardRepoForSwipe,
	cardgroupRepo CardgroupRepoForSwipe,
	swipeRepo SwipeRecordRepoForSwipe,
	scheduler *service.FSRSScheduler,
	userCardFSRSRepo UserCardFSRSRepoForSwipe,
	logger *slog.Logger,
) SwipeUsecase {
	if logger == nil {
		panic("usecase: swipe: logger is required")
	}
	if scheduler == nil {
		scheduler = service.NewFSRSScheduler()
	}
	uc := &swipeUsecase{
		cardRepo:      cardRepo,
		cardgroupRepo: cardgroupRepo,
		swipeRepo:     swipeRepo,
		userFSRSRepo:  userCardFSRSRepo,
		scheduler:     scheduler,
		applyRating: func(current *domain.UserCardFSRS, scheduler domain.FSRSScheduler, rating domain.Rating, now time.Time) error {
			return current.ApplyRating(scheduler, rating, now)
		},
		newSwipeRecord: domain.NewSwipeRecord,
		clock:          systemClock{},
		logger:         logger,
	}
	uc.tx = newTxRunner(db)
	return uc
}

func NewSwipeUsecaseWithTx(
	cardRepo CardRepoForSwipe,
	cardgroupRepo CardgroupRepoForSwipe,
	swipeRepo SwipeRecordRepoForSwipe,
	scheduler *service.FSRSScheduler,
	tx txRunner,
	userCardFSRSRepo UserCardFSRSRepoForSwipe,
	logger *slog.Logger,
) SwipeUsecase {
	uc := NewSwipeUsecase(nil, cardRepo, cardgroupRepo, swipeRepo, scheduler, userCardFSRSRepo, logger).(*swipeUsecase)
	uc.tx = tx
	return uc
}

func (u *swipeUsecase) HandleSwipe(ctx context.Context, in HandleSwipeInput) (HandleSwipeOutcome, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return HandleSwipeOutcome{}, err
	}
	rating, err := domain.RatingFromSwipe(in.Rating)
	if err != nil {
		// Use a plain user-facing message; err.Error() carries an internal layer
		// prefix ("rating: unknown swipe rating N") that is not appropriate on the wire.
		return HandleSwipeOutcome{Validation: NewInputValidationInfo("rating", "unknown swipe rating")}, nil
	}
	if err := authorizeCardgroupOrBadInput(ctx, u.cardgroupRepo, in.CardgroupID, domain.UserID(user.Sub)); err != nil {
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

	var now time.Time
	if u.userFSRSRepo == nil {
		return HandleSwipeOutcome{}, eris.New("usecase: swipe: user card fsrs repository is not configured")
	}
	err = runInTx(ctx, u.tx, func(tx repository.Tx) error {
		card, err := u.cardRepo.FindByIDForUpdateTx(ctx, tx, in.CardID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ucerr.NewValidationError("cardId", "card not found")
			}
			return wrapSwipeErr(err, "usecase: swipe: find card by id")
		}
		if !card.BelongsToCardgroup(in.CardgroupID) {
			return ucerr.NewValidationError("cardId", "card not found")
		}

		// Read the request instant through the injected clock port.
		now = u.clock.Now().UTC()
		byCardID, err := u.userFSRSRepo.FindByUserAndCardIDsTx(ctx, tx, user.Sub, []string{card.ID})
		if err != nil {
			return wrapSwipeErr(err, "usecase: swipe: find user-card fsrs")
		}
		// Repeat-review guard. The learn queue never serves a card twice within
		// one JST learn day or without FSRS scheduling credit, so a swipe that
		// violates either rule is a replay: a retried request on a flaky
		// connection, a second browser tab, or a reload after a false failure.
		// Accept it and ignore it — re-applying the rating would distort the
		// schedule and double-count the review in the learner's statistics.
		//
		// The check reads the row returned by the repository, never `current`
		// after the new-card synthesis below: NewUserCardFSRSForNewCard stamps
		// LastReview with now, so gating on the synthesized state would skip
		// the very first swipe of every brand-new card.
		//
		// Both disjuncts are required. ReviewedWithinLearnDay alone leaves a gap
		// of up to 23 hours: JST midnight is 15:00 UTC, so a review at 00:30 UTC
		// (09:30 JST) and a swipe at 23:30 UTC sit in different JST learn days
		// but the same UTC date and earn no credit. EarnsSchedulingCredit alone
		// drops the product's spacing rule: a review at 16:00 UTC (01:00 JST)
		// and a swipe at 01:00 UTC the next day are only nine hours apart and
		// remain inside one JST learn day even though FSRS grants credit.
		//
		// domain.ReviewedWithinLearnDay is the exact complement of the
		// serving-side SQL window (repository/card_due.go), so its comparator
		// and the serving comparator must move together.
		existing := byCardID[card.ID]
		if existing != nil &&
			(domain.ReviewedWithinLearnDay(existing.State.LastReview, now) ||
				!domain.EarnsSchedulingCredit(existing.State.LastReview, now)) {
			u.logger.InfoContext(ctx, "swipe: repeat review ignored",
				"card_id", card.ID,
				"learn_day_start", domain.StartOfLearnDay(now),
				"last_review", existing.State.LastReview,
			)
			return nil
		}
		current := domain.UserCardFSRSOrNew(existing, domain.UserID(user.Sub), card.ID, now)
		// Snapshot the pre-swipe state before applyRating mutates current.State
		// in place. For a brand-new card current came from
		// NewUserCardFSRSForNewCard, so before.Phase == FSRSPhaseNew.
		before := current.State
		if err := u.applyRating(current, u.scheduler, rating, now); err != nil {
			return eris.Wrap(err, "usecase: swipe: apply rating")
		}
		if err := u.userFSRSRepo.UpsertTx(ctx, tx, current); err != nil {
			return wrapSwipeErr(err, "usecase: swipe: upsert user-card fsrs")
		}
		sr, err := u.newSwipeRecord(domain.UserID(user.Sub), card.ID, card.CardgroupID, rating, now, before, current.State)
		if err != nil {
			return eris.Wrap(err, "usecase: swipe: new swipe record")
		}
		if err := u.swipeRepo.CreateTx(ctx, tx, sr); err != nil {
			return wrapSwipeErr(err, "usecase: swipe: insert swipe record")
		}
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
	// The transaction has committed: the FSRS row and the swipe record are
	// durable from here on. Everything below is read-only telemetry assembly,
	// so past this point the error channel means "the swipe was NOT persisted".
	metrics, err := u.performanceSnapshot(ctx, user.Sub, now)
	if err != nil {
		return HandleSwipeOutcome{}, err
	}
	return HandleSwipeOutcome{Swipe: &SwipeOutput{
		PerformanceMode: int(service.ModeFromMetrics(metrics)),
		Metrics:         metrics,
	}}, nil
}

// performanceSnapshot assembles the read-only performance telemetry that
// accompanies an already-committed swipe. An infrastructure failure of the
// recent-swipe read degrades to the neutral empty-window snapshot (which
// ModeFromMetrics maps to service.ModeDefault) and is logged rather than
// returned, because reporting a durable swipe as failed makes the client
// re-queue the card and review it twice. Context cancellation still propagates
// unwrapped: the caller is being torn down and has nothing to report to.
func (u *swipeUsecase) performanceSnapshot(ctx context.Context, userID string, now time.Time) (service.PerformanceMetrics, error) {
	recentSwipes, err := u.swipeRepo.ListRecentByUser(ctx, userID, swipePerformanceSampleLimit)
	if err != nil {
		if isContextDone(err) {
			return service.PerformanceMetrics{}, err
		}
		logging.LogWarn(ctx, u.logger,
			"swipe committed but recent-swipe read failed; returning default performance snapshot",
			eris.Wrap(err, "usecase: swipe: list recent swipes"),
		)
		return service.ComputeMetrics(nil, now), nil
	}
	return service.ComputeMetrics(swipeRecordsByValue(recentSwipes), now), nil
}

// wrapSwipeErr passes a context cancellation through unwrapped and wraps any
// other error with the caller-supplied chain prefix.
func wrapSwipeErr(err error, msg string) error {
	return wrapInfraErr(err, msg)
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
