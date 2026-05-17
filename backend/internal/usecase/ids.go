package usecase

import (
	"github.com/google/uuid"
	"github.com/rotisserie/eris"
)

// uuidV7 returns a new UUID v7 string. It fails only when crypto/rand is
// unavailable, in which case the system is already unhealthy and callers
// must surface the error rather than fall back to v4 (which uses the same
// rand source). See .claude/rules/go-library-gotchas.md § "uuid.NewV7
// failure must propagate".
func uuidV7() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", eris.Wrap(err, "uuid: NewV7 failed")
	}
	return id.String(), nil
}
