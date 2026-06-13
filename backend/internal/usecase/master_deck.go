package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// masterDeckCardgroupRepo is the subset of repository.MasterCardgroupRepository
// the master deck usecase consumes: locate a master template and enumerate the
// published default-starter set.
type masterDeckCardgroupRepo interface {
	FindByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	ListDefaultStarters(ctx context.Context) ([]*domain.MasterCardgroup, error)
}

// masterDeckCardRepo is the subset of repository.MasterCardRepository the master
// deck usecase consumes: read a master deck's cards to snapshot.
type masterDeckCardRepo interface {
	ListByMasterCardgroup(ctx context.Context, masterCardgroupID string) ([]*domain.MasterCard, error)
}

// masterDeckUserCardRepo is the subset of repository.CardRepository the master
// deck usecase consumes: bulk-insert the copied cards inside the caller's
// transaction.
type masterDeckUserCardRepo interface {
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error)
}

// masterDeckUserCardgroupRepo is the subset of repository.CardgroupRepository the
// master deck usecase consumes against the USER cardgroups table (not the master
// catalog). CountByOwner backs the idempotency guard: a user who already owns at
// least one cardgroup is not re-seeded on the next call. CreateTx inserts the
// snapshot cardgroup using the caller's transaction handle so the insert
// participates in the caller's transaction.
type masterDeckUserCardgroupRepo interface {
	CountByOwner(ctx context.Context, ownerID string, search *string) (int64, error)
	CreateTx(ctx context.Context, tx *gorm.DB, cg *domain.Cardgroup) error
}

// SeedForNewUserUsecase auto-provisions the published default-starter master
// decks into a newly-onboarded user's cardgroups. Wired into UserUsecase as an
// optional dependency: a nil seedUC disables the behaviour.
type SeedForNewUserUsecase interface {
	SeedForNewUser(ctx context.Context, userID string) error
}

// CopyMasterToUserUsecase snapshots a single master deck into a user-owned
// cardgroup. Used by the importMasterCardgroup mutation (via MasterCatalogUsecase.
// ImportMaster); the copy is a one-time snapshot with empty FSRS/swipe state.
type CopyMasterToUserUsecase interface {
	CopyMasterToUser(ctx context.Context, masterID, ownerID string) (*domain.Cardgroup, error)
}

// masterDeckUsecase implements both the public copy primitive and the
// seed-on-onboarding batch. copyMasterToUserTx is the shared tx-aware core; the
// two public entry points each open their own transaction and supply the caller
// wrap prefix.
type masterDeckUsecase struct {
	masterCG   masterDeckCardgroupRepo
	masterCard masterDeckCardRepo
	userCard   masterDeckUserCardRepo
	userCG     masterDeckUserCardgroupRepo
	tx         txRunner
	logger     *slog.Logger
}

// NewMasterDeckUsecase constructs a masterDeckUsecase for production use. db is
// the gorm handle used to open transactions; pass the same *gorm.DB used by the
// other usecase constructors. Panics when any dependency or the logger is nil.
func NewMasterDeckUsecase(
	masterCG masterDeckCardgroupRepo,
	masterCard masterDeckCardRepo,
	userCard masterDeckUserCardRepo,
	userCG masterDeckUserCardgroupRepo,
	db *gorm.DB,
	logger *slog.Logger,
) *masterDeckUsecase {
	if masterCG == nil {
		panic("usecase: master deck: masterCG is required")
	}
	if masterCard == nil {
		panic("usecase: master deck: masterCard is required")
	}
	if userCard == nil {
		panic("usecase: master deck: userCard is required")
	}
	if userCG == nil {
		panic("usecase: master deck: userCG is required")
	}
	if db == nil {
		panic("usecase: master deck: db is required")
	}
	if logger == nil {
		panic("usecase: master deck: logger is required")
	}
	return &masterDeckUsecase{
		masterCG:   masterCG,
		masterCard: masterCard,
		userCard:   userCard,
		userCG:     userCG,
		tx: func(ctx context.Context, fn func(tx *gorm.DB) error) error {
			return db.WithContext(ctx).Transaction(fn)
		},
		logger: logger,
	}
}

// NewMasterDeckUsecaseWithTx is the test-time constructor that injects an
// explicit transaction runner. Production callers must use NewMasterDeckUsecase.
// Panics when any dependency, the tx runner, or the logger is nil.
func NewMasterDeckUsecaseWithTx(
	masterCG masterDeckCardgroupRepo,
	masterCard masterDeckCardRepo,
	userCard masterDeckUserCardRepo,
	userCG masterDeckUserCardgroupRepo,
	tx txRunner,
	logger *slog.Logger,
) *masterDeckUsecase {
	if masterCG == nil {
		panic("usecase: master deck: masterCG is required")
	}
	if masterCard == nil {
		panic("usecase: master deck: masterCard is required")
	}
	if userCard == nil {
		panic("usecase: master deck: userCard is required")
	}
	if userCG == nil {
		panic("usecase: master deck: userCG is required")
	}
	if tx == nil {
		panic("usecase: master deck: tx runner is required")
	}
	if logger == nil {
		panic("usecase: master deck: logger is required")
	}
	return &masterDeckUsecase{
		masterCG:   masterCG,
		masterCard: masterCard,
		userCard:   userCard,
		userCG:     userCG,
		tx:         tx,
		logger:     logger,
	}
}

// CopyMasterToUser snapshots the master deck identified by masterID into a fresh
// cardgroup owned by ownerID, inside its own transaction. Returns the new
// cardgroup. The copy is independent of the source: later edits to the master
// deck do not propagate.
func (u *masterDeckUsecase) CopyMasterToUser(ctx context.Context, masterID, ownerID string) (*domain.Cardgroup, error) {
	var out *domain.Cardgroup
	if err := u.tx(ctx, func(tx *gorm.DB) error {
		cg, err := u.copyMasterToUserTx(ctx, tx, masterID, ownerID)
		if err != nil {
			return err
		}
		out = cg
		return nil
	}); err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master deck: copy master to user")
	}
	return out, nil
}

// SeedForNewUser copies every published default-starter master deck into the
// user's cardgroups. The whole batch runs in a single transaction guarded by a
// transaction-scoped advisory lock keyed on the user id so two concurrent seed
// attempts (e.g. a double onboarding submit) serialize. The idempotency guard
// short-circuits when the user already owns at least one cardgroup, so a retry
// after a partially-applied previous attempt does not double-seed.
func (u *masterDeckUsecase) SeedForNewUser(ctx context.Context, userID string) error {
	if err := u.tx(ctx, func(tx *gorm.DB) error {
		// hashtext returns int4; pg_advisory_xact_lock(0, hashtext(userID)) keys
		// the lock on the user within a fixed namespace so unrelated callers do
		// not contend. The lock releases automatically at transaction end.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(0, hashtext(?))", userID).Error; err != nil {
			return eris.Wrap(err, "usecase: master deck: seed for new user: advisory lock")
		}

		count, err := u.userCG.CountByOwner(ctx, userID, nil)
		if err != nil {
			return eris.Wrap(err, "usecase: master deck: seed for new user: count owner cardgroups")
		}
		if count > 0 {
			// Already seeded (or the user created their own cardgroup): no-op.
			return nil
		}

		starters, err := u.masterCG.ListDefaultStarters(ctx)
		if err != nil {
			return eris.Wrap(err, "usecase: master deck: seed for new user: list default starters")
		}
		for _, m := range starters {
			// The helper returns its error bare; the outer tx-return wrap below
			// applies the single "seed for new user" prefix, so wrapping here
			// would duplicate that frame in the error chain.
			if _, err := u.copyMasterToUserTx(ctx, tx, m.ID, userID); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if isContextDone(err) {
			return err
		}
		return eris.Wrap(err, "usecase: master deck: seed for new user")
	}
	return nil
}

// copyMasterToUserTx is the shared tx-aware copy core used by both
// CopyMasterToUser (single import) and SeedForNewUser (batch). It MUST NOT embed
// a fixed eris layer prefix: the two public callers each apply their own wrap
// prefix so the logged error_chain attributes the failure to the calling
// operation rather than this helper.
//
// The new cardgroup row is inserted via the cardgroup repository's CreateTx with
// the supplied tx handle so the insert participates in the caller's transaction.
// Cards are deep-copied with fresh ids and the new cardgroup id; FSRS/swipe
// state is left empty by construction (no rows are written to the per-user FSRS
// table).
func (u *masterDeckUsecase) copyMasterToUserTx(ctx context.Context, tx *gorm.DB, masterID, ownerID string) (*domain.Cardgroup, error) {
	master, err := u.masterCG.FindByID(ctx, masterID)
	if err != nil {
		return nil, eris.Wrap(err, "find master cardgroup")
	}

	cards, err := u.masterCard.ListByMasterCardgroup(ctx, masterID)
	if err != nil {
		return nil, eris.Wrap(err, "list master cards")
	}

	newCGID, err := domain.NewID()
	if err != nil {
		return nil, eris.Wrap(err, "new cardgroup id")
	}

	now := time.Now().UTC()
	newCG := &domain.Cardgroup{
		ID:        newCGID,
		OwnerID:   ownerID,
		Name:      master.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := u.userCG.CreateTx(ctx, tx, newCG); err != nil {
		return nil, eris.Wrap(err, "create user cardgroup")
	}

	userCards := make([]*domain.Card, 0, len(cards))
	for _, mc := range cards {
		cardID, err := domain.NewID()
		if err != nil {
			return nil, eris.Wrap(err, "new card id")
		}
		userCards = append(userCards, &domain.Card{
			ID:          cardID,
			CardgroupID: newCGID,
			Front:       mc.Front,
			Back:        mc.Back,
			Position:    mc.Position,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	if _, err := u.userCard.UpsertManyTx(ctx, tx, userCards); err != nil {
		return nil, eris.Wrap(err, "upsert user cards")
	}

	return newCG, nil
}
