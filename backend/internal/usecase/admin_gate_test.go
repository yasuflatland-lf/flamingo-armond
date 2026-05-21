package usecase

import (
	"context"
	"errors"
	"testing"

	"backend/internal/usecase/ucerr"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/require"
)

// stubAdminGateChecker is a per-test stub for AdminChecker whose behaviour is
// configured via a plain function field. It is intentionally separate from
// adminAuthChecker (in admin_user_test.go) because adminAuthChecker keys on a
// map of user IDs and does not support returning arbitrary errors — the cases
// below need fine-grained error injection (context.Canceled, ucerr.ErrUnauthenticated,
// *ucerr.ForbiddenError, and arbitrary eris-wrapped errors).
type stubAdminGateChecker struct {
	fn func(ctx context.Context, userID string) (bool, error)
}

func (s *stubAdminGateChecker) IsAdmin(ctx context.Context, userID string) (bool, error) {
	if s.fn == nil {
		panic("stubAdminGateChecker.IsAdmin called without fn set")
	}
	return s.fn(ctx, userID)
}

func TestNewAdminGate_PanicsOnNilChecker(t *testing.T) {
	require.PanicsWithValue(t, "usecase: admin gate: checker must not be nil", func() {
		_ = NewAdminGate(nil)
	})
}

func TestAdminGateRequire(t *testing.T) {
	t.Parallel()

	type wantErr int
	const (
		wantNil wantErr = iota
		wantUnauthenticated
		wantForbidden
		wantContextCanceled
		wantContextDeadline
		wantWrapped
	)

	const callerPfx = "usecase: test gate: require"

	// downstreamForbidden is the *ucerr.ForbiddenError that the stub returns for
	// the pass-through case. We keep a pointer so the same instance can be
	// identity-checked after the gate returns it.
	downstreamForbidden := ucerr.NewForbiddenError("downstream")

	cases := []struct {
		name         string
		ctx          func(t *testing.T) context.Context
		stubFn       func(ctx context.Context, userID string) (bool, error)
		callerPrefix string
		wantErrKind  wantErr
		wantCallerID string
	}{
		{
			// 1. No AuthUser at all on the context.
			name:         "nil_caller",
			ctx:          func(t *testing.T) context.Context { t.Helper(); return anonCtx() },
			stubFn:       nil, // IsAdmin must never be reached
			callerPrefix: callerPfx,
			wantErrKind:  wantUnauthenticated,
		},
		{
			// 2. AuthUser present but Sub is empty.
			name: "empty_sub",
			ctx: func(t *testing.T) context.Context {
				t.Helper()
				return authedCtx("")
			},
			stubFn:       nil, // IsAdmin must never be reached
			callerPrefix: callerPfx,
			wantErrKind:  wantUnauthenticated,
		},
		{
			// 3. IsAdmin returns (false, nil) — not an admin.
			name:         "not_admin",
			ctx:          func(t *testing.T) context.Context { t.Helper(); return authedCtx("user-1") },
			stubFn:       func(_ context.Context, _ string) (bool, error) { return false, nil },
			callerPrefix: callerPfx,
			wantErrKind:  wantForbidden,
		},
		{
			// 4. IsAdmin returns (true, nil) — success path.
			name:         "success",
			ctx:          func(t *testing.T) context.Context { t.Helper(); return authedCtx("user-1") },
			stubFn:       func(_ context.Context, _ string) (bool, error) { return true, nil },
			callerPrefix: callerPfx,
			wantErrKind:  wantNil,
			wantCallerID: "user-1",
		},
		{
			// 5. IsAdmin propagates context.Canceled — must be returned unwrapped.
			name:         "context_canceled",
			ctx:          func(t *testing.T) context.Context { t.Helper(); return authedCtx("user-1") },
			stubFn:       func(_ context.Context, _ string) (bool, error) { return false, context.Canceled },
			callerPrefix: callerPfx,
			wantErrKind:  wantContextCanceled,
		},
		{
			// 6. IsAdmin propagates context.DeadlineExceeded — must be returned unwrapped.
			name:         "context_deadline",
			ctx:          func(t *testing.T) context.Context { t.Helper(); return authedCtx("user-1") },
			stubFn:       func(_ context.Context, _ string) (bool, error) { return false, context.DeadlineExceeded },
			callerPrefix: callerPfx,
			wantErrKind:  wantContextDeadline,
		},
		{
			// 7. IsAdmin returns ucerr.ErrUnauthenticated — pass-through, no wrap.
			name:         "isadmin_returns_unauthenticated",
			ctx:          func(t *testing.T) context.Context { t.Helper(); return authedCtx("user-1") },
			stubFn:       func(_ context.Context, _ string) (bool, error) { return false, ucerr.ErrUnauthenticated },
			callerPrefix: callerPfx,
			wantErrKind:  wantUnauthenticated,
		},
		{
			// 8. IsAdmin returns *ucerr.ForbiddenError — pass-through, same instance.
			name:         "isadmin_returns_forbidden",
			ctx:          func(t *testing.T) context.Context { t.Helper(); return authedCtx("user-1") },
			stubFn:       func(_ context.Context, _ string) (bool, error) { return false, downstreamForbidden },
			callerPrefix: callerPfx,
			wantErrKind:  wantForbidden,
		},
		{
			// 9. IsAdmin returns an arbitrary infra error — must be wrapped with callerPrefix.
			name:         "isadmin_returns_infra_error",
			ctx:          func(t *testing.T) context.Context { t.Helper(); return authedCtx("user-1") },
			stubFn:       func(_ context.Context, _ string) (bool, error) { return false, errors.New("db: timeout") },
			callerPrefix: callerPfx,
			wantErrKind:  wantWrapped,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stub := &stubAdminGateChecker{fn: tc.stubFn}
			gate := NewAdminGate(stub)

			callerID, err := gate.Require(tc.ctx(t), tc.callerPrefix)

			switch tc.wantErrKind {
			case wantNil:
				require.NoError(t, err)
				require.Equal(t, tc.wantCallerID, callerID)

			case wantUnauthenticated:
				assertUnauthenticated(t, err)
				require.Empty(t, callerID)

			case wantForbidden:
				assertForbidden(t, err, "")
				require.Empty(t, callerID)

			case wantContextCanceled:
				require.True(t, errors.Is(err, context.Canceled),
					"expected errors.Is(err, context.Canceled)=true, got %v", err)
				require.Empty(t, callerID)

			case wantContextDeadline:
				require.True(t, errors.Is(err, context.DeadlineExceeded),
					"expected errors.Is(err, context.DeadlineExceeded)=true, got %v", err)
				require.Empty(t, callerID)

			case wantWrapped:
				assertInternalChain(t, err, tc.callerPrefix)
				// The original infra error must survive in the chain.
				unpacked := eris.Unpack(err)
				require.NotNil(t, unpacked.ErrExternal, "expected an external (non-eris) root in chain")
				require.Empty(t, callerID)
			}
		})
	}

	// Extra identity assertion for the pass-through *ForbiddenError case: the
	// exact pointer returned by the stub must escape without an additional wrap.
	t.Run("isadmin_returns_forbidden_same_instance", func(t *testing.T) {
		t.Parallel()
		stub := &stubAdminGateChecker{
			fn: func(_ context.Context, _ string) (bool, error) { return false, downstreamForbidden },
		}
		gate := NewAdminGate(stub)
		_, err := gate.Require(authedCtx("user-1"), callerPfx)
		require.ErrorIs(t, err, downstreamForbidden,
			"pass-through ForbiddenError must be the same instance — no additional wrap")
	})
}
