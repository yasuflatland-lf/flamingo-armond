// master_catalog_import.go holds the learner-consumption surface of the master
// catalog: the outcome carrier types and the methods that copy a published master
// into caller-owned cardgroups (import, merge, preview merge, seed default
// starters). Import, merge and preview merge route the caller-supplied master id
// through verifyPublishedMaster, so an unknown id, a DRAFT id and a published id
// holding zero cards all collapse to the same not-found outcome and neither draft
// existence nor an empty deck is ever disclosed; seed default starters takes no
// caller-supplied id and is catalog-scoped inside the delegated SeedForNewUser.

package usecase

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// MergeMasterOutcome is the usecase result of MergeMaster. On the valid paths
// exactly one outcome is active: the happy path sets Cardgroup with the Added/Updated
// tallies and leaves NotFound false; the not-found path sets NotFound=true and leaves
// Cardgroup nil with zero tallies. Destination cardgroup auth failures are returned as
// errors, not via this outcome.
type MergeMasterOutcome struct {
	// Cardgroup is the caller-owned destination after the merge. Non-nil iff NotFound is false.
	Cardgroup *domain.Cardgroup
	// Added is the number of cards newly inserted into the destination.
	Added int64
	// Updated is the number of existing cards (same front) overwritten.
	Updated int64
	// NotFound is true when the master id is unknown, not published, or published
	// with zero cards; draft existence and emptiness are subsumed so those ids are
	// indistinguishable from absent ids. True iff Cardgroup is nil. The XOR is a
	// producer contract, not a compile-time guarantee: a degenerate
	// {Cardgroup:nil, NotFound:false} result is treated as INTERNAL by the resolver's
	// defensive guard (newNoVariantSetError).
	NotFound bool
}

// PreviewMergeOutcome is the usecase result of PreviewMergeMaster. On the valid
// path Added/Updated carry the projected tally and NotFound is false; the not-found
// path sets NotFound=true with zero tallies. Destination auth failures are returned
// as errors, not via this outcome.
type PreviewMergeOutcome struct {
	Added    int64
	Updated  int64
	NotFound bool
}

// ImportMasterOutcome is the usecase result of ImportMaster. On the valid paths
// exactly one signal is set: Cardgroup on the happy path, NotFound=true when the
// master id is unknown, not published, or published with zero cards, or
// LimitReached when a non-admin caller already owns the maximum number of
// cardgroups. Both failure cases are surfaced
// as data (the MasterNotFoundError / CardgroupLimitReachedError union variants)
// rather than as errors so the resolver can return them in `data`. The XOR is a
// producer contract, not a compile-time guarantee: a degenerate
// {Cardgroup:nil, NotFound:false, LimitReached:nil} result is treated as INTERNAL
// by the resolver's defensive guard.
type ImportMasterOutcome struct {
	// Cardgroup is the newly created user-owned cardgroup snapshot on the happy
	// path. Non-nil iff neither NotFound nor LimitReached is set.
	Cardgroup *domain.Cardgroup
	// NotFound is true when the master id is unknown, not published, or published
	// with zero cards; it subsumes draft existence and emptiness so those ids are
	// indistinguishable from absent ids.
	NotFound bool
	// LimitReached is non-nil when the caller is a non-admin who already holds
	// domain.GeneralUserCardgroupLimit cardgroups. It carries the same cap/count
	// pair as CreateCardgroupOutcome.LimitReached so both entry points into "the
	// caller now owns a new deck" surface the quota identically.
	LimitReached *CardgroupLimitInfo
}

// ImportMaster copies the published master cardgroup identified by masterID into a
// fresh cardgroup owned by the authenticated caller. The master is gated through
// FindPublishedByID, which returns ErrNotFound for unknown ids, draft decks and
// published decks holding zero cards, so neither draft existence nor an empty
// deck is ever disclosed — all three collapse to ImportMasterOutcome{NotFound:true}
// and no cardgroup row is written. The delegated copy re-reads the master
// through the same catalog-scoped method inside its transaction, so a master
// unpublished — or emptied of its last card — between this gate and the write
// also collapses to NotFound (the ErrNotFound the copy surfaces is mapped below)
// rather than silently importing a now-invisible deck. Unauthenticated callers
// receive ucerr.ErrUnauthenticated. Import is the second entry point into "the caller now
// owns a new cardgroup", so it applies the same per-user cardgroup quota as
// CardgroupUsecase.Create via checkCardgroupLimit (admins exempt) — without it the
// cap would be a property of the create form rather than an invariant of the
// system. The copy is a one-time snapshot delegated to CopyMasterToUserUsecase;
// FSRS/swipe state starts empty.
func (u *masterCatalogUsecase) ImportMaster(ctx context.Context, masterID string) (ImportMasterOutcome, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return ImportMasterOutcome{}, err
	}

	_, notFound, err := u.verifyPublishedMaster(ctx, masterID, "usecase: master catalog: import: verify published")
	if err != nil {
		return ImportMasterOutcome{}, err
	}
	if notFound {
		return ImportMasterOutcome{NotFound: true}, nil
	}

	// The quota runs after the published gate so a capped caller probing an
	// unknown id still gets the non-disclosure not-found outcome, and before the
	// copy so no cardgroup row is ever written for a rejected import.
	limit, err := checkCardgroupLimit(ctx, u.cgCounter, u.adminGate, caller.Sub)
	if err != nil {
		return ImportMasterOutcome{}, err
	}
	if limit != nil {
		return ImportMasterOutcome{LimitReached: limit}, nil
	}

	cg, err := u.deckUC.CopyMasterToUser(ctx, masterID, caller.Sub)
	if err != nil {
		if isContextDone(err) {
			return ImportMasterOutcome{}, err
		}
		// The owner FK no longer resolves: the caller's account was deleted while
		// their JWT was still valid. Surface UNAUTHENTICATED so the client signs
		// them out instead of paging an operator with an INTERNAL error. Checked
		// ahead of the ErrNotFound branch below because the two are distinct
		// standalone sentinels — a missing owner is not an unpublished master.
		if errors.Is(err, repository.ErrCardgroupOwnerNotFound) {
			return ImportMasterOutcome{}, ucerr.ErrUnauthenticated
		}
		if errors.Is(err, repository.ErrNotFound) {
			// The master was unpublished, or lost its last card, between the
			// FindPublishedByID gate and the copy's own catalog-scoped re-read
			// (TOCTOU). Collapse into the same non-disclosure not-found outcome as
			// a pre-gate unknown/draft/empty master.
			return ImportMasterOutcome{NotFound: true}, nil
		}
		return ImportMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: import: copy master to user")
	}
	return ImportMasterOutcome{Cardgroup: cg}, nil
}

// MergeMaster merges the published master cardgroup identified by masterID into
// the caller-owned cardgroup cardgroupID. The master is gated through
// FindPublishedByID, collapsing unknown, draft and card-less decks into
// MergeMasterOutcome{NotFound:true} so neither draft existence nor an empty deck
// is ever disclosed. The delegated merge re-reads the master through the same
// catalog-scoped method inside its transaction, so a master unpublished or
// emptied between this gate and the write also collapses to NotFound (the
// ErrNotFound the merge surfaces is mapped below) rather than snapshotting a
// now-invisible deck. Destination ownership is enforced by the delegated usecase
// (BAD_USER_INPUT for unknown, UNAUTHENTICATED for foreign), surfaced as an error
// rather than via the outcome. Unauthenticated callers receive
// ucerr.ErrUnauthenticated. The merge is a one-time snapshot.
func (u *masterCatalogUsecase) MergeMaster(ctx context.Context, masterID, cardgroupID string) (MergeMasterOutcome, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return MergeMasterOutcome{}, err
	}

	_, notFound, err := u.verifyPublishedMaster(ctx, masterID, "usecase: master catalog: merge: verify published")
	if err != nil {
		return MergeMasterOutcome{}, err
	}
	if notFound {
		return MergeMasterOutcome{NotFound: true}, nil
	}

	res, err := u.deckUC.MergeMasterIntoCardgroup(ctx, masterID, domain.CardgroupID(cardgroupID), domain.UserID(caller.Sub))
	if err != nil {
		if isContextDone(err) {
			return MergeMasterOutcome{}, err
		}
		if errors.Is(err, repository.ErrNotFound) {
			// The master was unpublished, or lost its last card, between the
			// FindPublishedByID gate and the merge tx's own catalog-scoped re-read
			// (TOCTOU). Collapse into the same non-disclosure not-found outcome as a
			// pre-gate unknown/draft/empty master. That in-tx re-read is the only
			// ErrNotFound producer this branch can see: the
			// destination ownership gate maps a missing cardgroup to a
			// ucerr.ValidationError, and the post-commit destination read-back translates
			// its ErrNotFound into a non-sentinel internal error so a destination deleted
			// mid-merge is never reported as a missing master.
			return MergeMasterOutcome{NotFound: true}, nil
		}
		// Wrap unconditionally, exactly like ImportMaster wraps CopyMasterToUser.
		// A ucerr.ValidationError / ucerr.ErrUnauthenticated from the delegated
		// ownership gate is still classified correctly because FromUsecaseError
		// walks the eris chain (errors.Is / errors.AsType). No pass-through guard.
		return MergeMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: merge: merge master into cardgroup")
	}
	return MergeMasterOutcome{Cardgroup: res.Cardgroup, Added: res.Added, Updated: res.Updated}, nil
}

// PreviewMergeMaster mirrors MergeMaster as a read-only dry run. Same gates:
// unauthenticated -> ErrUnauthenticated; unknown/draft/card-less master -> NotFound
// (collapsed via FindPublishedByID, never disclosing draft existence); destination
// auth failures travel as errors from the delegated usecase.
func (u *masterCatalogUsecase) PreviewMergeMaster(ctx context.Context, masterID, cardgroupID string) (PreviewMergeOutcome, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return PreviewMergeOutcome{}, err
	}

	_, notFound, err := u.verifyPublishedMaster(ctx, masterID, "usecase: master catalog: preview merge: verify published")
	if err != nil {
		return PreviewMergeOutcome{}, err
	}
	if notFound {
		return PreviewMergeOutcome{NotFound: true}, nil
	}

	res, err := u.deckUC.PreviewMergeMasterIntoCardgroup(ctx, masterID, domain.CardgroupID(cardgroupID), domain.UserID(caller.Sub))
	if err != nil {
		if isContextDone(err) {
			return PreviewMergeOutcome{}, err
		}
		return PreviewMergeOutcome{}, eris.Wrap(err, "usecase: master catalog: preview merge: preview merge into cardgroup")
	}
	return PreviewMergeOutcome{Added: res.Added, Updated: res.Updated}, nil
}

// SeedDefaultStarters copies the published, non-empty default-starter master
// decks into the authenticated caller's own cardgroups (idempotent — a no-op if
// the caller already owns a cardgroup). A starter that is published but holds
// zero cards is skipped by ListPublishedDefaultStarters, so a new user is never
// seeded with an empty deck. Unauthenticated callers receive ucerr.ErrUnauthenticated.
func (u *masterCatalogUsecase) SeedDefaultStarters(ctx context.Context) ([]*domain.Cardgroup, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return nil, err
	}
	seeded, err := u.deckUC.SeedForNewUser(ctx, caller.Sub)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		// Same deleted-account path as ImportMaster: the seed writes cardgroups
		// owned by the caller, so an unresolvable owner FK means the account is
		// gone and the client must sign out rather than page an operator.
		if errors.Is(err, repository.ErrCardgroupOwnerNotFound) {
			return nil, ucerr.ErrUnauthenticated
		}
		return nil, eris.Wrap(err, "usecase: master catalog: seed default starters")
	}
	return seeded, nil
}
