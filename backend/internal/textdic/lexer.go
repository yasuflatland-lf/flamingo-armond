// Package textdic parses plain-text dictionary payloads of the form
// "<front-word> <back-definition>" pairs separated by newlines into
// structured ParsedWord records and per-line ValidationErrors.
//
// This file holds the hand-written rune scanner that feeds the
// goyacc-generated parser. It distinguishes ASCII "word" runs (front)
// from Japanese script runs (definition / back) and emits NEWLINE tokens
// so the grammar can anchor entries.
package textdic

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// ideographicSpace is U+3000, the fullwidth space that Japanese text
// commonly uses between front and back. unicode.IsSpace covers it in
// current Unicode tables, but we test it explicitly so the predicate
// stays robust if a future Go release reclassifies the rune.
const ideographicSpace rune = 0x3000

// lexer holds the input reader, the current line number for error
// reporting, and a slice of structured errors gathered during scanning.
//
// lineNo tracks the line that the *next* rune will be read from, so it is
// incremented immediately after consuming a line terminator. tokenLine
// records the line at which the most recently emitted token began, which
// is the value parser-stage Error callbacks should report (the parser
// sees the offending token, not what comes after it).
type lexer struct {
	input     *strings.Reader
	lineNo    int
	tokenLine int
	errors    []error
}

// newLexer constructs a lexer positioned at line 1.
func newLexer(input string) *lexer {
	return &lexer{input: strings.NewReader(input), lineNo: 1, tokenLine: 1}
}

// Peek returns the next rune without advancing the read position.
func (l *lexer) Peek() rune {
	r, _, err := l.input.ReadRune()
	if err == nil {
		l.input.UnreadRune()
	}
	return r
}

// isNewLine reports whether r begins a line terminator (LF or CRLF).
func (l *lexer) isNewLine(r rune) bool {
	return r == '\n' || (r == '\r' && l.Peek() == '\n')
}

// IsWhitespace reports whether r is regular Unicode whitespace or the
// fullwidth ideographic space (U+3000) commonly used in Japanese text.
func (l *lexer) IsWhitespace(r rune) bool {
	return unicode.IsSpace(r) || r == ideographicSpace
}

// isEnglishAndWhitespace recognises runes valid inside the front-word
// token: ASCII letters, whitespace, digits, and a small set of
// punctuation characters that may appear in dictionary headwords.
//
// Allows hyphen ('-'), colon (':'), semicolon (';') and backslash ('\\')
// because source dictionaries embed them in headwords (for example
// 'so-called', list separators between alternates, and escape-prefixed
// alternate forms). Existing payloads rely on this set; widening or
// narrowing it should be a deliberate, separately reviewed change.
func (l *lexer) isEnglishAndWhitespace(r rune) bool {
	return unicode.IsLetter(r) && r < unicode.MaxASCII || l.IsWhitespace(r) || unicode.IsNumber(r) || strings.ContainsRune("-;:\\", r)
}

// isJapanese reports whether r is part of the Japanese writing system.
// Hiragana, Katakana, Han ideographs, and the CJK symbol/halfwidth ranges
// are all considered definition (back) content.
func (l *lexer) isJapanese(r rune) bool {
	return unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Han, r) || (r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFFEF)
}

// Lex implements the yyLexer interface used by the goyacc-generated
// parser. It returns 0 on EOF, NEWLINE on a line terminator, WORD on an
// ASCII headword run, and DEFINITION on a Japanese-script run. Read
// failures other than io.EOF and unrecognised runes are surfaced via
// l.Error so they appear in the aggregated validation error list rather
// than being silently dropped at end-of-input.
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
		// Record the line that this NEWLINE token terminates so a later
		// syntax error attributed to the NEWLINE points at the correct
		// source line, then advance lineNo for the next token.
		l.tokenLine = l.lineNo
		l.lineNo++
		return NEWLINE
	}
	l.tokenLine = l.lineNo
	if l.isEnglishAndWhitespace(r) {
		return l.lexWord(lval)
	}
	if l.isJapanese(r) {
		return l.lexDefinition(lval)
	}
	l.Error(fmt.Sprintf("unrecognized character %q", r))
	return 0
}

// lexWord reads a contiguous run of front-word runes. It stops at EOF, at
// any Japanese rune (which begins the definition), or at a newline. The
// trailing whitespace is trimmed before yielding the token. The current
// line number is attached to lval so the grammar can stamp it on the Node.
func (l *lexer) lexWord(lval *yySymType) int {
	var wordBuilder strings.Builder
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
		if l.isJapanese(r) || l.isNewLine(r) {
			l.input.UnreadRune()
			break
		}
		wordBuilder.WriteRune(r)
	}
	lval.str = strings.TrimRightFunc(wordBuilder.String(), unicode.IsSpace)
	lval.line = l.lineNo
	return WORD
}

// lexDefinition reads runes until the next newline or EOF and returns a
// DEFINITION token. The line number is recorded for safety, although the
// grammar primarily reads the line off the preceding WORD.
func (l *lexer) lexDefinition(lval *yySymType) int {
	var defBuilder strings.Builder
	l.input.UnreadRune()
	for {
		ch, _, err := l.input.ReadRune()
		if err != nil {
			if err != io.EOF {
				l.Error("read: " + err.Error())
			}
			break
		}
		if l.isNewLine(ch) {
			l.input.UnreadRune()
			break
		}
		defBuilder.WriteRune(ch)
	}
	lval.str = strings.TrimRightFunc(defBuilder.String(), unicode.IsSpace)
	lval.line = l.lineNo
	return DEFINITION
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

// Error records a structured error from the lexer. The goyacc parser
// dispatches its yylex.Error calls here too (via the yyLexer interface),
// so we attribute the error to tokenLine - the line at which the most
// recently emitted token began - rather than lineNo, which has already
// advanced past any line terminator the parser was reacting to.
func (l *lexer) Error(e string) {
	line := l.tokenLine
	if line < 1 {
		line = l.lineNo
	}
	err := parseError{Line: line, Message: e}
	l.errors = append(l.errors, err)
}

// GetErrors returns the structured errors collected during scanning.
func (l *lexer) GetErrors() []error {
	return l.errors
}
