package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

// mockPrefRepo implements the narrow lastViewedCardgroupRepo interface consumed
// by LastViewedCardgroupUsecase. It records calls to UpsertLastViewedCardgroup
// and returns the configured error.
type mockPrefRepo struct {
	err            error
	called         int
	gotUserID      string
	gotCardgroupID string
}

func (m *mockPrefRepo) UpsertLastViewedCardgroup(_ context.Context, userID, cardgroupID string) error {
	m.called++
	m.gotUserID = userID
	m.gotCardgroupID = cardgroupID
	return m.err
}

// mockUserRefetchRepo implements the narrow userPreferenceRefetchRepo interface
// consumed by LastViewedCardgroupUsecase for the post-upsert refetch step.
type mockUserRefetchRepo struct {
	user  *domain.User
	err   error
	calls int
}

func (m *mockUserRefetchRepo) FindByID(_ context.Context, _ string) (*domain.User, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

// TestLastViewedCardgroup_Anonymous_Unauthenticated verifies that an anonymous
// caller (no auth context) is rejected with UNAUTHENTICATED before the
// repository is touched.
func TestLastViewedCardgroup_Anonymous_Unauthenticated(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(anonCtx(), "cg-1")
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if prefs.called != 0 {
		t.Fatalf("repository must not be called when caller is anonymous; got %d calls", prefs.called)
	}
}

// TestLastViewedCardgroup_HappyPath_ReturnsRefreshedUser verifies that the
// usecase calls UpsertLastViewedCardgroup with the caller's sub and the supplied
// cardgroupID, then returns the refreshed user from FindByID.
func TestLastViewedCardgroup_HappyPath_ReturnsRefreshedUser(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	cgID := "cg-123"
	want := &domain.User{
		ID:          "u-1",
		DisplayName: &dn,
	}
	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{user: want}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	got, err := uc.Set(authedCtx("u-1"), cgID)
	if err != nil {
		t.Fatalf("Set: unexpected error: %v", err)
	}
	if got == nil || got.ID != "u-1" {
		t.Fatalf("Set: returned user mismatch: got %+v, want %+v", got, want)
	}
	if prefs.called != 1 {
		t.Fatalf("expected 1 UpsertLastViewedCardgroup call, got %d", prefs.called)
	}
	if prefs.gotUserID != "u-1" {
		t.Fatalf("UpsertLastViewedCardgroup called with userID %q, want %q", prefs.gotUserID, "u-1")
	}
	if prefs.gotCardgroupID != cgID {
		t.Fatalf("UpsertLastViewedCardgroup called with cardgroupID %q, want %q", prefs.gotCardgroupID, cgID)
	}
	if users.calls != 1 {
		t.Fatalf("expected 1 FindByID call after upsert, got %d", users.calls)
	}
}

// TestLastViewedCardgroup_CardgroupNotFound_BadUserInput verifies that the
// repository's ErrCardgroupNotFound sentinel maps to BAD_USER_INPUT(cardgroupId).
// This is the central authorization-policy assertion: "not owned" and "does
// not exist" collapse to the same response so existence of other users'
// cardgroups is not leaked.
func TestLastViewedCardgroup_CardgroupNotFound_BadUserInput(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: repository.ErrCardgroupNotFound}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-foreign")
	assertGQLErr(t, err, "BAD_USER_INPUT", "cardgroupId")
	if users.calls != 0 {
		t.Fatalf("FindByID must not be called after a sentinel error; got %d calls", users.calls)
	}
}

// TestLastViewedCardgroup_LegacyErrNotFound_FallsThroughToInternal verifies
// that a repository implementation that surfaces only the legacy ErrNotFound
// (no errors.Join with ErrCardgroupNotFound) still maps to INTERNAL, because
// the usecase has no special handling for the general sentinel — it must NOT
// be silently routed into a BAD_USER_INPUT path. This guards the "always check
// more specific sentinel before the general one" rule.
func TestLastViewedCardgroup_LegacyErrNotFound_FallsThroughToInternal(t *testing.T) {
	t.Parallel()

	// Repository returns the bare ErrNotFound — not the joined sentinel. The
	// usecase has no special handling for this case, so it must surface as
	// INTERNAL rather than collapse to BAD_USER_INPUT.
	prefs := &mockPrefRepo{err: repository.ErrNotFound}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "INTERNAL", "")
}

// TestLastViewedCardgroup_GenericRepoError_Internal verifies that any
// non-sentinel error from UpsertLastViewedCardgroup is mapped to INTERNAL with
// the eris chain preserved.
func TestLastViewedCardgroup_GenericRepoError_Internal(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: eris.New("boom")}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "INTERNAL", "")
}

// TestLastViewedCardgroup_ContextCancelled_Cancelled verifies that a
// context-cancelled error from the repository surfaces as CANCELLED rather
// than INTERNAL — operators must not see a 5xx alarm for client-driven
// cancellations.
func TestLastViewedCardgroup_ContextCancelled_Cancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: context.Canceled}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "CANCELLED", "")
}

// TestLastViewedCardgroup_RefetchUserMissing_Internal verifies that a user
// row that vanishes between the UPDATE and the refetch is reported as
// INTERNAL — this can only happen if the auth.users row is concurrently
// deleted, which is genuinely abnormal.
func TestLastViewedCardgroup_RefetchUserMissing_Internal(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{err: repository.ErrNotFound}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "INTERNAL", "")
	if prefs.called != 1 {
		t.Fatalf("UpsertLastViewedCardgroup must have been called once before the refetch failure; got %d", prefs.called)
	}
}

// TestLastViewedCardgroup_RefetchContextCancelled_Cancelled verifies that a
// context cancellation during the refetch is surfaced as CANCELLED.
func TestLastViewedCardgroup_RefetchContextCancelled_Cancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{err: context.DeadlineExceeded}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "CANCELLED", "")
}

// TestLastViewedCardgroup_EmptySub_Unauthenticated verifies that an auth
// context with an empty Sub is treated as anonymous (defence-in-depth in case
// upstream middleware leaves a zero-value AuthUser in the context).
func TestLastViewedCardgroup_EmptySub_Unauthenticated(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	// authedCtx with empty sub.
	_, err := uc.Set(authedCtx(""), "cg-1")
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if prefs.called != 0 {
		t.Fatalf("repository must not be called for empty sub; got %d calls", prefs.called)
	}
}

// TestLastViewedCardgroup_SentinelOrderingMatters guards the
// "always check more specific sentinel before the general one" rule.
// ErrCardgroupNotFound is errors.Join'd with ErrNotFound, so errors.Is matches
// both. The usecase must branch on the specific sentinel first — verified here
// by sending the joined sentinel and asserting we receive BAD_USER_INPUT
// (cardgroupId) and not INTERNAL.
func TestLastViewedCardgroup_SentinelOrderingMatters(t *testing.T) {
	t.Parallel()

	if !errors.Is(repository.ErrCardgroupNotFound, repository.ErrNotFound) {
		t.Fatalf("invariant: ErrCardgroupNotFound must satisfy errors.Is(_, ErrNotFound)")
	}

	prefs := &mockPrefRepo{err: repository.ErrCardgroupNotFound}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "BAD_USER_INPUT", "cardgroupId")
}
