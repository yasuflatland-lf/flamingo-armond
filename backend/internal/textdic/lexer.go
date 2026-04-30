// Package textdic lexer: hand-written rune scanner that feeds the
// goyacc-generated parser. It distinguishes ASCII "word" runs (front)
// from Japanese script runs (definition / back) and emits NEWLINE tokens
// so the grammar can anchor entries.
package textdic

import (
	"io"
	"regexp"
	"strings"
	"unicode"
)

// lexer holds the input reader, the current line number for error
// reporting, and a slice of structured errors gathered during scanning.
type lexer struct {
	input  *strings.Reader
	lineNo int
	errors []error
}

// newLexer constructs a lexer positioned at line 1.
func newLexer(input string) *lexer {
	return &lexer{input: strings.NewReader(input), lineNo: 1}
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
	return unicode.IsSpace(r) || regexp.MustCompile(`\x{3000}`).MatchString(string(r))
}

// isEnglishAndWhitespace recognises runes valid inside the front-word
// token: ASCII letters, whitespace, digits, and a small set of
// punctuation characters that may appear in dictionary headwords.
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
// ASCII headword run, and DEFINITION on a Japanese-script run.
func (l *lexer) Lex(lval *yySymType) int {
	r, err := l.skipWhiteSpace()
	if err == io.EOF {
		// End of input.
		return 0
	}

	if l.isNewLine(r) {
		l.lineNo++
		return NEWLINE
	}
	if l.isEnglishAndWhitespace(r) {
		return l.lexWord(lval)
	}
	if l.isJapanese(r) {
		return l.lexDefinition(lval)
	}
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
		if err != nil || l.isJapanese(r) || l.isNewLine(r) {
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
		if err != nil || l.isNewLine(ch) {
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

// Error records a structured error from the lexer. The goyacc parser also
// calls Error on yyParserImpl; both sources are merged by parserWrapper.
func (l *lexer) Error(e string) {
	err := parseError{Line: l.lineNo, Message: e}
	l.errors = append(l.errors, err)
}

// GetErrors returns the structured errors collected during scanning.
func (l *lexer) GetErrors() []error {
	return l.errors
}
