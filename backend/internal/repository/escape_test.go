package repository

import "testing"

// TestEscapeLikePattern pins the output for a backslash: it is escaped exactly
// once, never re-escaped by a later rule. The literal-substring contract is
// TestEscapeLikePattern_Property_MatchesLiteralSubstring.
func TestEscapeLikePattern(t *testing.T) {
	if got := escapeLikePattern(`a\b`); got != `a\\b` {
		t.Errorf("escapeLikePattern(%q) = %q; want %q", `a\b`, got, `a\\b`)
	}
}
