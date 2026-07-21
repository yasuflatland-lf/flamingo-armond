// Package gqlerrtest holds assertion helpers for inspecting GraphQL error
// codes from tests. It lives outside the gqlerr package on purpose: gqlerr's
// exported surface is the set of constructors production code returns, and a
// classifier that only tests ever call does not belong there. Import this
// package from _test.go files only.
package gqlerrtest

import (
	"errors"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"backend/internal/gqlerr"
)

// IsCode reports whether err is a *gqlerror.Error whose extensions.code equals
// code. An empty code never matches.
func IsCode(err error, code gqlerr.Code) bool {
	if code == "" {
		return false
	}
	var gqe *gqlerror.Error
	if !errors.As(err, &gqe) {
		return false
	}
	got, _ := gqe.Extensions["code"].(string)
	return got == string(code)
}
