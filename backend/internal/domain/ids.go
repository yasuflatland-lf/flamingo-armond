package domain

import (
	"github.com/google/uuid"
	"github.com/rotisserie/eris"
)

// newV7 is the indirection seam for tests; production code calls uuid.NewV7.
var newV7 = uuid.NewV7

// NewID returns a fresh UUID v7 string. Fails only when crypto/rand is
// unavailable. Callers MUST propagate the error; see `.claude/rules/go-library-gotchas.md`
// § "`uuid.NewV7` failure must propagate".
func NewID() (string, error) {
	id, err := newV7()
	if err != nil {
		return "", eris.Wrap(err, "domain: new uuid v7")
	}
	return id.String(), nil
}
