// Package usecase — authentication guard helpers.
// requireCallerSub centralises the nil-or-empty-Sub check so every usecase
// method can share the same guard without repeating the two-condition predicate.
package usecase

import (
	"backend/internal/auth"
	"backend/internal/usecase/ucerr"
)

// requireCallerSub returns ucerr.ErrUnauthenticated if caller is nil or has an
// empty Sub claim. Call sites should return their own zero values alongside the
// returned error, for example:
//
//	if err := requireCallerSub(caller); err != nil {
//	    return nil, err
//	}
func requireCallerSub(caller *auth.AuthUser) error {
	if caller == nil || caller.Sub == "" {
		return ucerr.ErrUnauthenticated
	}
	return nil
}
