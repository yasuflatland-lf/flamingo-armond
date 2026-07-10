package usecase

import (
	"context"
	"errors"
	"testing"

	"backend/internal/domain"
	"backend/internal/usecase/ucerr"
)

// mockLearnModePrefRepo stubs updateLearnDisplayModePrefsRepo for
// UpsertLearnDisplayMode calls.
type mockLearnModePrefRepo struct {
	err       error
	called    int
	gotUserID string
	gotMode   string
}

func (m *mockLearnModePrefRepo) UpsertLearnDisplayMode(_ context.Context, userID, mode string) error {
	m.called++
	m.gotUserID = userID
	m.gotMode = mode
	return m.err
}

// mockLearnModeUserRepo stubs updateLearnDisplayModeUsersRepo for the
// post-write user refetch step.
type mockLearnModeUserRepo struct {
	user  *domain.User
	err   error
	calls int
}

func (m *mockLearnModeUserRepo) FindByID(_ context.Context, _ string) (*domain.User, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func TestUpdateLearnDisplayMode_Unauthenticated(t *testing.T) {
	t.Parallel()

	prefs := &mockLearnModePrefRepo{}
	users := &mockLearnModeUserRepo{}
	uc := NewUpdateLearnDisplayModeWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(anonCtx(), domain.LearnDisplayFlipToReveal)
	assertUnauthenticated(t, err)
	if prefs.called != 0 {
		t.Fatalf("repository must not be called when caller is anonymous; got %d calls", prefs.called)
	}
}

func TestUpdateLearnDisplayMode_Success(t *testing.T) {
	t.Parallel()

	want := &domain.User{ID: "u1"}
	prefs := &mockLearnModePrefRepo{}
	users := &mockLearnModeUserRepo{user: want}
	uc := NewUpdateLearnDisplayModeWithDeps(prefs, users, newTestLogger())

	got, err := uc.Set(authedCtx("u1"), domain.LearnDisplayFlipToReveal)
	if err != nil {
		t.Fatalf("Set: unexpected error: %v", err)
	}
	if got == nil || got.ID != "u1" {
		t.Fatalf("Set: returned user ID mismatch: got %+v, want ID=%q", got, "u1")
	}
	if prefs.called != 1 {
		t.Fatalf("expected 1 UpsertLearnDisplayMode call, got %d", prefs.called)
	}
	if prefs.gotUserID != "u1" {
		t.Fatalf("UpsertLearnDisplayMode called with userID %q, want %q", prefs.gotUserID, "u1")
	}
	if prefs.gotMode != domain.LearnDisplayFlipToReveal.String() {
		t.Fatalf("UpsertLearnDisplayMode called with mode %q, want %q", prefs.gotMode, domain.LearnDisplayFlipToReveal.String())
	}
	if users.calls != 1 {
		t.Fatalf("expected 1 FindByID call after update, got %d", users.calls)
	}
}

// TestUpdateLearnDisplayMode_PrefsWriteError verifies that a generic prefs repo
// error is eris-wrapped (not the bare UNAUTHENTICATED sentinel) and that the
// user refetch is skipped after a write failure.
func TestUpdateLearnDisplayMode_PrefsWriteError(t *testing.T) {
	t.Parallel()

	boom := errors.New("db unavailable")
	prefs := &mockLearnModePrefRepo{err: boom}
	users := &mockLearnModeUserRepo{}
	uc := NewUpdateLearnDisplayModeWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u1"), domain.LearnDisplayFlipToReveal)
	if err == nil {
		t.Fatal("Set: expected error, got nil")
	}
	if errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("Set: error must not be ErrUnauthenticated; got %v", err)
	}
	if users.calls != 0 {
		t.Fatalf("FindByID must not be called after prefs write failure; got %d calls", users.calls)
	}
}

// TestUpdateLearnDisplayMode_PrefsWriteCancelled verifies that context.Canceled
// from the prefs repo is returned as-is (identity preserved, not eris-wrapped
// as INTERNAL).
func TestUpdateLearnDisplayMode_PrefsWriteCancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockLearnModePrefRepo{err: context.Canceled}
	users := &mockLearnModeUserRepo{}
	uc := NewUpdateLearnDisplayModeWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u1"), domain.LearnDisplayFlipToReveal)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Set: expected errors.Is(err, context.Canceled)=true, got %v (%T)", err, err)
	}
}

// TestUpdateLearnDisplayMode_RefetchError verifies that a generic error from
// the user refetch step is eris-wrapped (not the bare UNAUTHENTICATED sentinel).
func TestUpdateLearnDisplayMode_RefetchError(t *testing.T) {
	t.Parallel()

	boom := errors.New("user repo unavailable")
	prefs := &mockLearnModePrefRepo{}
	users := &mockLearnModeUserRepo{err: boom}
	uc := NewUpdateLearnDisplayModeWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u1"), domain.LearnDisplayFlipToReveal)
	if err == nil {
		t.Fatal("Set: expected error on refetch failure, got nil")
	}
	if errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("Set: error must not be ErrUnauthenticated; got %v", err)
	}
}

// TestUpdateLearnDisplayMode_RefetchCancelled verifies that context.DeadlineExceeded
// from the user refetch step is returned as-is (identity preserved).
func TestUpdateLearnDisplayMode_RefetchCancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockLearnModePrefRepo{}
	users := &mockLearnModeUserRepo{err: context.DeadlineExceeded}
	uc := NewUpdateLearnDisplayModeWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u1"), domain.LearnDisplayFlipToReveal)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Set: expected errors.Is(err, context.DeadlineExceeded)=true, got %v (%T)", err, err)
	}
}
