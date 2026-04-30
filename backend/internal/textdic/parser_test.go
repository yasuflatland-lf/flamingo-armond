// Package textdic_test exercises the goyacc grammar (grammar.y) and the
// hand-written lexer (lexer.go) end-to-end via the public Process API.
// Direct grammar/lexer access is not required: each scenario is reachable
// from Process and that is the surface the rest of the backend depends on.
package textdic_test

import (
	"strings"
	"testing"

	"backend/internal/textdic"
)

// jpRunes builds Japanese-script content from raw rune code points. The
// repository's language policy forbids committing CJK literals, so tests
// construct inputs at runtime instead.
func jpRunes(runes ...rune) string { return string(runes) }

func TestGrammar_ErrorRecoveryStaysInBounds(t *testing.T) {
	t.Parallel()

	// A bare WORD line ("orphan") is malformed (no DEFINITION). The grammar
	// rule `entries : error NEWLINE` discards the offending entry and
	// resumes parsing on the next line. The key invariant: lines AFTER the
	// malformed row are still parsed normally and their line numbers stay
	// correct.
	dog := jpRunes(0x72AC)  // dog kanji
	cat := jpRunes(0x732B)  // cat kanji
	bird := jpRunes(0x9CE5) // bird kanji

	input := "alpha " + dog + "\n" +
		"orphan\n" +
		"beta " + cat + "\n" +
		"gamma " + bird + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}

	// At minimum the parser must recover and emit the lines that follow
	// the malformed row with their original line numbers intact.
	fronts := make(map[string]int, len(words))
	for _, w := range words {
		fronts[w.Front] = w.Line
	}

	for _, want := range []struct {
		front string
		line  int
	}{
		{"beta", 3},
		{"gamma", 4},
	} {
		got, ok := fronts[want.front]
		if !ok {
			t.Errorf("expected %q to be parsed after recovery, but it was missing (got %+v)", want.front, words)
			continue
		}
		if got != want.line {
			t.Errorf("%q line: got %d want %d", want.front, got, want.line)
		}
	}

	// "orphan" must never be reported as a successful word.
	if _, ok := fronts["orphan"]; ok {
		t.Errorf("malformed row should not yield a word, got %+v", words)
	}

	// The malformed row must surface as a validation error.
	if len(errs) == 0 {
		t.Errorf("expected at least one validation error for the malformed row")
	}
}

func TestGrammar_OnlyWhitespace(t *testing.T) {
	t.Parallel()

	// Whitespace-only input is non-empty so the early "empty payload" path
	// is bypassed. The grammar consumes the spaces/tabs without producing
	// any nodes; modern behaviour does not emit a "no nodes were parsed"
	// error in this case.
	input := "   \n  \t  \n  "

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 0 {
		t.Errorf("expected 0 words, got %d (%+v)", len(words), words)
	}

	for _, e := range errs {
		// The legacy "no nodes were parsed" diagnostic is not part of the
		// current contract for whitespace-only input.
		if strings.Contains(e.Message, "no nodes were parsed") {
			t.Errorf("unexpected legacy error message: %+v", e)
		}
		// And the empty-payload guard should not trigger either.
		if e.Message == "empty payload" {
			t.Errorf("empty-payload guard should not fire for whitespace-only input: %+v", e)
		}
	}
}

func TestLexer_LineNumberAfterCRLF(t *testing.T) {
	t.Parallel()

	// CRLF line endings must increment the line counter by exactly one per
	// terminator (isNewLine treats "\r\n" as a single line break).
	dog := jpRunes(0x72AC)
	cat := jpRunes(0x732B)
	bird := jpRunes(0x9CE5)

	input := "alpha " + dog + "\r\n" +
		"beta " + cat + "\r\n" +
		"gamma " + bird + "\r\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errs)
	}
	if len(words) != 3 {
		t.Fatalf("expected 3 words, got %d (%+v)", len(words), words)
	}

	wantLines := []int{1, 2, 3}
	for i, want := range wantLines {
		if words[i].Line != want {
			t.Errorf("words[%d].Line: got %d want %d", i, words[i].Line, want)
		}
	}
}
