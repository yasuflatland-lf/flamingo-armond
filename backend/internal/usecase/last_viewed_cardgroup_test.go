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
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(anonCtx(), "cg-1")
	assertUnauthenticated(t, err)
	if prefs.called != 0 {
		t.Fatalf("repository must not be called when caller is anonymous; got %d calls", prefs.called)
	}
}

func TestLastViewedCardgroup_HappyPath_ReturnsRefreshedUser(t *testing.T) {
	t.Parallel()

	cgID := "cg-123"
	want := &domain.User{
		ID:          "u-1",
		DisplayName: dnPtr("Alice"),
	}
	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{user: want}
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	outcome, err := uc.Set(authedCtx("u-1"), cgID)
	if err != nil {
		t.Fatalf("Set: unexpected error: %v", err)
	}
	if outcome.User == nil || outcome.User.ID != "u-1" {
		t.Fatalf("Set: outcome.User mismatch: got %+v, want %+v", outcome.User, want)
	}
	if outcome.Validation != nil {
		t.Fatalf("Set: unexpected Validation variant on success: %+v", outcome.Validation)
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

// TestLastViewedCardgroup_CardgroupNotFound_ValidationVariant verifies ErrCardgroupNotFound
// maps to the Validation outcome variant (cardgroupId) — "not owned" and "does not exist"
// collapse to the same variant to avoid leaking cardgroup existence.
func TestLastViewedCardgroup_CardgroupNotFound_ValidationVariant(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: repository.ErrCardgroupNotFound}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	outcome, err := uc.Set(authedCtx("u-1"), "cg-foreign")
	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.User != nil {
		t.Fatal("expected nil User on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on cardgroup not found")
	}
	if outcome.Validation.Field != "cardgroupId" {
		t.Fatalf("expected Validation.Field=%q, got %q", "cardgroupId", outcome.Validation.Field)
	}
	if users.calls != 0 {
		t.Fatalf("FindByID must not be called after a sentinel error; got %d calls", users.calls)
	}
}

// TestLastViewedCardgroup_LegacyErrNotFound_FallsThroughToInternal verifies that
// bare ErrNotFound (without ErrCardgroupNotFound join) maps to an error (INTERNAL),
// not the Validation variant — guards the "check specific sentinel first" rule.
func TestLastViewedCardgroup_LegacyErrNotFound_FallsThroughToInternal(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: repository.ErrNotFound}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertInternalChain(t, err, "usecase: set last viewed cardgroup")
}

func TestLastViewedCardgroup_GenericRepoError_Internal(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: eris.New("boom")}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertInternalChain(t, err, "usecase: set last viewed cardgroup")
}

func TestLastViewedCardgroup_ContextCancelled_Cancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{err: context.Canceled}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertCancelled(t, err)
}

// TestLastViewedCardgroup_RefetchUserMissing_Internal verifies that a user row
// deleted between the upsert and refetch reports as an error (INTERNAL), not
// the Validation variant — genuinely abnormal state.
func TestLastViewedCardgroup_RefetchUserMissing_Internal(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{err: repository.ErrNotFound}
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertInternalChain(t, err, "usecase: last viewed cardgroup: refetch own user row")
	if prefs.called != 1 {
		t.Fatalf("UpsertLastViewedCardgroup must have been called once before the refetch failure; got %d", prefs.called)
	}
}

func TestLastViewedCardgroup_RefetchContextCancelled_Cancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{err: context.DeadlineExceeded}
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u-1"), "cg-1")
	assertCancelled(t, err)
}

// TestLastViewedCardgroup_EmptySub_Unauthenticated verifies that an auth context
// with an empty Sub is treated as anonymous (defence-in-depth).
func TestLastViewedCardgroup_EmptySub_Unauthenticated(t *testing.T) {
	t.Parallel()

	prefs := &mockPrefRepo{}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx(""), "cg-1")
	assertUnauthenticated(t, err)
	if prefs.called != 0 {
		t.Fatalf("repository must not be called for empty sub; got %d calls", prefs.called)
	}
}

// TestLastViewedCardgroup_SentinelOrderingMatters verifies the joined sentinel:
// ErrCardgroupNotFound satisfies errors.Is(_, ErrNotFound), and the usecase
// must branch on the specific sentinel first, returning the Validation variant
// not an internal error.
func TestLastViewedCardgroup_SentinelOrderingMatters(t *testing.T) {
	t.Parallel()

	if !errors.Is(repository.ErrCardgroupNotFound, repository.ErrNotFound) {
		t.Fatalf("invariant: ErrCardgroupNotFound must satisfy errors.Is(_, ErrNotFound)")
	}

	prefs := &mockPrefRepo{err: repository.ErrCardgroupNotFound}
	users := &mockUserRefetchRepo{}
	uc := NewLastViewedCardgroupWithDeps(prefs, users, newTestLogger())

	outcome, err := uc.Set(authedCtx("u-1"), "cg-1")
	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Validation == nil {
		t.Fatal("expected Validation variant for ErrCardgroupNotFound, got nil")
	}
	if outcome.Validation.Field != "cardgroupId" {
		t.Fatalf("expected Validation.Field=%q, got %q", "cardgroupId", outcome.Validation.Field)
	}
}
