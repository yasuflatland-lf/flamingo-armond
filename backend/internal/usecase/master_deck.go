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
// catalog). FindByID satisfies CardgroupOwnershipFinder so the ownership gate can
// use u.userCG directly; it is also called post-merge to read the destination
// cardgroup back after the transaction commits. CountByOwner backs the idempotency
// guard: a user who already owns at least one cardgroup is not re-seeded on the
// next call. CreateTx inserts the snapshot cardgroup using the caller's transaction
// handle so the insert participates in the caller's transaction.
type masterDeckUserCardgroupRepo interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
	CountByOwner(ctx context.Context, ownerID string, search *string) (int64, error)
	CreateTx(ctx context.Context, tx *gorm.DB, cg *domain.Cardgroup) error
}

// SeedForNewUserUsecase provisions the published default-starter master decks
// into a user's cardgroups. It is invoked on demand by MasterCatalogUsecase.
// SeedDefaultStarters (behind the seedDefaultStarterCardgroups mutation) when a
// user takes the onboarding chooser's "start with the default decks" path —
// not automatically on onboarding completion. Returns the cardgroups created by
// this call (empty when no default starters exist, or when the idempotency guard
// short-circuits because the caller already owns a cardgroup).
type SeedForNewUserUsecase interface {
	SeedForNewUser(ctx context.Context, userID string) ([]*domain.Cardgroup, error)
}

// CopyMasterToUserUsecase snapshots a single master deck into a user-owned
// cardgroup. Used by the importMasterCardgroup mutation (via MasterCatalogUsecase.
// ImportMaster); the copy is a one-time snapshot with empty FSRS/swipe state.
type CopyMasterToUserUsecase interface {
	CopyMasterToUser(ctx context.Context, masterID, ownerID string) (*domain.Cardgroup, error)
}

// MergeMasterResult is the tally returned by MergeMasterIntoCardgroup: the
// destination cardgroup plus the insert/update counts from the upsert.
type MergeMasterResult struct {
	// Non-nil when the returned error is nil.
	Cardgroup *domain.Cardgroup
	Added     int64
	Updated   int64
}

// MergeMasterIntoCardgroupUsecase merges a master deck's cards into an existing
// caller-owned cardgroup. Used by mergeMasterCardgroup via MasterCatalogUsecase.
// MergeMaster. The merge is a one-time snapshot (no live link); re-merging the
// same deck re-syncs the destination to the current catalog content.
type MergeMasterIntoCardgroupUsecase interface {
	MergeMasterIntoCardgroup(ctx context.Context, masterID string, destCardgroupID domain.CardgroupID, ownerID domain.UserID) (*MergeMasterResult, error)
}

// masterDeckUsecase implements three public entry points: CopyMasterToUser (single
// import), SeedForNewUser (default-starter batch), and MergeMasterIntoCardgroup
// (merge into an existing caller-owned cardgroup). copyMasterToUserTx is the
// shared tx-aware core for CopyMasterToUser and SeedForNewUser; MergeMasterIntoCardgroup
// uses the lower-level copyMasterCardsIntoTx directly.
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
		tx:         newTxRunner(db),
		logger:     logger,
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
// after a partially-applied previous attempt does not double-seed. Returns the
// cardgroups created by this call (empty when no defaults exist or the guard
// short-circuits).
func (u *masterDeckUsecase) SeedForNewUser(ctx context.Context, userID string) ([]*domain.Cardgroup, error) {
	// Non-nil empty slice so the no-op / no-defaults paths return a consistent
	// empty container rather than nil (matches the repo's empty-return symmetry
	// convention; callers receive `[]` regardless of which path fired).
	seeded := []*domain.Cardgroup{}
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
			cg, err := u.copyMasterToUserTx(ctx, tx, m.ID, userID)
			if err != nil {
				return err
			}
			seeded = append(seeded, cg)
		}
		return nil
	}); err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master deck: seed for new user")
	}
	return seeded, nil
}

// copyMasterCardsIntoTx deep-copies the supplied master cards into the
// destination cardgroup with a fresh id and upserts them on (cardgroup_id, front)
// inside the caller's transaction. Callers list the master cards first and pass
// them in, so this helper stays free of the listing step and the caller controls
// the operation order. pin, when non-nil, overrides every copied card's
// CreatedAt/UpdatedAt (import pins to the new cardgroup's CreatedAt for a
// consistent batch timestamp; merge passes nil and keeps NewCard's now()).
// Returns the insert/update tally. MUST NOT embed a fixed eris layer prefix — the
// public callers apply their own wrap so the error_chain attributes the failure
// to the calling operation.
func (u *masterDeckUsecase) copyMasterCardsIntoTx(
	ctx context.Context, tx *gorm.DB, masterCards []*domain.MasterCard, destCG domain.CardgroupID, pin *time.Time,
) (repository.UpsertManyTxResult, error) {
	userCards := make([]*domain.Card, 0, len(masterCards))
	for _, mc := range masterCards {
		card, err := domain.NewCard(destCG, mc.Front.String(), mc.Back.String(), mc.Position)
		if err != nil {
			return repository.UpsertManyTxResult{}, eris.Wrap(err, "new card from master")
		}
		if pin != nil {
			card.CreatedAt = *pin
			card.UpdatedAt = *pin
		}
		userCards = append(userCards, card)
	}
	return u.userCard.UpsertManyTx(ctx, tx, userCards)
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

	newCG, err := domain.NewCardgroup(domain.UserID(ownerID), master.Name)
	if err != nil {
		return nil, eris.Wrap(err, "new cardgroup")
	}
	if err := u.userCG.CreateTx(ctx, tx, newCG); err != nil {
		return nil, eris.Wrap(err, "create user cardgroup")
	}

	pin := newCG.CreatedAt
	if _, err := u.copyMasterCardsIntoTx(ctx, tx, cards, newCG.ID, &pin); err != nil {
		return nil, err
	}

	return newCG, nil
}

// MergeMasterIntoCardgroup copies the master deck's cards into destCardgroupID,
// which ownerID must own, inside its own transaction. The destination ownership
// gate uses authorizeCardgroupOrBadInput (untrusted-input boundary): an unknown
// cardgroup is a recoverable validation error; a foreign cardgroup is
// UNAUTHENTICATED. Cards conflicting on (cardgroup_id, front) are overwritten
// (back/position/updated_at); ids are preserved so FSRS state survives. Returns the
// destination cardgroup plus the add/update tally.
func (u *masterDeckUsecase) MergeMasterIntoCardgroup(
	ctx context.Context, masterID string, destCardgroupID domain.CardgroupID, ownerID domain.UserID,
) (*MergeMasterResult, error) {
	// Ownership gate runs OUTSIDE the tx and returns its ucerr typed error
	// directly — mirrors card_import.go Import, which gates ownership before the
	// tx and returns the ucerr unwrapped. gqlerr.FromUsecaseError classifies via
	// errors.Is(err, ucerr.ErrUnauthenticated) and errors.AsType[*ucerr.ValidationError],
	// both of which walk the eris chain, so this needs no wrap and the tx error
	// path below needs no ucerr pass-through guard.
	if err := authorizeCardgroupOrBadInput(ctx, u.userCG, destCardgroupID, ownerID); err != nil {
		return nil, err
	}

	var res repository.UpsertManyTxResult
	if err := u.tx(ctx, func(tx *gorm.DB) error {
		cards, err := u.masterCard.ListByMasterCardgroup(ctx, masterID)
		if err != nil {
			return eris.Wrap(err, "usecase: master deck: merge master into cardgroup: list master cards")
		}
		r, err := u.copyMasterCardsIntoTx(ctx, tx, cards, destCardgroupID, nil)
		if err != nil {
			return err
		}
		res = r
		return nil
	}); err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master deck: merge master into cardgroup")
	}

	cg, err := u.userCG.FindByID(ctx, string(destCardgroupID))
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master deck: merge master into cardgroup: find destination")
	}
	return &MergeMasterResult{Cardgroup: cg, Added: res.Inserted, Updated: res.Updated}, nil
}
