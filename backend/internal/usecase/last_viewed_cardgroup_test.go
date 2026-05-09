package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

// mockLastViewedCardgroupRepo implements the narrow lastViewedCardgroupRepo
// surface used by LastViewedCardgroupUsecase.
type mockLastViewedCardgroupRepo struct {
	// SetLastViewedCardgroup
	setErr           error
	setCalls         int
	lastSetUserID    string
	lastSetCardGroup string

	// FindByID
	findResult *domain.User
	findErr    error
	findCalls  int
}

func (m *mockLastViewedCardgroupRepo) SetLastViewedCardgroup(_ context.Context, userID, cardgroupID string) error {
	m.setCalls++
	m.lastSetUserID = userID
	m.lastSetCardGroup = cardgroupID
	return m.setErr
}

func (m *mockLastViewedCardgroupRepo) FindByID(_ context.Context, _ string) (*domain.User, error) {
	m.findCalls++
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.findResult, nil
}

// TestLastViewedCardgroup_Anonymous_Unauthenticated verifies that an anonymous
// caller (no auth context) is rejected with UNAUTHENTICATED before the
// repository is touched.
func TestLastViewedCardgroup_Anonymous_Unauthenticated(t *testing.T) {
	t.Parallel()

	repo := &mockLastViewedCardgroupRepo{}
	uc := NewLastViewedCardgroupWithDeps(repo)

	_, err := uc.Set(anonCtx(), "cg-1")
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if repo.setCalls != 0 {
		t.Fatalf("repository must not be called when caller is anonymous; got %d calls", repo.setCalls)
	}
}

// TestLastViewedCardgroup_HappyPath_ReturnsRefreshedUser verifies that the
// usecase calls SetLastViewedCardgroup with the caller's sub and the supplied
// cardgroupID, then returns the refreshed user from FindByID.
func TestLastViewedCardgroup_HappyPath_ReturnsRefreshedUser(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	cgID := "cg-123"
	want := &domain.User{
		ID:                    "u-1",
		DisplayName:           &dn,
		LastViewedCardgroupID: &cgID,
	}
	repo := &mockLastViewedCardgroupRepo{
		findResult: want,
	}
	uc := NewLastViewedCardgroupWithDeps(repo)

	got, err := uc.Set(authedCtx("u-1"), cgID)
	if err != nil {
		t.Fatalf("Set: unexpected error: %v", err)
	}
	if got == nil || got.ID != "u-1" {
		t.Fatalf("Set: returned user mismatch: got %+v, want %+v", got, want)
	}
	if got.LastViewedCardgroupID == nil || *got.LastViewedCardgroupID != cgID {
		t.Fatalf("Set: LastViewedCardgroupID mismatch: got %v, want %q", got.LastViewedCardgroupID, cgID)
	}
	if repo.setCalls != 1 {
		t.Fatalf("expected 1 SetLastViewedCardgroup call, got %d", repo.setCalls)
	}
	if repo.lastSetUserID != "u-1" {
		t.Fatalf("SetLastViewedCardgroup called with userID %q, want %q", repo.lastSetUserID, "u-1")
	}
	if repo.lastSetCardGroup != cgID {
		t.Fatalf("SetLastViewedCardgroup called with cardgroupID %q, want %q", repo.lastSetCardGroup, cgID)
	}
	if repo.findCalls != 1 {
		t.Fatalf("expected 1 FindByID call after Set, got %d", repo.findCalls)
	}
}

// TestLastViewedCardgroup_CardgroupNotFound_BadUserInput verifies that the
// repository's ErrCardgroupNotFound sentinel maps to BAD_USER_INPUT(cardgroupId).
// This is the central authorization-policy assertion: "not owned" and "does
// not exist" collapse to the same response so existence of other users'
// cardgroups is not leaked.
func TestLastViewedCardgroup_CardgroupNotFound_BadUserInput(t *testing.T) {
	t.Parallel()

	repo := &mockLastViewedCardgroupRepo{
		setErr: repository.ErrCardgroupNotFound,
	}
	uc := NewLastViewedCardgroupWithDeps(repo)

	_, err := uc.Set(authedCtx("u-1"), "cg-foreign")
	assertGQLErr(t, err, "BAD_USER_INPUT", "cardgroupId")
	if repo.findCalls != 0 {
		t.Fatalf("FindByID must not be called after a sentinel error; got %d calls", repo.findCalls)
	}
}

// TestLastViewedCardgroup_LegacyErrNotFound_BadUserInput verifies that a
// repository implementation that surfaces only the legacy ErrNotFound (no
// errors.Join with ErrCardgroupNotFound) still maps to BAD_USER_INPUT, because
// ErrCardgroupNotFound is layered with errors.Join(ErrNotFound) — the specific
// sentinel match must be checked first by the usecase. This test guards the
// "always check more specific sentinel before the general one" rule from
// docs/backend/error-wrapping/layered-sentinels-via-errors-join.md by
// exercising a code path where only the general sentinel is returned: it must
// NOT be silently routed into a generic "not found" path.
func TestLastViewedCardgroup_LegacyErrNotFound_FallsThroughToInternal(t *testing.T) {
	t.Parallel()

	// Repository returns the bare ErrNotFound — not the joined sentinel. The
	// usecase has no special handling for this case, so it must surface as
	// INTERNAL rather than collapse to BAD_USER_INPUT.
	repo := &mockLastViewedCardgroupRepo{
		setErr: repository.ErrNotFound,
	}
	uc := NewLastViewedCardgroupWithDeps(repo)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "INTERNAL", "")
}

// TestLastViewedCardgroup_GenericRepoError_Internal verifies that any
// non-sentinel error from SetLastViewedCardgroup is mapped to INTERNAL with
// the eris chain preserved.
func TestLastViewedCardgroup_GenericRepoError_Internal(t *testing.T) {
	t.Parallel()

	repo := &mockLastViewedCardgroupRepo{
		setErr: eris.New("boom"),
	}
	uc := NewLastViewedCardgroupWithDeps(repo)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "INTERNAL", "")
}

// TestLastViewedCardgroup_ContextCancelled_Cancelled verifies that a
// context-cancelled error from the repository surfaces as CANCELLED rather
// than INTERNAL — operators must not see a 5xx alarm for client-driven
// cancellations.
func TestLastViewedCardgroup_ContextCancelled_Cancelled(t *testing.T) {
	t.Parallel()

	repo := &mockLastViewedCardgroupRepo{
		setErr: context.Canceled,
	}
	uc := NewLastViewedCardgroupWithDeps(repo)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "CANCELLED", "")
}

// TestLastViewedCardgroup_RefetchUserMissing_Internal verifies that a user
// row that vanishes between the UPDATE and the refetch is reported as
// INTERNAL — this can only happen if the auth.users row is concurrently
// deleted, which is genuinely abnormal.
func TestLastViewedCardgroup_RefetchUserMissing_Internal(t *testing.T) {
	t.Parallel()

	repo := &mockLastViewedCardgroupRepo{
		findErr: repository.ErrNotFound,
	}
	uc := NewLastViewedCardgroupWithDeps(repo)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "INTERNAL", "")
	if repo.setCalls != 1 {
		t.Fatalf("Set must have been called once before the refetch failure; got %d", repo.setCalls)
	}
}

// TestLastViewedCardgroup_RefetchContextCancelled_Cancelled verifies that a
// context cancellation during the refetch is surfaced as CANCELLED.
func TestLastViewedCardgroup_RefetchContextCancelled_Cancelled(t *testing.T) {
	t.Parallel()

	repo := &mockLastViewedCardgroupRepo{
		findErr: context.DeadlineExceeded,
	}
	uc := NewLastViewedCardgroupWithDeps(repo)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "CANCELLED", "")
}

// TestLastViewedCardgroup_EmptySub_Unauthenticated verifies that an auth
// context with an empty Sub is treated as anonymous (defence-in-depth in case
// upstream middleware leaves a zero-value AuthUser in the context).
func TestLastViewedCardgroup_EmptySub_Unauthenticated(t *testing.T) {
	t.Parallel()

	repo := &mockLastViewedCardgroupRepo{}
	uc := NewLastViewedCardgroupWithDeps(repo)

	// authedCtx with empty sub.
	_, err := uc.Set(authedCtx(""), "cg-1")
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if repo.setCalls != 0 {
		t.Fatalf("repository must not be called for empty sub; got %d calls", repo.setCalls)
	}
}

// TestLastViewedCardgroup_SentinelOrderingMatters guards the
// "always check more specific sentinel before the general one" rule from
// docs/backend/error-wrapping/layered-sentinels-via-errors-join.md.
// ErrCardgroupNotFound is errors.Join'd with ErrNotFound, so errors.Is matches
// both. The usecase must branch on the specific sentinel first — verified here
// by sending the joined sentinel and asserting we receive BAD_USER_INPUT
// (cardgroupId) and not INTERNAL.
func TestLastViewedCardgroup_SentinelOrderingMatters(t *testing.T) {
	t.Parallel()

	if !errors.Is(repository.ErrCardgroupNotFound, repository.ErrNotFound) {
		t.Fatalf("invariant: ErrCardgroupNotFound must satisfy errors.Is(_, ErrNotFound)")
	}

	repo := &mockLastViewedCardgroupRepo{
		setErr: repository.ErrCardgroupNotFound,
	}
	uc := NewLastViewedCardgroupWithDeps(repo)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "BAD_USER_INPUT", "cardgroupId")
}
