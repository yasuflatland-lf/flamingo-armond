package usecase

import (
	"errors"
	"fmt"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// validateRelayArgs enforces Relay pagination argument coherence.
// The Relay spec pairs after with first (forward direction) and before with
// last (backward direction). The five guards below reject every other
// combination so callers never receive a silently re-interpreted page boundary.
// Callers should invoke this before any repository call so invalid arguments
// are rejected early.
//
// Returns a *ucerr.ValidationError on violation; nil otherwise.
func validateRelayArgs(first, last *int, after, before *string) error {
	if after != nil && before != nil {
		return ucerr.NewValidationError("after", "after and before are mutually exclusive")
	}
	if first != nil && *first > 0 && before != nil {
		return ucerr.NewValidationError("before", "before requires last, not first")
	}
	if last != nil && *last > 0 && after != nil {
		return ucerr.NewValidationError("after", "after requires first, not last")
	}
	if before != nil && (first == nil || *first <= 0) && (last == nil || *last <= 0) {
		return ucerr.NewValidationError("before", "before requires last")
	}
	if after != nil && (first == nil || *first <= 0) && (last == nil || *last <= 0) {
		return ucerr.NewValidationError("after", "after requires first")
	}
	return nil
}

// translateTrinaryTextErr maps a domain trinary-text sentinel (Bio / Description
// too-long) into a usecase-layer typed error. It is the shared body of
// translateBioErr and translateDescriptionErr: err == nil returns nil; a match on
// tooLong returns a field-scoped ValidationError whose message references max; any
// other error is wrapped with the caller-supplied prefix (the prefix travels from
// the caller so the error_chain names the calling field's module, per the shared-
// helper wrap convention).
func translateTrinaryTextErr(err error, field string, max int, tooLong error, wrap string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, tooLong) {
		return ucerr.NewValidationError(field, fmt.Sprintf("%s must be at most %d characters", field, max))
	}
	return eris.Wrap(err, wrap)
}

// translateBioErr maps domain Bio sentinels into usecase-layer typed errors.
// Unexpected errors are wrapped with eris. Returns nil when err is nil.
func translateBioErr(err error) error {
	return translateTrinaryTextErr(err, "bio", domain.BioMax, domain.ErrBioTooLong, "usecase: translate bio error")
}

// translateCardErr maps domain Card sentinels into usecase-layer typed errors.
// Unexpected errors are wrapped with eris. Returns nil when err is nil.
func translateCardErr(err error) error {
	if err == nil {
		return nil
	}
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

// translateTextLengthViolation maps a *repository.TextLengthViolationError -- the
// database-side CHECK backstop on a "<table>_<column>_length" constraint
// (SQLSTATE 23514) -- to a field-scoped BAD_USER_INPUT validation error keyed on
// the column the constraint guards ("front", "back", "name").
//
// It returns nil when err is not a text-length violation, so callers use it as a
// pre-filter before their existing eris.Wrap. The domain layer enforces the
// user-visible cap in grapheme clusters and the database bound is a wide
// multiple of it, so this path fires only for pathological combining-mark input;
// classifying it as BAD_USER_INPUT rather than INTERNAL means the learner sees a
// length message instead of an unexplained failure.
func translateTextLengthViolation(err error) error {
	if v, ok := errors.AsType[*repository.TextLengthViolationError](err); ok {
		return ucerr.NewValidationError(v.Field, fmt.Sprintf("%s is too long", v.Field))
	}
	return nil
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

// translateDescriptionErr maps domain Description sentinels into usecase-layer
// typed errors. Unexpected errors are wrapped with eris. Returns nil when err
// is nil.
func translateDescriptionErr(err error) error {
	return translateTrinaryTextErr(err, "description", domain.DescriptionMax, domain.ErrDescriptionTooLong, "usecase: translate description error")
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
	case errors.Is(err, domain.ErrDisplayNameReserved):
		return ucerr.NewValidationError("displayName", "displayName is reserved")
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

// SentinelMapping maps a repository error sentinel to an input-validation field/message pair.
type SentinelMapping struct {
	Sentinel error
	Field    string
	Message  string
}

// classifyRepoErr classifies a repository error sentinel set into either input-validation data
// or a propagating error. The mappings list defines the sentinel-to-field/message mapping;
// if no mapping matches, context-done errors pass through unchanged and other errors are
// wrapped with the supplied prefix per error-wrapping conventions.
func classifyRepoErr(err error, wrap string, mappings []SentinelMapping) (*InputValidationInfo, error) {
	for _, m := range mappings {
		if errors.Is(err, m.Sentinel) {
			return NewInputValidationInfo(m.Field, m.Message), nil
		}
	}
	if isContextDone(err) {
		return nil, err
	}
	return nil, eris.Wrap(err, wrap)
}
