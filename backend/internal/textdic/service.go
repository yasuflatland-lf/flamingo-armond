// Package textdic service entrypoint: a single Process function that
// turns a plain-text dictionary payload into ParsedWord records and
// structured ValidationErrors. The resolver layer is responsible for
// base64 decoding before calling Process.
package textdic

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// maxPayloadBytes caps the parser at 1 MiB. Beyond this, Process returns
// a payload-level ValidationError (Line == 0) without parsing. 1 MiB
// already represents tens of thousands of entries.
const maxPayloadBytes = 1 << 20

// ParsedWord is the public, wire-friendly representation of a successful
// parse. Line is the 1-indexed source line so the UI can highlight inputs.
// Constructed only by Process within this package; external zero-value
// construction is allowed by Go but produces a value the parser would never emit.
type ParsedWord struct {
	Front string
	Back  string
	Line  int
}

// ValidationError is the public, line-scoped error type. Line == 0
// indicates an error not tied to a specific line (e.g. payload-size).
// Constructed only by Process within this package; external zero-value
// construction is allowed by Go but produces a value the parser would never emit.
type ValidationError struct {
	Line    int
	Message string
}

// Process parses a plain-text dictionary payload.
//
// Returns:
//   - words: successfully parsed entries.
//   - errs:  per-line structured validation errors.
//   - err:   fatal parser failures (recovered panics).
//
// Empty input is not fatal; callers receive a single "empty payload"
// ValidationError on line 1. Concurrency is handled by parserExecMutex
// inside runParse, so Process takes no additional lock.
func Process(input string) (words []ParsedWord, errs []ValidationError, err error) {
	// Recover panics from the goyacc-generated parser. Programmer errors
	// (nil deref, index out of range) are re-panicked so tests and CI
	// surface them rather than mapping them to user-input parse failures.
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

	nodes, rawErrs := runParse(newLexer(input))

	words = make([]ParsedWord, 0, len(nodes))
	for _, n := range nodes {
		if n.Word == "" {
			continue
		}
		words = append(words, ParsedWord{Front: n.Word, Back: n.Definition, Line: n.Line})
	}

	errs = make([]ValidationError, 0, len(rawErrs))
	for _, e := range rawErrs {
		if pe, ok := e.(parseError); ok {
			errs = append(errs, ValidationError{Line: pe.Line, Message: pe.Message})
			continue
		}
		errs = append(errs, ValidationError{Line: 0, Message: e.Error()})
	}
	return words, errs, nil
}
