// Package textdic tests internal lexer behaviour that is not exercisable
// through the public Process API alone.
package textdic

import (
	"testing"
	"unicode"
)

// jp builds a string from raw rune code points so all CJK literals remain
// ASCII in committed source while the lexer sees real Unicode at runtime.
func jpRune(runes ...rune) string { return string(runes) }

var (
	lexDefDog  = jpRune(0x72AC) // han "dog"
	lexDefCat  = jpRune(0x732B) // han "cat"
	lexDefBird = jpRune(0x9CE5) // han "bird"
)

// TestLexer_PeekIsNonDestructive_CRLF verifies that calling Peek() before
// Lex() does not consume any input. The test checks that CRLF line endings
// produce the correct token sequence and line numbers regardless of how many
// times Peek is called beforehand.
func TestLexer_PeekIsNonDestructive_CRLF(t *testing.T) {
	t.Parallel()

	// Input has three entries separated by CRLF line endings.
	input := "alpha " + lexDefDog + "\r\n" +
		"beta " + lexDefCat + "\r\n" +
		"gamma " + lexDefBird + "\r\n"

	// Lex the input without any leading Peek calls to establish a baseline.
	wantTokens := lexAll(t, newLexer(input))

	// Lex again but call Peek() multiple times at the very start before any
	// Lex call. Peek must be a no-op w.r.t. the token stream.
	l2 := newLexer(input)
	for i := 0; i < 5; i++ {
		l2.Peek()
	}
	gotTokens := lexAll(t, l2)

	if len(gotTokens) != len(wantTokens) {
		t.Fatalf("token count: got %d want %d\ngot:  %+v\nwant: %+v",
			len(gotTokens), len(wantTokens), gotTokens, wantTokens)
	}
	for i, want := range wantTokens {
		got := gotTokens[i]
		if got.tok != want.tok || got.str != want.str || got.line != want.line {
			t.Errorf("token[%d]: got {tok:%d str:%q line:%d} want {tok:%d str:%q line:%d}",
				i, got.tok, got.str, got.line, want.tok, want.str, want.line)
		}
	}

	// Cross-check: words must appear on lines 1, 2, 3 respectively.
	wordLines := make([]int, 0, 3)
	for _, tk := range gotTokens {
		if tk.tok == WORD {
			wordLines = append(wordLines, tk.line)
		}
	}
	if len(wordLines) != 3 {
		t.Fatalf("expected 3 WORD tokens, got %d (%+v)", len(wordLines), gotTokens)
	}
	for i, want := range []int{1, 2, 3} {
		if wordLines[i] != want {
			t.Errorf("WORD[%d] line: got %d want %d", i, wordLines[i], want)
		}
	}
}

// TestLexer_PeekDoesNotAdvanceCursor verifies that Peek returns the correct
// rune and does not shift the read position so a subsequent ReadRune still
// sees the same rune.
func TestLexer_PeekDoesNotAdvanceCursor(t *testing.T) {
	t.Parallel()

	l := newLexer("hello")

	first := l.Peek()
	second := l.Peek()
	if first != second {
		t.Errorf("consecutive Peek calls returned different runes: %q vs %q", first, second)
	}
	if first != 'h' {
		t.Errorf("Peek returned %q, want 'h'", first)
	}

	// ReadRune must still return 'h' because Peek must not advance the cursor.
	r, _, err := l.input.ReadRune()
	if err != nil {
		t.Fatalf("ReadRune after Peek: %v", err)
	}
	if r != 'h' {
		t.Errorf("ReadRune after Peek returned %q, want 'h'", r)
	}
}

// tokenRecord is a minimal snapshot of one lexed token for comparison.
type tokenRecord struct {
	tok  int
	str  string
	line int
}

// lexAll drives the lexer to EOF and collects all emitted tokens.
func lexAll(t *testing.T, l *lexer) []tokenRecord {
	t.Helper()
	var out []tokenRecord
	for {
		var lval yySymType
		tok := l.Lex(&lval)
		if tok == 0 {
			break
		}
		// Normalise the string field: for NEWLINE tokens lval.str is empty,
		// which is fine. Strip trailing Unicode spaces to match lexRun behaviour.
		out = append(out, tokenRecord{
			tok:  tok,
			str:  trimRight(lval.str),
			line: lval.line,
		})
	}
	return out
}

func trimRight(s string) string {
	end := len(s)
	for end > 0 {
		r, size := lastRune(s[:end])
		if !unicode.IsSpace(r) {
			break
		}
		end -= size
	}
	return s[:end]
}

func lastRune(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}
	// Walk backwards to find the start of the last UTF-8 sequence.
	i := len(s) - 1
	for i > 0 && s[i]&0xC0 == 0x80 {
		i--
	}
	r := []rune(s[i:])[0]
	return r, len(s) - i
}
