package notion

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/xerrors"
)

type lexer struct {
	input *strings.Reader
}

func newLexer(input string) *lexer {
	return &lexer{input: strings.NewReader(input)}
}

func (l *lexer) Peek() rune {
	r, _, err := l.input.ReadRune()
	if err == nil {
		l.input.UnreadRune()
	}
	return r
}

func (l *lexer) isNewLine(r rune) bool {
	return r == '\n' || (r == '\r' && l.Peek() == '\n')
}

func (l *lexer) IsWhitespace(r rune) bool {
	return unicode.IsSpace(r) || regexp.MustCompile(`\x{3000}`).MatchString(string(r))
}

func (l *lexer) isEnglishAndWhitespace(r rune) bool {
	return unicode.IsLetter(r) && r < unicode.MaxASCII || l.IsWhitespace(r) || unicode.IsNumber(r) || strings.ContainsRune("-;:\\", r)
}

func (l *lexer) isJapanese(r rune) bool {
	return unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Han, r) || (r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFFEF)
}

func (l *lexer) Lex(lval *yySymType) int {
	for {
		r, err := l.skipWhiteSpace()
		if err != nil {
			return EOF
		}

		if l.isNewLine(r) {
			return NEWLINE
		}

		if l.isEnglishAndWhitespace(r) {
			return l.lexWord(lval)
		} else if l.isJapanese(r) {
			return l.lexDefinition(lval)
		}
	}
}

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
	return WORD
}

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
	return DEFINITION
}

func (l *lexer) skipWhiteSpace() (rune, error) {
	for {
		r, _, err := l.input.ReadRune()
		if err != nil || !l.IsWhitespace(r) || l.isNewLine(r) {
			return r, err
		}
	}
}

func (l *lexer) Error(e string) {
	xerrors.Errorf("error: %+v\n", e)
}
