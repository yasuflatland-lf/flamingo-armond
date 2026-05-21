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

// validateRelayArgs enforces Relay pagination argument coherence.
// The Relay spec pairs after with first (forward direction) and before with
// last (backward direction). The five guards below reject every other
// combination before any repository call is made, so callers never receive a
// silently re-interpreted page boundary.
//
// Returns a *ucerr.ValidationError on violation; nil otherwise.
func validateRelayArgs(first, last *int, after, before *string) error {
	// Mutual exclusion: cursors from opposite directions cannot coexist.
	if after != nil && before != nil {
		return ucerr.NewValidationError("after", "after and before are mutually exclusive")
	}
	// Direction mismatch: first (forward count) paired with before (backward cursor).
	if first != nil && *first > 0 && before != nil {
		return ucerr.NewValidationError("before", "before requires last, not first")
	}
	// Direction mismatch: last (backward count) paired with after (forward cursor).
	if last != nil && *last > 0 && after != nil {
		return ucerr.NewValidationError("after", "after requires first, not last")
	}
	// Cursor without companion count: page size and direction are unresolvable.
	if before != nil && (first == nil || *first <= 0) && (last == nil || *last <= 0) {
		return ucerr.NewValidationError("before", "before requires last")
	}
	if after != nil && (first == nil || *first <= 0) && (last == nil || *last <= 0) {
		return ucerr.NewValidationError("after", "after requires first")
	}
	return nil
}

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

// translateCardgroupNameErr maps domain sentinel errors from ParseCardgroupName
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
