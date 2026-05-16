package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

// mockPrefRepo stubs lastViewedCardgroupRepo and records UpsertLastViewedCardgroup calls.
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

// mockUserRefetchRepo stubs userPreferenceRefetchRepo for the post-upsert refetch step.
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

// TestLastViewedCardgroup_CardgroupNotFound_BadUserInput verifies ErrCardgroupNotFound
// maps to BAD_USER_INPUT(cardgroupId) — "not owned" and "does not exist" collapse
// to the same response to avoid leaking cardgroup existence.
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

// TestLastViewedCardgroup_LegacyErrNotFound_FallsThroughToInternal verifies that
// bare ErrNotFound (without ErrCardgroupNotFound join) maps to INTERNAL, not
// BAD_USER_INPUT — guards the "check specific sentinel first" rule.
func TestLastViewedCardgroup_LegacyErrNotFound_FallsThroughToInternal(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: repository.ErrNotFound}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "INTERNAL", "")
}

func TestLastViewedCardgroup_GenericRepoError_Internal(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: eris.New("boom")}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "INTERNAL", "")
}

func TestLastViewedCardgroup_ContextCancelled_Cancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: context.Canceled}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "CANCELLED", "")
}

// TestLastViewedCardgroup_RefetchUserMissing_Internal verifies that a user row
// deleted between the upsert and refetch reports as INTERNAL (genuinely abnormal).
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

func TestLastViewedCardgroup_RefetchContextCancelled_Cancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{err: context.DeadlineExceeded}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertGQLErr(t, err, "CANCELLED", "")
}

// TestLastViewedCardgroup_EmptySub_Unauthenticated verifies that an auth context
// with an empty Sub is treated as anonymous (defence-in-depth).
func TestLastViewedCardgroup_EmptySub_Unauthenticated(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users)

	_, err := uc.Set(authedCtx(""), "cg-1")
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if prefs.called != 0 {
		t.Fatalf("repository must not be called for empty sub; got %d calls", prefs.called)
	}
}

// TestLastViewedCardgroup_SentinelOrderingMatters verifies the joined sentinel:
// ErrCardgroupNotFound satisfies errors.Is(_, ErrNotFound), and the usecase
// must branch on the specific sentinel first, returning BAD_USER_INPUT not INTERNAL.
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
