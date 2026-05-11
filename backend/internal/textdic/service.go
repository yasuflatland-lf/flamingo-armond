// Package textdic service entrypoint: a single Process function that
// turns a plain-text dictionary payload into ParsedWord records and
// structured ValidationErrors. The resolver layer is responsible for
// base64 decoding before calling Process.
package textdic

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/rotisserie/eris"
)

// maxPayloadBytes caps the parser at 1 MiB. Beyond this, Process returns
// a payload-level ValidationError (Line == 0) without parsing. 1 MiB
// already represents tens of thousands of entries.
const maxPayloadBytes = 1 << 20

// ParsedWord is the public, wire-friendly representation of a successful
// parse. Line is the 1-indexed source line so the UI can highlight inputs.
// Only Process constructs values of this type within the package.
type ParsedWord struct {
	Front string
	Back  string
	Line  int
}

// SkipKind classifies a single textdic parser diagnostic.
//
// The zero value (SkipKindUnknown) is reserved as a programming-error sentinel:
// any ValidationError or parseError whose Kind is left unset by construction
// indicates a missed call site. Real construction must always set Kind
// explicitly (Hard, FrontOnly, BackOnly, or Unrecognized).
type SkipKind uint8

const (
	SkipKindUnknown      SkipKind = iota // zero value — programming bug if observed
	SkipKindHard                         // payload-level / lexer-parser hard error
	SkipKindFrontOnly                    // lone WORD: definition absent
	SkipKindBackOnly                     // lone DEFINITION: front absent
	SkipKindUnrecognized                 // unrecognized character
)

// String returns the wire-aligned string representation, matching the
// GraphQL DictionaryValidationKind enum literals.
func (k SkipKind) String() string {
	switch k {
	case SkipKindHard:
		return "HARD"
	case SkipKindFrontOnly:
		return "FRONT_ONLY"
	case SkipKindBackOnly:
		return "BACK_ONLY"
	case SkipKindUnrecognized:
		return "UNRECOGNIZED"
	default:
		return "UNKNOWN"
	}
}

// ValidationError is the public, line-scoped error type. Line == 0
// indicates an error not tied to a specific line (e.g. payload-size).
// Only Process constructs values of this type within the package.
//
// Kind classifies the error: SkipKindHard means a hard lexer/parser failure;
// other values represent soft skips (lone front, lone back, unrecognized
// character). Snippet carries the raw source text that triggered the
// diagnostic — the WORD token value for SkipKindFrontOnly, the DEFINITION
// token value for SkipKindBackOnly, and the recovered malformed line (not a
// single token) for SkipKindUnrecognized. Empty for hard errors and
// payload-level errors. Message is the UI-facing description.
type ValidationError struct {
	Line    int
	Message string
	Kind    SkipKind
	Snippet string
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
			err = eris.Errorf("textdic: parser panic: %v\n%s", r, debug.Stack())
		}
	}()

	if len(input) > maxPayloadBytes {
		return []ParsedWord{}, []ValidationError{{Line: 0, Message: fmt.Sprintf("payload exceeds %d bytes", maxPayloadBytes), Kind: SkipKindHard, Snippet: ""}}, nil
	}
	if len(input) == 0 {
		return []ParsedWord{}, []ValidationError{{Line: 1, Message: "empty payload", Kind: SkipKindHard, Snippet: ""}}, nil
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
			errs = append(errs, ValidationError{Line: pe.Line, Message: pe.Message, Kind: pe.Kind, Snippet: pe.Snippet})
			continue
		}
		errs = append(errs, ValidationError{Line: 0, Message: e.Error(), Kind: SkipKindHard, Snippet: ""})
	}
	return words, errs, nil
}
