// This grammar file is processed by goyacc to generate parser.go.
// Package documentation lives in lexer.go to avoid duplicating a package
// header into the generated parser.go.
%{
package textdic

import (
	"fmt"
	"sync"
)

// node is the internal AST element produced by the parser. The exported
// wire type is ParsedWord (see service.go); node stays internal so the
// goyacc grammar surface can change without breaking callers.
type node struct {
	Word       string
	Definition string
	Line       int
}

// parseError is the structured error produced by the lexer and the
// goyacc-generated parser. Line is the 1-based source line at which the
// error was detected. Kind classifies the error: SkipKindHard means a hard
// lexer/parser failure, while SkipKindFrontOnly, SkipKindBackOnly, and
// SkipKindUnrecognized represent soft skips recorded by grammar productions
// or the lexer. Message is the human-readable description preserved for UI.
// Snippet carries the raw source text that triggered the diagnostic — the
// WORD token value for SkipKindFrontOnly, the DEFINITION token value for
// SkipKindBackOnly, and the recovered malformed line (not a single token)
// for SkipKindUnrecognized. Empty for hard errors.
type parseError struct {
	Line    int
	Message string
	Kind    SkipKind
	Snippet string
}

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

%right WORD
%right DEFINITION

%%
start
	: entries { $$ = $1; yyrcvr.setNodes($1); }
	;

entries
	: entries entry { if $2.Word != "" { $$ = append($1, $2) } else { $$ = $1 } }
	| entry { if $1.Word != "" { $$ = []node{$1} } else { $$ = []node{} } }
	| error NEWLINE { } // Recover from a malformed entry and resume on the next line.
	;

entry
	: WORD DEFINITION { $$ = node{Word: $1, Definition: $2, Line: yyDollar[1].line} }
	| WORD { $$ = node{}; currentParser.recordSkip(yyDollar[1].line, SkipKindFrontOnly, yyDollar[1].str, "skipped: front-only line (no definition)") }
	| DEFINITION { $$ = node{}; currentParser.recordSkip(yyDollar[1].line, SkipKindBackOnly, yyDollar[1].str, "skipped: back-only line (no front)") }
	| NEWLINE { $$ = node{} } // Blank line: skip without producing a node.
	;

%%

// The goyacc-generated parser uses package-level state (yyParserImpl,
// currentParser), so concurrent calls must be serialised. parserExecMutex
// guards every Parse invocation; nothing else needs additional locking.
var (
	parserExecMutex sync.Mutex
	currentParser   *parserWrapper
)

// parserWrapper bridges the goyacc-generated yyParserImpl with the service
// layer and aggregates lexer + parser errors. Concurrent invocation is
// already serialised by parserExecMutex, so no per-instance lock is needed.
type parserWrapper struct {
	lexer  yyLexer
	nodes  []node
	errors []error
}

// runParse runs the goyacc-generated parser against yylex and returns the
// collected nodes plus any lexer/parser errors.
func runParse(yylex yyLexer) ([]node, []error) {
	parserExecMutex.Lock()
	defer parserExecMutex.Unlock()

	p := &parserWrapper{lexer: yylex}
	currentParser = p
	defer func() { currentParser = nil }()

	yyErrorVerbose = true
	yyNewParser().(*yyParserImpl).Parse(yylex)

	if lx, ok := yylex.(*lexer); ok {
		p.errors = append(p.errors, lx.errors...)
	}
	return p.nodes, p.errors
}

// setNodes is invoked from the start production to publish the final node
// list onto the active parser wrapper.
func (yyrcvr *yyParserImpl) setNodes(nodes []node) {
	if currentParser != nil {
		currentParser.nodes = nodes
	}
}

// recordSkip is only invoked via currentParser.recordSkip(...) from the
// grammar's skip productions, so currentParser (and therefore p) is non-nil
// at the call site — runParse holds parserExecMutex and assigns currentParser
// before yyNewParser().Parse runs.
func (p *parserWrapper) recordSkip(line int, kind SkipKind, snippet string, message string) {
	p.errors = append(p.errors, parseError{Line: line, Message: message, Kind: kind, Snippet: snippet})
}

// Error is the goyacc error callback. tokenLine reflects the line at which
// the offending token began, so we attribute the error there rather than
// to lineNo (which has already advanced past any line terminator the
// parser was reacting to).
func (yyrcvr *yyParserImpl) Error(s string) {
	if currentParser == nil {
		return
	}
	line := 1
	if lx, ok := currentParser.lexer.(*lexer); ok {
		line = lx.tokenLine
		if line < 1 {
			line = lx.lineNo
		}
	}
	currentParser.errors = append(currentParser.errors, parseError{Line: line, Message: s, Kind: SkipKindHard, Snippet: ""})
}
