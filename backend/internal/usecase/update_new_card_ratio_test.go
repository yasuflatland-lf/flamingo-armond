package usecase

import (
	"context"
	"errors"
	"testing"

	"backend/internal/domain"
	"backend/internal/usecase/ucerr"
)

// mockNewCardRatioPrefRepo stubs updateNewCardRatioPrefsRepo for
// UpsertNewCardRatio calls.
type mockNewCardRatioPrefRepo struct {
	err       error
	called    int
	gotUserID string
	gotNum    int
	gotDen    int
}

func (m *mockNewCardRatioPrefRepo) UpsertNewCardRatio(_ context.Context, userID string, num, den int) error {
	m.called++
	m.gotUserID = userID
	m.gotNum = num
	m.gotDen = den
	return m.err
}

// mockNewCardRatioUserRepo stubs updateNewCardRatioUsersRepo for the
// post-write user refetch step.
type mockNewCardRatioUserRepo struct {
	user  *domain.User
	err   error
	calls int
}

func (m *mockNewCardRatioUserRepo) FindByID(_ context.Context, _ string) (*domain.User, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func TestUpdateNewCardRatio_Unauthenticated(t *testing.T) {
	t.Parallel()

	prefs := &mockNewCardRatioPrefRepo{}
	users := &mockNewCardRatioUserRepo{}
	uc := newUpdateNewCardRatioWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(anonCtx(), 4, 5)
	assertUnauthenticated(t, err)
	if prefs.called != 0 {
		t.Fatalf("repository must not be called when caller is anonymous; got %d calls", prefs.called)
	}
}

func TestUpdateNewCardRatio_Success(t *testing.T) {
	t.Parallel()

	// 6/10 reduces to 3/5; the usecase parses the raw ints and persists the
	// reduced fraction.
	want := &domain.User{ID: "u1"}
	prefs := &mockNewCardRatioPrefRepo{}
	users := &mockNewCardRatioUserRepo{user: want}
	uc := newUpdateNewCardRatioWithDeps(prefs, users, newTestLogger())

	got, err := uc.Set(authedCtx("u1"), 6, 10)
	if err != nil {
		t.Fatalf("Set: unexpected error: %v", err)
	}
	if got == nil || got.ID != "u1" {
		t.Fatalf("Set: returned user ID mismatch: got %+v, want ID=%q", got, "u1")
	}
	if prefs.called != 1 {
		t.Fatalf("expected 1 UpsertNewCardRatio call, got %d", prefs.called)
	}
	if prefs.gotUserID != "u1" {
		t.Fatalf("UpsertNewCardRatio called with userID %q, want %q", prefs.gotUserID, "u1")
	}
	if prefs.gotNum != 3 || prefs.gotDen != 5 {
		t.Fatalf("UpsertNewCardRatio called with %d/%d, want reduced 3/5", prefs.gotNum, prefs.gotDen)
	}
	if users.calls != 1 {
		t.Fatalf("expected 1 FindByID call after update, got %d", users.calls)
	}
}

// TestUpdateNewCardRatio_PrefsWriteError verifies that a generic prefs repo
// error is eris-wrapped (not the bare UNAUTHENTICATED sentinel) and that the
// user refetch is skipped after a write failure.
func TestUpdateNewCardRatio_PrefsWriteError(t *testing.T) {
	t.Parallel()

	boom := errors.New("db unavailable")
	prefs := &mockNewCardRatioPrefRepo{err: boom}
	users := &mockNewCardRatioUserRepo{}
	uc := newUpdateNewCardRatioWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u1"), 4, 5)
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

// TestUpdateNewCardRatio_PrefsWriteCancelled verifies that context.Canceled
// from the prefs repo is returned as-is (identity preserved, not eris-wrapped
// as INTERNAL).
func TestUpdateNewCardRatio_PrefsWriteCancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockNewCardRatioPrefRepo{err: context.Canceled}
	users := &mockNewCardRatioUserRepo{}
	uc := newUpdateNewCardRatioWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u1"), 4, 5)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Set: expected errors.Is(err, context.Canceled)=true, got %v (%T)", err, err)
	}
}

// TestUpdateNewCardRatio_RefetchError verifies that a generic error from the
// user refetch step is eris-wrapped (not the bare UNAUTHENTICATED sentinel).
func TestUpdateNewCardRatio_RefetchError(t *testing.T) {
	t.Parallel()

	boom := errors.New("user repo unavailable")
	prefs := &mockNewCardRatioPrefRepo{}
	users := &mockNewCardRatioUserRepo{err: boom}
	uc := newUpdateNewCardRatioWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u1"), 4, 5)
	if err == nil {
		t.Fatal("Set: expected error on refetch failure, got nil")
	}
	if errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("Set: error must not be ErrUnauthenticated; got %v", err)
	}
}

// TestUpdateNewCardRatio_RefetchCancelled verifies that context.DeadlineExceeded
// from the user refetch step is returned as-is (identity preserved).
func TestUpdateNewCardRatio_RefetchCancelled(t *testing.T) {
	t.Parallel()

	prefs := &mockNewCardRatioPrefRepo{}
	users := &mockNewCardRatioUserRepo{err: context.DeadlineExceeded}
	uc := newUpdateNewCardRatioWithDeps(prefs, users, newTestLogger())

	_, err := uc.Set(authedCtx("u1"), 4, 5)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Set: expected errors.Is(err, context.DeadlineExceeded)=true, got %v (%T)", err, err)
	}
}

// TestUpdateNewCardRatio_InvalidRatio_FieldAttribution pins the wire field a bad
// ratio faults on to the reduced fraction domain.ParseNewCardRatio actually
// checks: a new-card share outside (0, denominator) faults the numerator; a
// non-positive or over-cap (reduced) denominator faults the denominator. The
// "3/303" case reduces to 1/101, so its over-cap denominator is only visible on
// the reduced fraction. No write runs for any invalid ratio.
func TestUpdateNewCardRatio_InvalidRatio_FieldAttribution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		num, den  int
		wantField string
	}{
		{"numerator equals denominator", 5, 5, "numerator"},
		{"numerator exceeds denominator", 7, 5, "numerator"},
		{"zero numerator", 0, 5, "numerator"},
		{"negative numerator", -1, 5, "numerator"},
		{"non-positive denominator", 1, 0, "denominator"},
		{"reduced denominator over cap", 1, 101, "denominator"},
		{"reduced denominator over cap after reduction", 3, 303, "denominator"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prefs := &mockNewCardRatioPrefRepo{}
			users := &mockNewCardRatioUserRepo{}
			uc := newUpdateNewCardRatioWithDeps(prefs, users, newTestLogger())

			_, err := uc.Set(authedCtx("u1"), tc.num, tc.den)
			assertValidationError(t, err, tc.wantField, "invalid new-card ratio")
			if prefs.called != 0 {
				t.Fatalf("UpsertNewCardRatio must not run for an invalid ratio; got %d calls", prefs.called)
			}
			if users.calls != 0 {
				t.Fatalf("FindByID must not run for an invalid ratio; got %d calls", users.calls)
			}
		})
	}
}

// TestTranslateNewCardRatioErr_NilAndDefaultArm covers the two branches
// domain.ParseNewCardRatio can never drive: a nil error passes through, and an
// unexpected non-domain error maps to INTERNAL rather than a validation error.
func TestTranslateNewCardRatioErr_NilAndDefaultArm(t *testing.T) {
	t.Parallel()

	if err := translateNewCardRatioErr(nil); err != nil {
		t.Fatalf("translateNewCardRatioErr(nil): want nil, got %v", err)
	}

	err := translateNewCardRatioErr(errors.New("surprise"))
	assertInternalChain(t, err, "usecase: update new card ratio: translate ratio error")
}
