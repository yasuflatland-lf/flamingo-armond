// Package textdic service entrypoint: a single package-level Process
// function that turns a plain-text dictionary payload into ParsedWord
// records and structured ValidationErrors. Resolver layers are expected
// to handle base64 decoding before calling Process.
package textdic

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
)

// maxPayloadBytes caps the parser at 1 MiB. Beyond this, Process returns
// a payload-level ValidationError (Line == 0) without parsing. A 1 MiB
// payload already represents tens of thousands of dictionary entries,
// well beyond what a single swiping submission needs.
const maxPayloadBytes = 1 << 20 // 1 MiB

// ParsedWord is the public, wire-friendly representation of a successful
// parse. Front holds the headword, Back holds its definition, and Line
// records the 1-indexed source line so the UI can highlight inputs.
type ParsedWord struct {
	Front string
	Back  string
	Line  int
}

// ValidationError is the public, line-scoped error type returned to
// callers. Line == 0 indicates an error not tied to a specific line
// (e.g. payload-size violations).
type ValidationError struct {
	Line    int
	Message string
}

// processMu serialises Process invocations because the goyacc-generated
// parser relies on package-global state (yyParserImpl, currentParser).
var processMu sync.Mutex

// Process parses a plain-text dictionary payload.
//
// Returns:
//   - words: successfully parsed entries.
//   - errs:  per-line structured validation errors.
//   - err:   fatal parser failures (panics, unrecoverable internal state).
//
// The function never panics: any panic from the goyacc-generated parser
// is recovered and reported via err. Empty input is not treated as fatal;
// callers receive a single "empty payload" ValidationError on line 1.
// Base64 decoding is the resolver layer's responsibility; Process only
// understands plain text.
func Process(input string) (words []ParsedWord, errs []ValidationError, err error) {
	// Recover from panics inside the goyacc-generated parser and surface
	// them as err. Programmer errors (runtime.Error: nil deref, index out
	// of range, etc.) are re-panicked so tests and CI surface them rather
	// than silently mapping them to user-input parse failures.
	defer func() {
		if r := recover(); r != nil {
			if rt, ok := r.(runtime.Error); ok {
				panic(rt)
			}
			err = fmt.Errorf("textdic: parser panic: %v\n%s", r, debug.Stack())
		}
	}()

	if len(input) > maxPayloadBytes {
		return []ParsedWord{}, []ValidationError{{Line: 0, Message: fmt.Sprintf("payload exceeds %d bytes", maxPayloadBytes)}}, nil
	}

	if len(input) == 0 {
		return []ParsedWord{}, []ValidationError{{Line: 1, Message: "empty payload"}}, nil
	}

	// The goyacc-generated parser uses package-global state; serialise execution.
	processMu.Lock()
	defer processMu.Unlock()

	l := newLexer(input)
	p := NewParser(l)

	nodes := p.GetNodes()
	rawErrs := p.GetErrors()

	words = make([]ParsedWord, 0, len(nodes))
	for _, n := range nodes {
		if n.Word == "" {
			continue
		}
		words = append(words, ParsedWord{Front: n.Word, Back: n.Definition, Line: n.Line})
	}

	errs = make([]ValidationError, 0, len(rawErrs))
	for _, e := range rawErrs {
		var pe parseError
		if asPe, ok := e.(parseError); ok {
			pe = asPe
		} else {
			pe = parseError{Line: 0, Message: e.Error()}
		}
		errs = append(errs, ValidationError{Line: pe.Line, Message: pe.Message})
	}

	return words, errs, nil
}
