package domain

import (
	"github.com/google/uuid"
	"github.com/rotisserie/eris"
)

// NewID returns a fresh UUID v7 string. Fails only when crypto/rand is unavailable.
// Callers MUST propagate the error; do not fall back to v4 (same RNG source).
// The wrap prefix uses the "domain:" layer token per the error-wrapping convention.
func NewID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", eris.Wrap(err, "domain: new uuid v7")
	}
	return id.String(), nil
}
