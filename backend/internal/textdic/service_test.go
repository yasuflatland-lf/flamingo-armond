// Package textdic_test exercises the public Process entrypoint exposed
// by service.go.
package textdic_test

import (
	"strings"
	"sync"
	"testing"

	"backend/internal/textdic"
)

// jp builds a Japanese-script string from raw rune code points so committed
// source stays ASCII-only (per the repository language policy) while the
// lexer sees real Hiragana/Katakana/Han runes at runtime.
func jp(runes ...rune) string { return string(runes) }

// Japanese fragments shared across tests. Built from raw code points so all
// CJK literals stay auditable in one place.
var (
	defRingo = jp(0x308A, 0x3093, 0x3054) // hiragana "ringo" (apple)
	defDog   = jp(0x72AC)                 // han "dog"
	defCat   = jp(0x732B)                 // han "cat"
	defBird  = jp(0x9CE5)                 // han "bird"
	defFish  = jp(0x9B5A)                 // han "fish"
	defBook  = jp(0x672C)                 // han "book"
)

func TestProcess_HappyPath(t *testing.T) {
	t.Parallel()

	// "apple" + U+3000 (ideographic space) + ringo. The lexer treats U+3000
	// as whitespace, so the front token is "apple" and the back is ringo.
	input := "apple" + jp(0x3000) + defRingo

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errs)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word, got %d (%+v)", len(words), words)
	}
	if words[0].Front != "apple" {
		t.Errorf("Front: got %q want %q", words[0].Front, "apple")
	}
	if words[0].Back != defRingo {
		t.Errorf("Back: got %q want %q", words[0].Back, defRingo)
	}
	if words[0].Line != 1 {
		t.Errorf("Line: got %d want 1", words[0].Line)
	}
}

func TestProcess_EmptyPayload(t *testing.T) {
	t.Parallel()

	words, errs, err := textdic.Process("")
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 0 {
		t.Errorf("expected 0 words, got %d", len(words))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 validation error, got %d (%+v)", len(errs), errs)
	}
	if errs[0].Line != 1 {
		t.Errorf("Line: got %d want 1", errs[0].Line)
	}
	if errs[0].Message != "empty payload" {
		t.Errorf("Message: got %q want %q", errs[0].Message, "empty payload")
	}
}

func TestProcess_OversizedPayload(t *testing.T) {
	t.Parallel()

	// 1 MiB + 1 byte exceeds maxPayloadBytes.
	input := strings.Repeat("a", (1<<20)+1)

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if words != nil && len(words) != 0 {
		t.Errorf("expected no words, got %d", len(words))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 validation error, got %d (%+v)", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "payload exceeds") {
		t.Errorf("expected message to mention 'payload exceeds', got %q", errs[0].Message)
	}
}

func TestProcess_LineTracking(t *testing.T) {
	t.Parallel()

	// Line 1: word, Line 2: blank, Line 3: word.
	input := "apple " + defRingo + "\n\n" + "dog " + defDog + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errs)
	}
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d (%+v)", len(words), words)
	}
	if words[0].Line != 1 {
		t.Errorf("words[0].Line: got %d want 1", words[0].Line)
	}
	if words[1].Line != 3 {
		t.Errorf("words[1].Line: got %d want 3", words[1].Line)
	}
}

func TestProcess_BlankLines(t *testing.T) {
	t.Parallel()

	// Leading, interior, and trailing blank lines must not influence the
	// number of parsed words.
	input := "\n\n" +
		"cat " + defCat + "\n" +
		"\n" +
		"  \n" +
		"dog " + defDog + "\n" +
		"\n\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d (%+v)", len(words), words)
	}
	if len(errs) != 0 {
		t.Errorf("expected no validation errors, got %+v", errs)
	}
	if words[0].Front != "cat" || words[0].Back != defCat {
		t.Errorf("words[0]: got %+v", words[0])
	}
	if words[1].Front != "dog" || words[1].Back != defDog {
		t.Errorf("words[1]: got %+v", words[1])
	}
}

func TestProcess_MalformedRows(t *testing.T) {
	t.Parallel()

	// The first line is malformed (WORD with no DEFINITION); the grammar's
	// "error NEWLINE" recovery rule should discard it. Subsequent valid
	// lines must still be parsed.
	input := "apple\n" +
		"dog " + defDog + "\n" +
		"cat " + defCat + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("expected 2 words after recovery, got %d (%+v)", len(words), words)
	}
	if len(errs) == 0 {
		t.Errorf("expected at least one validation error for the malformed row")
	}
	for _, w := range words {
		if w.Front == "apple" {
			t.Errorf("malformed entry should not yield a word: %+v", w)
		}
	}
}

func TestProcess_RecoversFromUnrecognizedLine(t *testing.T) {
	t.Parallel()

	input := "good " + defDog + "\n" +
		"@broken\n" +
		"fine " + defCat + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("expected 2 words after recovery, got %d (%+v)", len(words), words)
	}
	if words[0].Front != "good" {
		t.Errorf("words[0].Front: got %q want %q", words[0].Front, "good")
	}
	if words[1].Front != "fine" {
		t.Errorf("words[1].Front: got %q want %q", words[1].Front, "fine")
	}
	if !hasValidationError(errs, 2, "unrecognized character '@'") {
		t.Errorf("expected unrecognized-character error on line 2, got %+v", errs)
	}
}

func TestProcess_RecoversFromUnrecognizedLineWithCRLF(t *testing.T) {
	t.Parallel()

	input := "good " + defDog + "\r\n" +
		"@broken\r\n" +
		"fine " + defCat + "\r\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("expected 2 words after recovery, got %d (%+v)", len(words), words)
	}
	if words[0].Line != 1 {
		t.Errorf("words[0].Line: got %d want 1", words[0].Line)
	}
	if words[1].Line != 3 {
		t.Errorf("words[1].Line: got %d want 3", words[1].Line)
	}
	if !hasValidationError(errs, 2, "unrecognized character '@'") {
		t.Errorf("expected unrecognized-character error on line 2, got %+v", errs)
	}
}

func TestProcess_RecoversWhenFirstLineHasUnrecognizedChar(t *testing.T) {
	t.Parallel()

	input := "@broken\n" +
		"good " + defDog + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word after recovery, got %d (%+v)", len(words), words)
	}
	if words[0].Front != "good" {
		t.Errorf("words[0].Front: got %q want %q", words[0].Front, "good")
	}
	if !hasValidationError(errs, 1, "unrecognized character '@'") {
		t.Errorf("expected unrecognized-character error on line 1, got %+v", errs)
	}
}

func TestProcess_RecoversFromMultipleUnrecognizedChars(t *testing.T) {
	t.Parallel()

	input := "a " + defDog + "\n" +
		"@broken\n" +
		"c " + defCat + "\n" +
		"{broken\n" +
		"e " + defBird + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 3 {
		t.Fatalf("expected 3 words after recovery, got %d (%+v)", len(words), words)
	}
	for i, want := range []string{"a", "c", "e"} {
		if words[i].Front != want {
			t.Errorf("words[%d].Front: got %q want %q", i, words[i].Front, want)
		}
	}
	if !hasValidationError(errs, 2, "unrecognized character '@'") {
		t.Errorf("expected unrecognized-character error on line 2, got %+v", errs)
	}
	if !hasValidationError(errs, 4, "unrecognized character '{'") {
		t.Errorf("expected unrecognized-character error on line 4, got %+v", errs)
	}
}

func TestProcess_UnrecognizedCharAtEOFWithoutNewline(t *testing.T) {
	t.Parallel()

	input := "good " + defDog + "\n" +
		"@"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word before EOF error, got %d (%+v)", len(words), words)
	}
	if words[0].Front != "good" {
		t.Errorf("words[0].Front: got %q want %q", words[0].Front, "good")
	}
	if !hasValidationError(errs, 2, "unrecognized character '@'") {
		t.Errorf("expected unrecognized-character error on line 2, got %+v", errs)
	}
}

func TestProcess_BackStartingWithHalfWidthParen(t *testing.T) {
	t.Parallel()

	wantBack := "(" + jp(0x5BB6, 0x5EAD, 0x306E) + ")" + jp(0x5927, 0x9ED2, 0x67F1)
	input := "primary breadwinner " + wantBack + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errs)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word, got %d (%+v)", len(words), words)
	}
	if words[0].Front != "primary breadwinner" {
		t.Errorf("Front: got %q want %q", words[0].Front, "primary breadwinner")
	}
	if words[0].Back != wantBack {
		t.Errorf("Back: got %q want %q", words[0].Back, wantBack)
	}
}

func TestProcess_BackStartingWithHalfWidthBracket(t *testing.T) {
	t.Parallel()

	wantBack := "[" + jp(0x52D5, 0x8A5E) + "] " + jp(0x53D6, 0x308A, 0x6271, 0x3046)
	input := "transitive " + wantBack + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errs)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word, got %d (%+v)", len(words), words)
	}
	if words[0].Front != "transitive" {
		t.Errorf("Front: got %q want %q", words[0].Front, "transitive")
	}
	if words[0].Back != wantBack {
		t.Errorf("Back: got %q want %q", words[0].Back, wantBack)
	}
}

func TestProcess_BackWithEmbeddedParensStillWorks(t *testing.T) {
	t.Parallel()

	wantBack := defDog + "(test)"
	input := "ritual " + wantBack + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errs)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word, got %d (%+v)", len(words), words)
	}
	if words[0].Back != wantBack {
		t.Errorf("Back: got %q want %q", words[0].Back, wantBack)
	}
}

func TestProcess_FrontWithEmbeddedParenStillWorks(t *testing.T) {
	t.Parallel()

	input := "take(s) " + defDog + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errs)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word, got %d (%+v)", len(words), words)
	}
	if words[0].Front != "take(s)" {
		t.Errorf("Front: got %q want %q", words[0].Front, "take(s)")
	}
}

func TestProcess_LineWithOnlyParenBackIsRejected(t *testing.T) {
	t.Parallel()

	input := "(comment) only\n" +
		"fine " + defCat + "\n"

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(words) != 1 {
		t.Fatalf("expected 1 word after recovery, got %d (%+v)", len(words), words)
	}
	if words[0].Front != "fine" {
		t.Errorf("words[0].Front: got %q want %q", words[0].Front, "fine")
	}
	if len(errs) == 0 {
		t.Errorf("expected at least one validation error for the headword-less row")
	}
}

func TestProcess_MultipleEntries(t *testing.T) {
	t.Parallel()

	// Five-entry sample modelled on the legacy parser_test.go golden input.
	input := strings.Join([]string{
		"apple " + defRingo,
		"dog " + defDog,
		"cat " + defCat,
		"bird " + defBird,
		"fish " + defFish,
		"book " + defBook,
	}, "\n") + "\n"

	want := []textdic.ParsedWord{
		{Front: "apple", Back: defRingo, Line: 1},
		{Front: "dog", Back: defDog, Line: 2},
		{Front: "cat", Back: defCat, Line: 3},
		{Front: "bird", Back: defBird, Line: 4},
		{Front: "fish", Back: defFish, Line: 5},
		{Front: "book", Back: defBook, Line: 6},
	}

	words, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errs)
	}
	if len(words) != len(want) {
		t.Fatalf("expected %d words, got %d (%+v)", len(want), len(words), words)
	}
	for i, w := range want {
		if words[i] != w {
			t.Errorf("words[%d]: got %+v want %+v", i, words[i], w)
		}
	}
}

func TestProcess_OversizedPayloadLineZero(t *testing.T) {
	t.Parallel()

	// Payload-size violations are not tied to a particular source line; the
	// resolver contract uses Line == 0 as the file-level error marker.
	input := strings.Repeat("a", (1<<20)+1)

	_, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 validation error, got %d (%+v)", len(errs), errs)
	}
	if errs[0].Line != 0 {
		t.Errorf("Line: got %d want 0 (file-level error marker)", errs[0].Line)
	}
}

func TestProcess_PayloadAtBoundary(t *testing.T) {
	t.Parallel()

	// 1 MiB exactly must be accepted (off-by-one guard around maxPayloadBytes).
	// "apple " + defRingo + "\n" repeats produce well-formed entries; we pad
	// the tail with blank lines so the byte length lands precisely on 1 MiB.
	const maxBytes = 1 << 20
	entry := "apple " + defRingo + "\n"
	repeats := maxBytes / len(entry)
	body := strings.Repeat(entry, repeats)
	pad := maxBytes - len(body)
	if pad > 0 {
		body += strings.Repeat("\n", pad)
	}
	if len(body) != maxBytes {
		t.Fatalf("test setup: expected exactly %d bytes, got %d", maxBytes, len(body))
	}

	words, errs, err := textdic.Process(body)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	for _, e := range errs {
		if strings.Contains(e.Message, "payload exceeds") {
			t.Fatalf("1 MiB exact payload should be accepted, got size error: %+v", e)
		}
	}
	if len(words) != repeats {
		t.Errorf("expected %d parsed words, got %d", repeats, len(words))
	}
}

func TestProcess_InvalidUTF8(t *testing.T) {
	t.Parallel()

	// Inputs with invalid UTF-8 byte sequences must not panic and must not
	// be silently swallowed. The lexer surfaces the read failure via the
	// validation-error channel; the call returns normally.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Process panicked on invalid UTF-8: %v", r)
		}
	}()

	// "apple " + DEFINITION + invalid trailing bytes. The leading entry is
	// well-formed so we exercise both successful tokens and a recorded
	// scanner error in the same call.
	input := "apple " + defRingo + "\n" + "\xff\xfe\xfd"

	_, _, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
}

func TestProcess_SyntaxErrorOnLine2(t *testing.T) {
	t.Parallel()

	// Line 1 is well-formed; line 2 is malformed (bare WORD with no
	// DEFINITION). The grammar should report the syntax error against
	// line 2, not against line 1 (the previous default).
	input := "apple " + defRingo + "\n" +
		"orphan\n"

	_, errs, err := textdic.Process(input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatalf("expected at least one validation error for the malformed row")
	}

	// At least one syntax-style error must point at line 2.
	var sawLine2 bool
	for _, e := range errs {
		if e.Line == 2 {
			sawLine2 = true
			break
		}
	}
	if !sawLine2 {
		t.Errorf("expected a validation error on line 2, got %+v", errs)
	}
}

func TestProcess_Concurrent(t *testing.T) {
	t.Parallel()

	// Process serialises internally via processMu, but the test still
	// exercises the locking under -race to catch any regressions.
	input := "apple " + defRingo + "\n" + "dog " + defDog + "\n"

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			words, errs, err := textdic.Process(input)
			if err != nil {
				errCh <- err
				return
			}
			if len(errs) != 0 {
				t.Errorf("unexpected validation errors: %+v", errs)
				return
			}
			if len(words) != 2 {
				t.Errorf("expected 2 words, got %d", len(words))
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for e := range errCh {
		t.Errorf("goroutine reported error: %v", e)
	}
}

func hasValidationError(errs []textdic.ValidationError, line int, contains string) bool {
	for _, e := range errs {
		if e.Line == line && strings.Contains(e.Message, contains) {
			return true
		}
	}
	return false
}
