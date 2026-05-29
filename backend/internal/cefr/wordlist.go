package cefr

import (
	"embed"
	"strings"

	"backend/internal/domain"

	"github.com/rotisserie/eris"
)

//go:embed data/oxford-3000.md data/oxford-5000.md
var dataFS embed.FS

// WordList is an in-memory Oxford 3000/5000 lookup table keyed by normalized
// word/phrase. It implements domain.CEFRWordList.
type WordList struct {
	levels map[string]domain.CEFRLevel
}

// Lookup returns the level for an already-normalized key. Callers normalize via
// domain.NormalizeWord before calling.
func (w *WordList) Lookup(normalizedWord string) (domain.CEFRLevel, bool) {
	level, ok := w.levels[normalizedWord]
	return level, ok
}

// Len reports the number of distinct keys loaded. Used by tests and sanity
// checks.
func (w *WordList) Len() int { return len(w.levels) }

// NewWordList parses both embedded Oxford word lists and merges them into one
// lookup table; on a duplicate key the harder level wins. It panics if either
// embedded file is missing, malformed, or empty — the data is compiled in, so
// any failure is a build/release defect, never a runtime condition.
func NewWordList() *WordList {
	merged := make(map[string]domain.CEFRLevel)
	for _, name := range []string{"data/oxford-3000.md", "data/oxford-5000.md"} {
		raw, err := dataFS.ReadFile(name)
		if err != nil {
			panic("cefr: read embedded word list " + name + ": " + err.Error())
		}
		parsed, err := ParseMarkdown(string(raw))
		if err != nil {
			panic("cefr: parse embedded word list " + name + ": " + err.Error())
		}
		for key, level := range parsed {
			merged[key] = merged[key].Harder(level)
		}
	}
	if len(merged) == 0 {
		panic("cefr: embedded word lists produced zero entries")
	}
	return &WordList{levels: merged}
}

// ParseMarkdown reads a `## <LEVEL>` / `- <key>` formatted word list and returns
// a map from normalized word/phrase to CEFR level. A `## <LEVEL>` heading sets
// the active level for subsequent `-` bullets. Each bullet's text is normalized
// via domain.NormalizeWord. Text from the first `_(` annotation marker onward,
// if present, is dropped (accommodates part-of-speech tags such as `_(n.)` in
// alternative source formats). It returns an error on a bullet before any
// heading, or on an unrecognised level token.
func ParseMarkdown(src string) (map[string]domain.CEFRLevel, error) {
	out := make(map[string]domain.CEFRLevel)
	current := domain.CEFRUnknown
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			level, valid := domain.ParseCEFRLevel(rest)
			if !valid {
				return nil, eris.Errorf("cefr: unrecognised level heading %q", rest)
			}
			current = level
			continue
		}
		if rest, ok := strings.CutPrefix(line, "- "); ok {
			if !current.IsValid() {
				return nil, eris.Errorf("cefr: word %q appears before any level heading", rest)
			}
			if idx := strings.Index(rest, "_("); idx >= 0 {
				rest = rest[:idx]
			}
			key := domain.NormalizeWord(rest)
			if key == "" {
				continue
			}
			out[key] = out[key].Harder(current)
		}
	}
	return out, nil
}
