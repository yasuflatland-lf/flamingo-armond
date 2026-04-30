// This grammar file is processed by goyacc to generate parser.go.
// Package documentation lives in lexer.go so it does not get duplicated
// into the generated parser.go alongside this header.
%{
package textdic

import (
	"fmt"
	"sync"
)

// node is the internal AST element produced by the parser. It carries the
// source line number so callers can map errors and warnings back to the
// input. The exported wire type is ParsedWord (see service.go); node stays
// internal to keep the goyacc grammar stable.
type node struct {
	Word       string
	Definition string
	Line       int
}

// parseError is the internal structured error type produced by the lexer
// and the goyacc-generated parser. It implements the StructuredError
// interface declared in errors.go.
type parseError struct {
	Line    int
	Message string
}

// Error formats the parseError as "<line>:<message>" so it composes well
// with the standard error interface and existing log infrastructure.
func (e parseError) Error() string {
	return fmt.Sprintf("%d:%s", e.Line, e.Message)
}

%}

%union {
	str   string
	line  int
	node  node
	nodes []node
}

%token<str> WORD DEFINITION NEWLINE
%type<node> entry
%type<nodes> entries
%type<nodes> start

%right DEFINITION
%right WORD

%%
start
	: entries { $$ = $1; yyrcvr.setNodes($1); }
	;

entries
	: entries entry { if $2.Word != "" { $$ = append($1, $2) } else { $$ = $1 } }
	| entry { if $1.Word != "" { $$ = []node{$1} } else { $$ = []node{} } }
	| error NEWLINE { } // Error recovery: discard the offending entry and resume.
	;

entry
	: WORD DEFINITION { $$ = node{Word: $1, Definition: $2, Line: yyDollar[1].line} }
	| NEWLINE { $$ = node{} } // Skip empty lines without producing a node.
	;

%%

// Thread-local storage for the currently executing parser. The
// goyacc-generated parser uses package-level state, so concurrent calls
// must be serialised at a higher level (see service.Process).
var (
	parserExecMutex sync.Mutex
	currentParser   *parserWrapper
)

// wrappedParser is the minimal interface exposed to the service layer. It
// stays unexported because Process is the only public entrypoint; nothing
// outside this package needs to drive the parser directly.
type wrappedParser interface {
	Parse(yyLexer) int
	GetNodes() []node
	GetErrors() []error
}

// parserWrapper bridges the goyacc-generated yyParserImpl with the
// service-level wrappedParser interface and aggregates lexer + parser errors.
type parserWrapper struct {
	lexer  yyLexer
	nodes  []node
	errors []error
	mu     sync.RWMutex
}

// newParser constructs a parser wrapper, runs Parse against the supplied
// lexer, and returns the wrapper so callers can collect nodes and errors.
func newParser(yylex yyLexer) wrappedParser {
	p := &parserWrapper{lexer: yylex}
	p.Parse(yylex)
	return p
}

// Parse drives the goyacc-generated parser. It serialises execution via
// parserExecMutex because yyNewParser shares package-global state.
func (p *parserWrapper) Parse(yylex yyLexer) int {
	parserExecMutex.Lock()
	defer parserExecMutex.Unlock()

	currentParser = p
	defer func() { currentParser = nil }()

	yyErrorVerbose = true
	parser := yyNewParser().(*yyParserImpl)
	result := parser.Parse(yylex)

	// Surface lexer-level errors alongside parser errors.
	if lexerWithErrors, ok := yylex.(*lexer); ok {
		p.mu.Lock()
		p.errors = append(p.errors, lexerWithErrors.GetErrors()...)
		p.mu.Unlock()
	}

	return result
}

// GetNodes returns the list of parsed nodes under read lock.
func (p *parserWrapper) GetNodes() []node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nodes
}

// GetErrors returns the aggregated parser + lexer errors under read lock.
func (p *parserWrapper) GetErrors() []error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.errors
}

// setNodes is invoked from the start production to publish the final node
// list onto the active parser wrapper.
func (yyrcvr *yyParserImpl) setNodes(nodes []node) {
	if currentParser != nil {
		currentParser.mu.Lock()
		defer currentParser.mu.Unlock()
		currentParser.nodes = nodes
	}
}

// GetNodes is exposed on yyParserImpl for parity with the service-level
// wrappedParser interface; primarily useful for diagnostics.
func (yyrcvr *yyParserImpl) GetNodes() []node {
	if currentParser != nil {
		currentParser.mu.RLock()
		defer currentParser.mu.RUnlock()
		return currentParser.nodes
	}
	return nil
}

// Error is the goyacc error callback. The active parserWrapper holds a
// reference to the lexer, so we recover the line at which the offending
// token began (lex.tokenLine); this satisfies the schema contract that
// error.line is the 1-based source line where the error was detected.
// Lexer-level structured errors are still merged separately in Parse.
func (yyrcvr *yyParserImpl) Error(s string) {
	if currentParser == nil {
		return
	}
	currentParser.mu.Lock()
	defer currentParser.mu.Unlock()

	line := 1
	if lex, ok := currentParser.lexer.(*lexer); ok {
		line = lex.tokenLine
		if line < 1 {
			line = lex.lineNo
		}
	}
	currentParser.errors = append(currentParser.errors, parseError{Line: line, Message: s})
}
