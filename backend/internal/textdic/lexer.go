// Package textdic parses plain-text dictionary payloads of the form
// "<front-word> <back-definition>" pairs separated by newlines into
// structured ParsedWord records and per-line ValidationErrors.
//
// This file holds the hand-written rune scanner that feeds the
// goyacc-generated parser. It distinguishes ASCII front-word runs from
// Japanese-script definition runs and emits NEWLINE tokens so the grammar
// can anchor entries.
package textdic

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// ideographicSpace is U+3000, the fullwidth space that Japanese text
// commonly uses between front and back. unicode.IsSpace covers it today,
// but we test it explicitly so the predicate stays robust if a future Go
// release reclassifies the rune.
const ideographicSpace rune = 0x3000

// lexer holds the input reader and tracks two distinct line numbers:
// lineNo points at the line the next rune will be read from, while
// tokenLine records the line at which the most recently emitted token
// began (used to attribute parser errors back to the offending source).
type lexer struct {
	input     *strings.Reader
	lineNo    int
	tokenLine int
	errors    []error
}

func newLexer(input string) *lexer {
	return &lexer{input: strings.NewReader(input), lineNo: 1, tokenLine: 1}
}

// Peek returns the next rune without consuming any input. Restoration is
// offset-based (Seek to the saved position), so Peek does not depend on
// the underlying reader's UnreadRune semantics.
func (l *lexer) Peek() rune {
	offset, err := l.input.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0
	}
	r, _, readErr := l.input.ReadRune()
	if readErr != nil {
		return 0
	}
	l.input.Seek(offset, io.SeekStart) //nolint:errcheck
	return r
}

func (l *lexer) isNewLine(r rune) bool {
	return r == '\n' || (r == '\r' && l.Peek() == '\n')
}

// IsWhitespace reports whether r is regular Unicode whitespace or the
// fullwidth ideographic space (U+3000) commonly used in Japanese text.
func (l *lexer) IsWhitespace(r rune) bool {
	return unicode.IsSpace(r) || r == ideographicSpace
}

// isEnglishAndWhitespace recognises runes valid inside the front-word
// token. Hyphen, colon, semicolon, and backslash are allowed because
// existing dictionary payloads embed them in headwords (so-called,
// alternates, escape-prefixed forms); widening or narrowing this set
// should be a deliberate, separately reviewed change.
func (l *lexer) isEnglishAndWhitespace(r rune) bool {
	return unicode.IsLetter(r) && r < unicode.MaxASCII || l.IsWhitespace(r) || unicode.IsNumber(r) || strings.ContainsRune("-;:\\", r)
}

func (l *lexer) isJapanese(r rune) bool {
	return unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Han, r) || (r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFFEF)
}

// canStartDefinition widens the definition dispatch gate for ASCII
// bracket-openers commonly used before Japanese definitions. Once
// lexDefinition starts, it already accepts every non-newline rune.
func (l *lexer) canStartDefinition(r rune) bool {
	return l.isJapanese(r) || r == '(' || r == '['
}

// Lex implements the yyLexer interface. It returns 0 on EOF, NEWLINE on a
// line terminator, WORD on an ASCII headword run, and DEFINITION on a
// Japanese-script run. Read failures other than io.EOF and unrecognised
// runes are surfaced via l.Error so they appear in the validation errors.
func (l *lexer) Lex(lval *yySymType) int {
	r, err := l.skipWhiteSpace()
	if err == io.EOF {
		return 0
	}
	if err != nil {
		l.Error("read: " + err.Error())
		return 0
	}

	if l.isNewLine(r) {
		// Record the line that this NEWLINE terminates so a syntax error
		// attributed to it points at the correct source line, then advance.
		l.tokenLine = l.lineNo
		l.lineNo++
		return NEWLINE
	}
	l.tokenLine = l.lineNo
	if l.isEnglishAndWhitespace(r) {
		return l.lexWord(lval)
	}
	if l.canStartDefinition(r) {
		return l.lexDefinition(lval)
	}
	l.Error(fmt.Sprintf("unrecognized character %q", r))
	return l.recoverToNewline()
}

// lexRun reads runes until stop returns true (or EOF) and returns the
// trimmed accumulated string. The terminating rune is left unread so the
// caller can re-emit it (e.g. as a NEWLINE).
func (l *lexer) lexRun(stop func(rune) bool) string {
	var b strings.Builder
	l.input.UnreadRune()
	for {
		r, _, err := l.input.ReadRune()
		if err != nil {
			if err != io.EOF {
				// UnreadRune is only valid after a successful ReadRune,
				// so we cannot push the failure back; record and stop.
				l.Error("read: " + err.Error())
			}
			break
		}
		if stop(r) {
			l.input.UnreadRune()
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimRightFunc(b.String(), unicode.IsSpace)
}

func (l *lexer) lexWord(lval *yySymType) int {
	var b strings.Builder
	l.input.UnreadRune()
	var prev rune
	havePrev := false
	for {
		r, _, err := l.input.ReadRune()
		if err != nil {
			if err != io.EOF {
				l.Error("read: " + err.Error())
			}
			break
		}
		if l.isJapanese(r) || l.isNewLine(r) || ((r == '(' || r == '[') && havePrev && l.IsWhitespace(prev)) {
			l.input.UnreadRune()
			break
		}
		b.WriteRune(r)
		prev = r
		havePrev = true
	}
	lval.str = strings.TrimRightFunc(b.String(), unicode.IsSpace)
	lval.line = l.lineNo
	return WORD
}

func (l *lexer) lexDefinition(lval *yySymType) int {
	lval.str = l.lexRun(l.isNewLine)
	lval.line = l.lineNo
	return DEFINITION
}

// recoverToNewline consumes the rest of a malformed line and emits NEWLINE
// so the parser's error recovery can resume on the following line.
func (l *lexer) recoverToNewline() int {
	for {
		r, _, err := l.input.ReadRune()
		if err != nil {
			return 0
		}
		if l.isNewLine(r) {
			if r == '\r' {
				l.input.ReadRune() //nolint:errcheck
			}
			l.tokenLine = l.lineNo
			l.lineNo++
			return NEWLINE
		}
	}
}

// skipWhiteSpace advances past intra-line whitespace, leaving newline
// runes in place so they can be emitted as NEWLINE tokens.
func (l *lexer) skipWhiteSpace() (rune, error) {
	for {
		r, _, err := l.input.ReadRune()
		if err != nil || !l.IsWhitespace(r) || l.isNewLine(r) {
			return r, err
		}
	}
}

// Error records a structured error from the lexer. tokenLine reflects the
// start of the most recently emitted token, which is the right line to
// blame; lineNo may have advanced past any line terminator already.
func (l *lexer) Error(e string) {
	line := l.tokenLine
	if line < 1 {
		line = l.lineNo
	}
	l.errors = append(l.errors, parseError{Line: line, Message: e})
}
