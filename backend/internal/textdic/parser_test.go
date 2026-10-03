// Package textdic_test exercises the goyacc grammar (grammar.y) and the
// hand-written lexer (lexer.go) end-to-end via the public Process API.
package textdic_test

import (
	"strings"
	"testing"

	"backend/internal/textdic"
)

func TestGrammar_FrontOnlyLineIsSkippedInPlace(t *testing.T) {
	t.Parallel()

	// A bare WORD line ("orphan") has no DEFINITION. The `entry: WORD`
	// production records a front-only skip and parsing continues on the
	// next line; subsequent lines must keep their original line numbers.
	input := "alpha " + defDog + "\n" +
		"orphan\n" +
		"beta " + defCat + "\n" +
		"gamma " + defBird + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}

	// The lines after the skipped row must be emitted with their original
	// line numbers.
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
			t.Errorf("expected %q to be parsed after the skipped row, but it was missing (got %+v)", want.front, words)
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

	// The malformed row must surface as a skipped validation error.
	if !hasValidationError(errs, 2, "skipped: front-only line (no definition)") {
		t.Errorf("expected skipped front-only line on line 2, got %+v", errs)
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
