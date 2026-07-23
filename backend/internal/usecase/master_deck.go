package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// masterDeckCardgroupRepo is the subset of repository.MasterCardgroupRepository
// the master deck usecase consumes: locate a catalog-visible master template and
// enumerate the catalog-visible default-starter set. Catalog visibility is
// `status = published AND at least one master card exists`, so a deck emptied by
// an admin or a Notion sync prune is as invisible as a draft. The lookups are
// deliberately catalog-scoped (FindPublishedByID / FindPublishedByIDTx, not the
// any-status FindByID): every path re-probes the master through the same
// visibility gate that MasterCatalogUsecase applies before it delegates, so an
// unpublish landing after that gate is caught rather than snapshotted.
//
// The two lookups differ only in where they run. The write paths take
// FindPublishedByIDTx, which reads on the transaction connection and locks the
// master row FOR SHARE, so a concurrent unpublish blocks until the write's
// transaction ends — the unpublish window is closed, not merely narrowed. The
// read-only preview has no transaction to align with and takes the pooled
// FindPublishedByID; it verifies the same visibility so preview and merge reject
// an unpublished deck identically. The emptiness half needs no lock on either
// path: it is decided by len(cards) on the enumeration each path consumes, which
// no interleaving can defeat.
//
// Unknown, unpublished and empty all collapse to repository.ErrNotFound, which
// the catalog layer maps to the same non-disclosure not-found outcome and
// SeedForNewUser treats as "skip this starter".
type masterDeckCardgroupRepo interface {
	FindPublishedByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	FindPublishedByIDTx(ctx context.Context, tx *gorm.DB, id string) (*domain.MasterCardgroup, error)
	ListPublishedDefaultStarters(ctx context.Context) ([]*domain.MasterCardgroup, error)
}

// masterDeckCardRepo is the subset of repository.MasterCardRepository the master
// deck usecase consumes: read a master deck's cards to snapshot.
type masterDeckCardRepo interface {
	ListByMasterCardgroup(ctx context.Context, masterCardgroupID string) ([]*domain.MasterCard, error)
}

// masterDeckUserCardRepo is the subset of repository.CardRepository the master
// deck usecase consumes: fold case variants before a merge upsert, bulk-upsert
// copied cards inside the caller's transaction, and count folded matches for
// the read-only preview path.
type masterDeckUserCardRepo interface {
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error)
	FoldFrontCaseToTx(ctx context.Context, tx *gorm.DB, cardgroupID string, fronts []string) (int64, error)
	CountMatchingFrontsFold(ctx context.Context, cardgroupID string, loweredFronts []string) (int64, error)
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
	// AcquireUserSeedLockTx takes a per-user transaction-scoped advisory lock so
	// two concurrent seed-for-new-user calls for the same user serialize. The
	// dialect detail (the advisory-lock SQL) lives in the repository.
	AcquireUserSeedLockTx(ctx context.Context, tx *gorm.DB, userID string) error
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

// PreviewMergeResult is the projected tally of a merge computed without writing.
type PreviewMergeResult struct {
	Added   int64
	Updated int64
}

// PreviewMergeMasterIntoCardgroupUsecase computes the projected add/update tally
// of merging a master deck into a destination cardgroup, without mutating.
type PreviewMergeMasterIntoCardgroupUsecase interface {
	PreviewMergeMasterIntoCardgroup(ctx context.Context, masterID string, destCardgroupID domain.CardgroupID, ownerID domain.UserID) (PreviewMergeResult, error)
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

// newMasterDeckUsecaseWithTx is the test-time constructor that injects an
// explicit transaction runner. Production callers must use NewMasterDeckUsecase.
// Panics when any dependency, the tx runner, or the logger is nil.
func newMasterDeckUsecaseWithTx(
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
		if translated := translateTextLengthViolation(err); translated != nil {
			return nil, translated
		}
		return nil, eris.Wrap(err, "usecase: master deck: copy master to user")
	}
	return out, nil
}

// SeedForNewUser copies every catalog-visible default-starter master deck into
// the user's cardgroups. Starters that are published but hold zero cards are
// skipped by ListPublishedDefaultStarters, so the learner is never seeded with
// an empty deck; when every starter is empty nothing is created and the user's
// owned-deck count stays 0, leaving the next seed attempt free to run normally.
// The whole batch runs in a single transaction guarded by a
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
		// Take a per-user transaction-scoped advisory lock so two concurrent seed
		// attempts for the same user serialize. The advisory-lock SQL (a Postgres
		// dialect detail) lives in the repository; the lock releases at tx end.
		if err := u.userCG.AcquireUserSeedLockTx(ctx, tx, userID); err != nil {
			return err
		}

		count, err := u.userCG.CountByOwner(ctx, userID, nil)
		if err != nil {
			return eris.Wrap(err, "usecase: master deck: seed for new user: count owner cardgroups")
		}
		if count > 0 {
			// Already seeded (or the user created their own cardgroup): no-op.
			// This guard is also why seeding needs no explicit cardgroup-quota
			// check: it only ever runs for an owner holding zero cardgroups, and
			// the published default-starter set is admin-curated and small.
			return nil
		}

		starters, err := u.masterCG.ListPublishedDefaultStarters(ctx)
		if err != nil {
			return eris.Wrap(err, "usecase: master deck: seed for new user: list default starters")
		}
		for _, m := range starters {
			// The helper returns its error bare; the outer tx-return wrap below
			// applies the single "seed for new user" prefix, so wrapping here
			// would duplicate that frame in the error chain.
			cg, err := u.copyMasterToUserTx(ctx, tx, m.ID, userID)
			if errors.Is(err, repository.ErrNotFound) {
				// The starter left the catalog between ListPublishedDefaultStarters
				// and its copy — unpublished, deleted, or emptied of its last card.
				// Skip it and seed the rest: one starter losing visibility mid-signup
				// must not fail the whole seed, and the guarantee this protects is
				// "never seed an empty deck", not "seed every listed starter".
				continue
			}
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
		if translated := translateTextLengthViolation(err); translated != nil {
			return nil, translated
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
// CreatedAt (import pins it to the new cardgroup's CreatedAt; merge passes nil
// and keeps the constructor's time). The database assigns updated_at uniformly
// from the transaction timestamp. Returns the insert/update tally. MUST NOT
// embed a fixed eris layer prefix — the
// public callers apply their own wrap so the error_chain attributes the failure
// to the calling operation.
func (u *masterDeckUsecase) copyMasterCardsIntoTx(
	ctx context.Context, tx *gorm.DB, masterCards []*domain.MasterCard, destCG domain.CardgroupID, pin *time.Time,
) (repository.UpsertManyTxResult, error) {
	userCards := make([]*domain.Card, 0, len(masterCards))
	for _, mc := range masterCards {
		card, err := domain.NewCardFromValidated(destCG, mc.Front, mc.Back, mc.Position)
		if err != nil {
			return repository.UpsertManyTxResult{}, eris.Wrap(err, "new card from master")
		}
		if pin != nil {
			card.CreatedAt = *pin
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
// The master is re-read through the catalog-scoped FindPublishedByIDTx (not the
// any-status FindByID), so a master unpublished between MasterCatalogUsecase's
// gate and this copy yields repository.ErrNotFound and no rows are written
// rather than silently snapshotting a draft.
//
// That probe CLOSES the unpublish window rather than narrowing it: it runs on
// the tx handle and locks the master row FOR SHARE, so an unpublish that has
// already committed is seen by the probe, and one that has not blocks until this
// transaction ends. Either way no draft deck is ever snapshotted.
//
// The emptiness half of catalog visibility needs no lock: it is decided by
// len(cards) on the enumeration this copy consumes — see the guard at its call
// site — which holds no matter how the reads interleave with a concurrent
// last-card delete. The enumeration therefore stays on the pooled connection.
//
// SeedForNewUser tolerates that ErrNotFound by skipping the starter, since one
// deck leaving the catalog mid-signup must not fail the whole seed.
//
// The new cardgroup row is inserted via the cardgroup repository's CreateTx with
// the supplied tx handle so the insert participates in the caller's transaction.
// Cards are deep-copied with fresh ids and the new cardgroup id; FSRS/swipe
// state is left empty by construction (no rows are written to the per-user FSRS
// table).
func (u *masterDeckUsecase) copyMasterToUserTx(ctx context.Context, tx *gorm.DB, masterID, ownerID string) (*domain.Cardgroup, error) {
	master, err := u.masterCG.FindPublishedByIDTx(ctx, tx, masterID)
	if err != nil {
		return nil, eris.Wrap(err, "find published master cardgroup")
	}

	cards, err := u.masterCard.ListByMasterCardgroup(ctx, masterID)
	if err != nil {
		return nil, eris.Wrap(err, "list master cards")
	}
	// Emptiness is decided by the enumeration this copy actually consumes, not
	// by the FindPublishedByIDTx probe above. The probe's FOR SHARE lock covers
	// the master row only, so a last-card delete landing between the two reads
	// would otherwise pass the catalog gate and then write a cardgroup with no
	// cards. Deriving the verdict from `cards` makes an empty import unreachable
	// regardless of how the two reads interleave, without locking master_cards.
	if len(cards) == 0 {
		return nil, eris.Wrap(repository.ErrNotFound, "master cardgroup holds no cards")
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
// UNAUTHENTICATED. The master is re-read through the catalog-scoped
// FindPublishedByIDTx before its cards are listed, so a master unpublished
// between MasterCatalogUsecase's gate and this write yields
// repository.ErrNotFound (which MergeMaster maps to the not-found outcome)
// rather than snapshotting a draft. That probe CLOSES the unpublish window: it
// runs on this transaction's connection and locks the master row FOR SHARE, so
// an unpublish either committed before it (and is seen) or blocks until this
// transaction ends. Emptiness needs no such lock: it
// is decided by len(cards) on the enumeration this merge consumes, so a deck
// that loses its last card can never be merged as a successful 0/0. Cards
// conflicting on (cardgroup_id, front) have back and position overwritten, and
// the database advances updated_at; ids are preserved so FSRS state survives. Returns
// the destination cardgroup plus the add/update tally.
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
		// Re-probe the master through the catalog-scoped method so an unpublish
		// landing after MasterCatalogUsecase.MergeMaster's FindPublishedByID gate
		// is caught (TOCTOU). The probe runs on this transaction's connection and
		// locks the master row FOR SHARE, so the window is closed rather than
		// narrowed: an unpublish that committed first is visible here, and one that
		// arrives later blocks on the lock until this transaction ends. The
		// emptiness half needs no lock — it is closed just below, off the
		// enumeration this merge consumes.
		// ErrNotFound (unknown, unpublished or empty) travels up the eris chain;
		// MergeMaster collapses it into the same non-disclosure not-found outcome
		// as a pre-gate unknown/draft/empty master.
		if _, err := u.masterCG.FindPublishedByIDTx(ctx, tx, masterID); err != nil {
			return eris.Wrap(err, "usecase: master deck: merge master into cardgroup: verify published master")
		}
		cards, err := u.masterCard.ListByMasterCardgroup(ctx, masterID)
		if err != nil {
			return eris.Wrap(err, "usecase: master deck: merge master into cardgroup: list master cards")
		}
		// Same reasoning as copyMasterToUserTx: the verdict comes from the
		// enumeration this merge consumes, so a deck emptied between the probe
		// above and this read collapses to not-found instead of reporting a
		// successful 0/0 merge against a deck that has left the catalog.
		if len(cards) == 0 {
			return eris.Wrap(repository.ErrNotFound, "usecase: master deck: merge master into cardgroup: master cardgroup holds no cards")
		}
		fronts := make([]string, len(cards))
		for i, card := range cards {
			fronts[i] = string(card.Front)
		}
		if _, err := u.userCard.FoldFrontCaseToTx(ctx, tx, string(destCardgroupID), fronts); err != nil {
			if isContextDone(err) {
				return err
			}
			return eris.Wrap(err, "usecase: master deck: merge master into cardgroup: fold case variants")
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
		if translated := translateTextLengthViolation(err); translated != nil {
			return nil, translated
		}
		return nil, eris.Wrap(err, "usecase: master deck: merge master into cardgroup")
	}

	cg, err := u.userCG.FindByID(ctx, string(destCardgroupID))
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		if errors.Is(err, repository.ErrNotFound) {
			// The merge already committed; no FOR UPDATE is held on the destination row
			// (the card upsert takes only an FK FOR KEY SHARE, released at commit), so a
			// concurrent delete of the destination cardgroup can land here. Translate the
			// sentinel into a plain internal error: letting repository.ErrNotFound travel
			// up would make MergeMaster's master-scoped not-found branch report the
			// *catalog* deck as missing, hiding both the committed merge and the deleted
			// destination.
			return nil, eris.New("usecase: master deck: merge master into cardgroup: destination cardgroup vanished after merge commit")
		}
		return nil, eris.Wrap(err, "usecase: master deck: merge master into cardgroup: find destination")
	}
	return &MergeMasterResult{Cardgroup: cg, Added: res.Inserted, Updated: res.Updated}, nil
}

// PreviewMergeMasterIntoCardgroup mirrors MergeMasterIntoCardgroup as a read-only
// dry run: it runs the same ownership gate, then counts how many of the master
// deck's fronts already exist in the destination after case-folding, matching
// the merge's in-transaction case-variant rename before its exact-front upsert.
// Added + Updated equals the master deck's card count.
//
// It mirrors MergeMasterIntoCardgroup's published re-verification so the dry run
// and the write it previews reject an unpublished deck identically: master cards
// outlive an unpublish, so without this probe the preview would report a tally
// for a deck the follow-up merge refuses. The probe is the pooled
// FindPublishedByID, not the FOR SHARE FindPublishedByIDTx — a read-only dry run
// opens no transaction, so it has no write to serialise an unpublish against and
// must not hold a lock across a user's think-time. An unpublish landing between
// the preview response and the confirm therefore still turns into a not-found on
// merge; that gap spans two requests and no lock can reach it.
func (u *masterDeckUsecase) PreviewMergeMasterIntoCardgroup(
	ctx context.Context, masterID string, destCardgroupID domain.CardgroupID, ownerID domain.UserID,
) (PreviewMergeResult, error) {
	if err := authorizeCardgroupOrBadInput(ctx, u.userCG, destCardgroupID, ownerID); err != nil {
		return PreviewMergeResult{}, err
	}

	// ErrNotFound (unknown, unpublished or empty) travels up the eris chain;
	// PreviewMergeMaster collapses it into the same non-disclosure not-found
	// outcome MergeMaster returns for the identical state.
	if _, err := u.masterCG.FindPublishedByID(ctx, masterID); err != nil {
		if isContextDone(err) {
			return PreviewMergeResult{}, err
		}
		return PreviewMergeResult{}, eris.Wrap(err, "usecase: master deck: preview merge: verify published master")
	}

	cards, err := u.masterCard.ListByMasterCardgroup(ctx, masterID)
	if err != nil {
		if isContextDone(err) {
			return PreviewMergeResult{}, err
		}
		return PreviewMergeResult{}, eris.Wrap(err, "usecase: master deck: preview merge: list master cards")
	}

	fronts := make([]string, len(cards))
	for i, c := range cards {
		fronts[i] = strings.ToLower(string(c.Front))
	}

	overlap, err := u.userCard.CountMatchingFrontsFold(ctx, string(destCardgroupID), fronts)
	if err != nil {
		if isContextDone(err) {
			return PreviewMergeResult{}, err
		}
		return PreviewMergeResult{}, eris.Wrap(err, "usecase: master deck: preview merge: count matching fronts fold")
	}

	total := int64(len(cards))
	return PreviewMergeResult{Added: total - overlap, Updated: overlap}, nil
}
