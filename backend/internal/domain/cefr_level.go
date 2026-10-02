package domain

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// CEFRLevel is the Common European Framework of Reference level assigned to a
// word. Levels A1..C1 originate from the Oxford 3000/5000 word lists; C2 covers
// advanced vocabulary sourced from the Cambridge English Vocabulary Profile for
// words not present in the Oxford lists. The zero value (CEFRUnknown) means
// "not classified"; use a value in the A1..C2 set or check IsValid before
// relying on the level.
type CEFRLevel int

// Do not reorder: Rank and IsValid rely on the monotone ordering CEFRUnknown=0, A1=1 .. C2=6.
const (
	CEFRUnknown CEFRLevel = iota // zero value: not classified
	CEFRA1
	CEFRA2
	CEFRB1
	CEFRB2
	CEFRC1
	CEFRC2
)

// Rank returns the difficulty ordering (A1=1 .. C2=6); CEFRUnknown ranks 0.
// A higher rank is a harder word.
func (l CEFRLevel) Rank() int { return int(l) }

// IsValid reports whether l is one of the recognised A1..C2 levels.
func (l CEFRLevel) IsValid() bool { return l >= CEFRA1 && l <= CEFRC2 }

// String returns the canonical CEFR token ("A1".."C2"), or "" for CEFRUnknown.
func (l CEFRLevel) String() string {
	switch l {
	case CEFRA1:
		return "A1"
	case CEFRA2:
		return "A2"
	case CEFRB1:
		return "B1"
	case CEFRB2:
		return "B2"
	case CEFRC1:
		return "C1"
	case CEFRC2:
		return "C2"
	default:
		return ""
	}
}

// Harder returns the harder (higher-ranked) of l and other. It is the tie-break
// used when several candidate levels apply to one front string. Behaviour on a
// CEFRLevel constructed by raw int cast outside the A1..C2 range is unspecified.
func (l CEFRLevel) Harder(other CEFRLevel) CEFRLevel {
	if other.Rank() > l.Rank() {
		return other
	}
	return l
}

// ParseCEFRLevel maps a canonical token ("A1".."C2", case-insensitive,
// surrounding whitespace ignored) to a CEFRLevel. It returns
// (CEFRUnknown, false) for any unrecognised token.
func ParseCEFRLevel(s string) (CEFRLevel, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "A1":
		return CEFRA1, true
	case "A2":
		return CEFRA2, true
	case "B1":
		return CEFRB1, true
	case "B2":
		return CEFRB2, true
	case "C1":
		return CEFRC1, true
	case "C2":
		return CEFRC2, true
	default:
		return CEFRUnknown, false
	}
}

// NormalizeWord canonicalises a word or phrase for CEFR lookup: NFC, lowercase,
// NFC again, fold curly apostrophes to straight, trim leading/trailing
// whitespace, punctuation and symbols, and collapse each internal unicode.IsSpace
// run to one ASCII space. The markdown parser (keys) and the classifier
// (queries) share it so both agree; the result is idempotent.
func NormalizeWord(s string) string {
	s = norm.NFC.String(s)
	s = strings.ToLower(s)
	// Lowercasing can leave a base letter and a following combining mark that
	// NFC would compose (U+0130 lowers to plain "i"), so re-compose.
	s = norm.NFC.String(s)
	s = strings.ReplaceAll(s, "‘", "'")
	s = strings.ReplaceAll(s, "’", "'")
	s = strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	return strings.Join(strings.Fields(s), " ")
}

// CEFRWordList is the consumer-defined interface for the CEFR word-list
// lookup. The cefr adapter (*cefr.WordList) satisfies it implicitly. Lookup
// expects an already-normalized key (see NormalizeWord) and reports the level
// and whether the key was present.
type CEFRWordList interface {
	Lookup(normalizedWord string) (CEFRLevel, bool)
}
