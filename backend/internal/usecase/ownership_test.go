package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
)

// mockOwnershipFinder is a minimal stub for CardgroupOwnershipFinder used by
// the ownership helper tests. It returns the configured result/error pair
// unconditionally so callers can pin a single scenario per test.
type mockOwnershipFinder struct {
	result *domain.Cardgroup
	err    error
}

func (m *mockOwnershipFinder) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return m.result, m.err
}

// TestAuthorizeCardgroupOrBadInput_PropagatesCancelled verifies that
// context.Canceled returned by the repository passes through unwrapped, so
// the caller's errors.Is(err, context.Canceled) on the bare top-level error
// works at the usecase boundary.
func TestAuthorizeCardgroupOrBadInput_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{err: context.Canceled}
	err := authorizeCardgroupOrBadInput(context.Background(), repo, "cg-1", "u-1")

	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// TestAuthorizeCardgroupOrBadInput_PropagatesDeadlineExceeded verifies the
// same contract for context.DeadlineExceeded, exercising the second arm of
// isContextDone so a future regression that drops DeadlineExceeded support
// from the guard is detected here as well as in swipe.go / learn.go.
func TestAuthorizeCardgroupOrBadInput_PropagatesDeadlineExceeded(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{err: context.DeadlineExceeded}
	err := authorizeCardgroupOrBadInput(context.Background(), repo, "cg-1", "u-1")

	assertCancelled(t, err)
	require.Equal(t, context.DeadlineExceeded, err, "expected unwrapped context.DeadlineExceeded, got %v", err)
}

// TestAuthorizeCardgroupOrUnauthenticated_PropagatesCancelled verifies that
// context.Canceled returned by the repository passes through unwrapped from
// the unauthenticated-on-missing variant as well.
func TestAuthorizeCardgroupOrUnauthenticated_PropagatesCancelled(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{err: context.Canceled}
	err := authorizeCardgroupOrUnauthenticated(context.Background(), repo, "cg-1", "u-1")

	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// TestAuthorizeCardgroupOrBadInput_NotFound_ReturnsValidationError pins the
// pre-existing ErrNotFound branch so the new isContextDone guard cannot
// silently shadow it.
func TestAuthorizeCardgroupOrBadInput_NotFound_ReturnsValidationError(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{err: repository.ErrNotFound}
	err := authorizeCardgroupOrBadInput(context.Background(), repo, "cg-1", "u-1")
	assertValidationError(t, err, "cardgroupId", "cardgroup not found")
}

// TestAuthorizeCardgroupOrUnauthenticated_NotFound_ReturnsUnauthenticated
// pins the pre-existing ErrNotFound branch for the unauthenticated variant.
func TestAuthorizeCardgroupOrUnauthenticated_NotFound_ReturnsUnauthenticated(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{err: repository.ErrNotFound}
	err := authorizeCardgroupOrUnauthenticated(context.Background(), repo, "cg-1", "u-1")
	assertUnauthenticated(t, err)
}

// TestAuthorizeCardgroupOrBadInput_NonOwner_ReturnsUnauthenticated pins the
// happy-path-with-non-owner branch so the guard insertion cannot perturb
// the post-lookup ownership check.
func TestAuthorizeCardgroupOrBadInput_NonOwner_ReturnsUnauthenticated(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{result: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "other-user"}}
	err := authorizeCardgroupOrBadInput(context.Background(), repo, "cg-1", "u-1")
	assertUnauthenticated(t, err)
}

// TestAuthorizeCardgroupOrUnauthenticated_PropagatesDeadlineExceeded verifies
// the same contract for context.DeadlineExceeded in the unauthenticated-on-
// missing variant, exercising the second arm of isContextDone so a future
// regression that drops DeadlineExceeded support from the guard is detected
// here as well as in swipe.go / learn.go.
func TestAuthorizeCardgroupOrUnauthenticated_PropagatesDeadlineExceeded(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{err: context.DeadlineExceeded}
	err := authorizeCardgroupOrUnauthenticated(context.Background(), repo, "cg-1", "u-1")

	assertCancelled(t, err)
	require.Equal(t, context.DeadlineExceeded, err, "expected unwrapped context.DeadlineExceeded, got %v", err)
}

// TestAuthorizeCardgroupOrUnauthenticated_NonOwner_ReturnsUnauthenticated pins
// the post-lookup ownership check for the unauthenticated variant so the
// isContextDone guard insertion cannot perturb it.
func TestAuthorizeCardgroupOrUnauthenticated_NonOwner_ReturnsUnauthenticated(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{result: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "other-user"}}
	err := authorizeCardgroupOrUnauthenticated(context.Background(), repo, "cg-1", "u-1")
	assertUnauthenticated(t, err)
}

// TestAuthorizeCardgroupOrBadInput_Success_ReturnsNil verifies that the helper
// returns nil when the repository finds the cardgroup and the caller is its owner.
func TestAuthorizeCardgroupOrBadInput_Success_ReturnsNil(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{result: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}}
	err := authorizeCardgroupOrBadInput(context.Background(), repo, "cg-1", "u-1")
	require.NoError(t, err)
}

// TestAuthorizeCardgroupOrBadInput_InfraError_WrappedAsInternal verifies that a
// non-sentinel, non-context infrastructure error is wrapped by eris and that the
// chain contains the canonical prefix so structured logging surfaces the right
// operation name.
func TestAuthorizeCardgroupOrBadInput_InfraError_WrappedAsInternal(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{err: errors.New("db down")}
	err := authorizeCardgroupOrBadInput(context.Background(), repo, "cg-1", "u-1")
	assertInternalChain(t, err, "usecase: authorize cardgroup: find by id")
}

// TestAuthorizeCardgroupOrUnauthenticated_Success_ReturnsNil verifies that the
// helper returns nil when the repository finds the cardgroup and the caller is
// its owner.
func TestAuthorizeCardgroupOrUnauthenticated_Success_ReturnsNil(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{result: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1"}}
	err := authorizeCardgroupOrUnauthenticated(context.Background(), repo, "cg-1", "u-1")
	require.NoError(t, err)
}

// TestAuthorizeCardgroupOrUnauthenticated_InfraError_WrappedAsInternal verifies
// that a non-sentinel, non-context infrastructure error is wrapped by eris and
// that the chain contains the canonical prefix in the unauthenticated-on-missing
// variant.
func TestAuthorizeCardgroupOrUnauthenticated_InfraError_WrappedAsInternal(t *testing.T) {
	t.Parallel()
	repo := &mockOwnershipFinder{err: errors.New("db down")}
	err := authorizeCardgroupOrUnauthenticated(context.Background(), repo, "cg-1", "u-1")
	assertInternalChain(t, err, "usecase: authorize cardgroup: find by id")
}
