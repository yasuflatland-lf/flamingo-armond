package usecase

import (
	"context"
	"testing"

	"backend/internal/domain"
)

// mockLearnModePrefRepo stubs updateLearnDisplayModePrefsRepo for
// UpdateLearnDisplayMode calls.
type mockLearnModePrefRepo struct {
	err       error
	called    int
	gotUserID string
	gotMode   string
}

func (m *mockLearnModePrefRepo) UpdateLearnDisplayMode(_ context.Context, userID, mode string) error {
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
		t.Fatalf("expected 1 UpdateLearnDisplayMode call, got %d", prefs.called)
	}
	if prefs.gotUserID != "u1" {
		t.Fatalf("UpdateLearnDisplayMode called with userID %q, want %q", prefs.gotUserID, "u1")
	}
	if prefs.gotMode != domain.LearnDisplayFlipToReveal.String() {
		t.Fatalf("UpdateLearnDisplayMode called with mode %q, want %q", prefs.gotMode, domain.LearnDisplayFlipToReveal.String())
	}
	if users.calls != 1 {
		t.Fatalf("expected 1 FindByID call after update, got %d", users.calls)
	}
}
