package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/rotisserie/eris"

	"backend/internal/usecase/ucerr"
)

// assertUnauthenticated asserts that err satisfies errors.Is(err, ucerr.ErrUnauthenticated).
func assertUnauthenticated(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("assertUnauthenticated: expected errors.Is(err, ErrUnauthenticated)=true, got err=%v (%T)", err, err)
	}
}

// assertValidationError asserts that err narrows to *ucerr.ValidationError via errors.AsType.
// When field is non-empty, also asserts ve.Field == field.
// When message is non-empty, also asserts ve.Message == message.
func assertValidationError(t *testing.T, err error, field, message string) {
	t.Helper()
	ve, ok := errors.AsType[*ucerr.ValidationError](err)
	if !ok {
		t.Fatalf("assertValidationError: expected *ucerr.ValidationError in chain, got %T: %v", err, err)
	}
	if field != "" && ve.Field != field {
		t.Fatalf("assertValidationError: Field: want %q, got %q", field, ve.Field)
	}
	if message != "" && ve.Message != message {
		t.Fatalf("assertValidationError: Message: want %q, got %q", message, ve.Message)
	}
}

// assertForbidden asserts that err narrows to *ucerr.ForbiddenError via errors.AsType.
// When message is non-empty, also asserts fe.Message == message.
func assertForbidden(t *testing.T, err error, message string) {
	t.Helper()
	fe, ok := errors.AsType[*ucerr.ForbiddenError](err)
	if !ok {
		t.Fatalf("assertForbidden: expected *ucerr.ForbiddenError in chain, got %T: %v", err, err)
	}
	if message != "" && fe.Message != message {
		t.Fatalf("assertForbidden: Message: want %q, got %q", message, fe.Message)
	}
}

// assertCancelled asserts that err satisfies errors.Is for context.Canceled or
// context.DeadlineExceeded.
func assertCancelled(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("assertCancelled: expected context.Canceled or context.DeadlineExceeded in chain, got %T: %v", err, err)
	}
}

// assertInternalChain asserts that err is non-nil, does not match any of the
// typed-error narrowings (Unauthenticated, ValidationError, ForbiddenError,
// Cancelled/DeadlineExceeded), and that at least one frame in the eris unpack
// chain (ErrChain wrap messages, ErrRoot message, or ErrExternal message)
// contains wantSubstr.
func assertInternalChain(t *testing.T, err error, wantSubstr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("assertInternalChain: expected non-nil error")
	}
	if errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("assertInternalChain: error is ErrUnauthenticated, not internal-class: %v", err)
	}
	if _, ok := errors.AsType[*ucerr.ValidationError](err); ok {
		t.Fatalf("assertInternalChain: error is *ucerr.ValidationError, not internal-class: %v", err)
	}
	if _, ok := errors.AsType[*ucerr.ForbiddenError](err); ok {
		t.Fatalf("assertInternalChain: error is *ucerr.ForbiddenError, not internal-class: %v", err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("assertInternalChain: error is context-cancelled, not internal-class: %v", err)
	}

	unpacked := eris.Unpack(err)

	// Collect all text from the unpack chain.
	var frames []string
	for _, link := range unpacked.ErrChain {
		frames = append(frames, link.Msg)
	}
	if unpacked.ErrRoot.Msg != "" {
		frames = append(frames, unpacked.ErrRoot.Msg)
	}
	if unpacked.ErrExternal != nil {
		frames = append(frames, unpacked.ErrExternal.Error())
	}

	for _, f := range frames {
		if strings.Contains(f, wantSubstr) {
			return
		}
	}
	t.Fatalf("assertInternalChain: no frame contains %q; frames: %v", wantSubstr, frames)
}

// --- Positive-path tests for each helper ---

func TestAssertUnauthenticated_Passes(t *testing.T) {
	t.Parallel()
	assertUnauthenticated(t, ucerr.ErrUnauthenticated)
	// Also passes when wrapped with eris.
	assertUnauthenticated(t, eris.Wrap(ucerr.ErrUnauthenticated, "outer"))
}

func TestAssertValidationError_Passes(t *testing.T) {
	t.Parallel()
	err := &ucerr.ValidationError{Field: "name", Message: "required"}
	assertValidationError(t, err, "name", "required")
	assertValidationError(t, err, "name", "") // skips message check
	assertValidationError(t, err, "", "")     // skips both checks
}

func TestAssertForbidden_Passes(t *testing.T) {
	t.Parallel()
	err := &ucerr.ForbiddenError{Message: "no access"}
	assertForbidden(t, err, "no access")
	assertForbidden(t, err, "") // skips message check
}

func TestAssertCancelled_Passes(t *testing.T) {
	t.Parallel()
	assertCancelled(t, context.Canceled)
	assertCancelled(t, context.DeadlineExceeded)
	assertCancelled(t, eris.Wrap(context.Canceled, "outer"))
}

func TestAssertInternalChain_Passes(t *testing.T) {
	t.Parallel()
	base := errors.New("db: timeout")
	err := eris.Wrap(base, "usecase: fetch user")
	assertInternalChain(t, err, "usecase: fetch user")
	assertInternalChain(t, err, "db: timeout") // matches ErrExternal frame
}

// newTestLogger returns a *slog.Logger backed by slog.DiscardHandler so unit
// tests can construct usecases without producing log noise.
func newTestLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// mockAdminChecker is a stub for the AdminChecker interface. Admin-facing
// usecase tests wrap it via NewAdminGate(...).
type mockAdminChecker struct {
	isAdmin bool
	err     error
	calls   int
	// onCall, when set, runs at the top of IsAdmin. Tests use it to snapshot
	// sibling-mock counters and assert call ordering (e.g. that the admin-role
	// advisory lock was already taken when the membership read happened).
	onCall func()
}

func (m *mockAdminChecker) IsAdmin(_ context.Context, _ string) (bool, error) {
	m.calls++
	if m.onCall != nil {
		m.onCall()
	}
	if m.err != nil {
		return false, m.err
	}
	return m.isAdmin, nil
}
