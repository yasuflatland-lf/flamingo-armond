package service

import (
	"strings"

	"backend/internal/domain"
)

// CEFRClassifier assigns a CEFR level to a card's front text using an Oxford
// word list. It first attempts an exact whole-string match (catches idioms and
// phrasal verbs stored as multi-word entries); a whole-string hit wins. It then
// falls back to per-token matching and keeps the hardest matching level. A
// front with no matching word classifies as CEFRUnknown.
type CEFRClassifier struct {
	words domain.CEFRWordList
}

// NewCEFRClassifier builds a classifier over the given word list. It panics on
// a nil word list — a classifier with no data is a wiring bug, never a runtime
// condition.
func NewCEFRClassifier(words domain.CEFRWordList) *CEFRClassifier {
	if words == nil {
		panic("service: CEFRClassifier requires a non-nil word list")
	}
	return &CEFRClassifier{words: words}
}

// Classify returns the CEFR level for front and whether any Oxford word
// matched. When several tokens match in the per-token path, the hardest
// (highest-ranked) level wins; a whole-string match returns immediately without
// consulting the per-token path. The bool is false (and the level CEFRUnknown)
// when nothing matches.
func (c *CEFRClassifier) Classify(front string) (domain.CEFRLevel, bool) {
	// Whole-string match first: a multi-word key like "give up" only matches
	// here, and a whole-string hit beats any token-level result.
	if whole := domain.NormalizeWord(front); whole != "" {
		if level, ok := c.words.Lookup(whole); ok {
			return level, true
		}
	}

	// Token-level match: keep the hardest level among matching tokens.
	best := domain.CEFRUnknown
	found := false
	for _, token := range strings.Fields(front) {
		key := domain.NormalizeWord(token)
		if key == "" {
			continue
		}
		if level, ok := c.words.Lookup(key); ok {
			best = best.Harder(level)
			found = true
		}
	}
	return best, found
}
