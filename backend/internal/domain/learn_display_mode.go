package domain

import (
	"strings"

	"github.com/rotisserie/eris"
)

// LearnDisplayMode is how the back of a flashcard is shown on /learn.
// The zero value is invalid; construct via ParseLearnDisplayMode.
type LearnDisplayMode string

const (
	// LearnDisplayFlipToReveal hides the back until the learner taps to reveal
	// it (active-recall default).
	LearnDisplayFlipToReveal LearnDisplayMode = "flip_to_reveal"
	// LearnDisplayAlwaysVisible shows front and back together from the start.
	LearnDisplayAlwaysVisible LearnDisplayMode = "always_visible"
)

// DefaultLearnDisplayMode is applied when a user has no stored preference.
const DefaultLearnDisplayMode = LearnDisplayFlipToReveal

// ParseLearnDisplayMode trims and validates s. An empty or unknown value is a
// caller error, not a silent fallback.
func ParseLearnDisplayMode(s string) (LearnDisplayMode, error) {
	switch LearnDisplayMode(strings.TrimSpace(s)) {
	case LearnDisplayFlipToReveal:
		return LearnDisplayFlipToReveal, nil
	case LearnDisplayAlwaysVisible:
		return LearnDisplayAlwaysVisible, nil
	default:
		return "", eris.Errorf("domain: invalid learn display mode %q", s)
	}
}

// String returns the persisted form.
func (m LearnDisplayMode) String() string { return string(m) }
