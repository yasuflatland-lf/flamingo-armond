package usecase

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

// userFinder is the minimal read-back surface the post-mutation refetch needs:
// a single FindByID keyed by user id. Every per-usecase consumer interface
// (updateNewCardRatioUsersRepo, updateLearnDisplayModeUsersRepo,
// lastViewedCardgroupUsersRepo, adminUserRepository) structurally satisfies it,
// so refetchUser and setUserPreference accept those narrow interfaces without
// widening any of them.
type userFinder interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

// refetchUser loads the user after a mutation so callers see a fresh row
// (e.g. with the trigger-refreshed updated_at). A missing row after a
// successful mutation is unusual; surface it as INTERNAL with the supplied
// context. The original ErrNotFound is wrapped (not replaced) so the chain
// stays intact for errors.Is checks downstream and so the eris error_chain
// log entry preserves the originating sentinel.
//
// Shared by the admin edit path (AdminUser.EditUser) and the per-user
// preference mutations (setUserPreference, last-viewed cardgroup). users is the
// caller's own narrow FindByID interface, so no repository interface is widened.
func refetchUser(ctx context.Context, users userFinder, id, wrap string) (*domain.User, error) {
	user, err := users.FindByID(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, eris.Wrapf(err, "%s: user disappeared", wrap)
		case isContextDone(err):
			return nil, err
		default:
			return nil, eris.Wrap(err, wrap)
		}
	}
	return user, nil
}

// setUserPreference is the shared core for the simple upsert-then-refetch
// preference mutations: authenticate the caller, run the caller-supplied upsert
// closure, then refetch the user row so the resolver returns a fresh User.
//
// The upsert closure receives the authenticated caller sub, so each usecase
// keeps its own narrow prefs interface (a closure over u.prefs) instead of
// widening a shared one. wrapPrefix is the caller's error module prefix
// (e.g. "usecase: update new card ratio"): an infra upsert failure is wrapped
// with it, and the refetch reuses "<wrapPrefix>: refetch own user row".
// context.Canceled / context.DeadlineExceeded pass through unwrapped.
//
// Preferences that carry per-mutation nuance the plain shape cannot express —
// last-viewed's ErrCardgroupNotFound -> Validation branch and its distinct
// outcome type — do NOT route through this core; they reuse requireCallerSub and
// refetchUser directly.
func setUserPreference(
	ctx context.Context,
	upsert func(ctx context.Context, sub string) error,
	users userFinder,
	wrapPrefix string,
) (*domain.User, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return nil, err
	}

	if err := upsert(ctx, caller.Sub); err != nil {
		return nil, wrapInfraErr(err, wrapPrefix)
	}

	return refetchUser(ctx, users, caller.Sub, wrapPrefix+": refetch own user row")
}
