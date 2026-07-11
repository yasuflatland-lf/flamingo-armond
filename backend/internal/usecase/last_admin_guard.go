package usecase

import (
	"context"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/usecase/ucerr"
)

// AdminCounter reports the number of users holding the admin role. It is the
// narrow port consumed by guardNotLastAdmin; both adminUserRoleRepository
// (DeleteUser) and UserRolesRepository (DeleteMyAccount) satisfy it via their
// CountAdmins method.
type AdminCounter interface {
	CountAdmins(ctx context.Context) (int64, error)
}

// guardNotLastAdmin enforces the "never delete the last admin" availability
// invariant shared by DeleteUser and DeleteMyAccount. When isTargetAdmin is
// true and the account being removed is the only remaining admin, it returns a
// forbidden error carrying forbidMsg so the system is never left without an
// admin. When isTargetAdmin is false the admin count is never consulted.
//
// The membership decision differs per caller (HasRole for an arbitrary target
// vs IsAdmin for the caller), so it is computed at the call site and passed in
// as isTargetAdmin. wrapPrefix and forbidMsg are likewise caller-supplied: the
// wrap prefix must attribute the CountAdmins failure to the calling module, not
// this helper, per the shared-helper rule in .claude/rules/error-wrapping.md.
func guardNotLastAdmin(
	ctx context.Context,
	isTargetAdmin bool,
	counter AdminCounter,
	wrapPrefix string,
	forbidMsg string,
) error {
	if !isTargetAdmin {
		return nil
	}
	n, err := counter.CountAdmins(ctx)
	if err != nil {
		if isContextDone(err) {
			return err
		}
		return eris.Wrap(err, wrapPrefix)
	}
	if domain.IsLastAdmin(n) {
		return ucerr.NewForbiddenError(forbidMsg)
	}
	return nil
}
