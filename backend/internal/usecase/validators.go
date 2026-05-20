// Package usecase — validator helpers.
// translateErr functions consolidate domain-sentinel-to-ucerr translation in
// one file so the mapping logic is easy to audit and update in one place.
package usecase

import (
	"errors"
	"fmt"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/usecase/ucerr"
)

// translateBioErr maps domain Bio sentinels into usecase-layer typed errors.
// Unexpected errors are wrapped with eris. Returns nil when err is nil.
func translateBioErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrBioTooLong) {
		return ucerr.NewValidationError("bio", fmt.Sprintf("bio must be at most %d characters", domain.BioMax))
	}
	return eris.Wrap(err, "usecase: translate bio error")
}

// translateCardErr maps domain Card sentinels into usecase-layer typed errors.
// Unexpected errors are wrapped with eris.
func translateCardErr(err error) error {
	switch {
	case errors.Is(err, domain.ErrCardFrontRequired):
		return ucerr.NewValidationError("front", "front is required")
	case errors.Is(err, domain.ErrCardFrontTooLong):
		return ucerr.NewValidationError("front", fmt.Sprintf("front must be at most %d characters", domain.CardTextMax))
	case errors.Is(err, domain.ErrCardBackRequired):
		return ucerr.NewValidationError("back", "back is required")
	case errors.Is(err, domain.ErrCardBackTooLong):
		return ucerr.NewValidationError("back", fmt.Sprintf("back must be at most %d characters", domain.CardTextMax))
	default:
		return eris.Wrap(err, "usecase: translate card err: unexpected domain error")
	}
}

// translateCardgroupNameErr maps domain sentinel errors from Cardgroup.Validate
// to usecase-layer typed errors. Unexpected domain errors are wrapped with eris.
// Returns nil when err is nil.
func translateCardgroupNameErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrCardgroupNameRequired):
		return ucerr.NewValidationError("name", "name is required")
	case errors.Is(err, domain.ErrCardgroupNameTooLong):
		return ucerr.NewValidationError("name", fmt.Sprintf("name must be at most %d characters", domain.CardgroupNameMax))
	default:
		return eris.Wrap(err, "usecase: translate cardgroup name error")
	}
}

// translateDisplayNameErr maps domain DisplayName sentinels into usecase-layer
// typed errors. Unexpected errors are wrapped with eris. Returns nil when err
// is nil.
func translateDisplayNameErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrDisplayNameRequired):
		return ucerr.NewValidationError("displayName", "displayName is required")
	case errors.Is(err, domain.ErrDisplayNameTooLong):
		return ucerr.NewValidationError("displayName", fmt.Sprintf("displayName must be at most %d characters", domain.DisplayNameMax))
	default:
		return eris.Wrap(err, "usecase: translate display name error")
	}
}

// translateRoleNameErr maps domain RoleName sentinels into usecase-layer typed
// errors. Unexpected errors are wrapped with eris. Returns nil when err is nil.
func translateRoleNameErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrRoleNameRequired):
		return ucerr.NewValidationError("name", "name is required")
	case errors.Is(err, domain.ErrRoleNameTooLong):
		return ucerr.NewValidationError("name", fmt.Sprintf("name must be at most %d characters", domain.RoleNameMax))
	case errors.Is(err, domain.ErrRoleNameInvalid):
		return ucerr.NewValidationError("name", "name must contain only lowercase letters, digits, '_' or '-'")
	default:
		return eris.Wrap(err, "usecase: translate role name error")
	}
}
