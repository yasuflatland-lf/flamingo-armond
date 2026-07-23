package usecase

import (
	"context"

	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/usecase/ucerr"
)

// AdminCounter serializes and reports the number of users holding the admin
// role. It is the narrow port consumed by acquireAdminRoleLock and
// guardNotLastAdmin; both
// adminUserRoleRepository (EditUser, DeleteUser) and UserRolesRepository
// (DeleteMyAccount) satisfy it via their AcquireAdminRoleLockTx and
// CountAdminsTx methods.
type AdminCounter interface {
	AcquireAdminRoleLockTx(ctx context.Context, tx *gorm.DB) error
	CountAdminsTx(ctx context.Context, tx *gorm.DB) (int64, error)
}

// acquireAdminRoleLock takes the admin-role advisory lock inside the caller's
// transaction. Every request that may remove an admin must call it before
// reading whether its target currently holds the admin role, so the membership
// read, the admin count, and the mutation they guard form one serialized unit.
//
// Taking the lock only after the membership read reopens the race the lock
// exists to close: a target read as a non-admin (so the guard is skipped
// entirely) can be promoted concurrently, and the demotion or deletion then
// commits against a target that has since become the last admin.
//
// wrapPrefix is caller-supplied so the repository failure is attributed to the
// calling module, not this helper, per the shared-helper rule in
// .claude/rules/error-wrapping.md.
func acquireAdminRoleLock(ctx context.Context, tx *gorm.DB, counter AdminCounter, wrapPrefix string) error {
	if err := counter.AcquireAdminRoleLockTx(ctx, tx); err != nil {
		return wrapInfraErr(err, wrapPrefix)
	}
	return nil
}

// guardNotLastAdmin enforces the "never leave the system without an admin"
// availability invariant shared by EditUser's demote-other branch, DeleteUser,
// and DeleteMyAccount. When isTargetAdmin is true and the account losing the
// admin role is the only remaining admin, it returns a forbidden error carrying
// forbidMsg. When isTargetAdmin is false the admin count is never consulted.
//
// Precondition: the caller already holds the admin-role advisory lock, taken
// via acquireAdminRoleLock inside tx, and read isTargetAdmin under it. The
// count is then read inside the same transaction while the lock is held, so the
// guard and the mutation it protects are serialized against every other request
// that can remove an admin. Without the lock the check is a pure TOCTOU: two
// callers each demoting the other observe two admins, both pass, and both
// commit, leaving zero admins.
//
// The membership decision differs per caller (HasRoleTx for an arbitrary target
// vs IsAdmin for the caller), so it is computed at the call site and passed in
// as isTargetAdmin. wrapPrefix and forbidMsg are likewise caller-supplied.
func guardNotLastAdmin(
	ctx context.Context,
	tx *gorm.DB,
	isTargetAdmin bool,
	counter AdminCounter,
	wrapPrefix string,
	forbidMsg string,
) error {
	if !isTargetAdmin {
		return nil
	}
	n, err := counter.CountAdminsTx(ctx, tx)
	if err != nil {
		return wrapInfraErr(err, wrapPrefix)
	}
	if domain.IsLastAdmin(n) {
		return ucerr.NewForbiddenError(forbidMsg)
	}
	return nil
}
