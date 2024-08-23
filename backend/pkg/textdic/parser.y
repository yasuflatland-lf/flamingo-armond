%{
package textdic

import "sync"

// Define Node and Nodes types
type Node struct {
	Word       string
	Definition string
}

type Nodes []Node

%}

%union {
    mutex sync.RWMutex
	str  string
	node Node
	nodes Nodes
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
	| entry { if $1.Word != "" { $$ = []Node{$1} } else { $$ = Nodes{} } }
	;

entry
	: WORD DEFINITION { $$ = Node{Word: $1, Definition: $2} }
	| NEWLINE { $$ = Node{} } // Ignore empty line
	;

%%

type Parser interface {
	Parse(yyLexer) int
	GetNodes() []Node
}

func NewParser(yylex yyLexer) Parser {
	yyparser := &yyParserImpl{}
    yyparser.lval.mutex.RLock()
    defer yyparser.lval.mutex.RUnlock()
	yyparser.Parse(yylex)
	return yyparser
}

func (yyrcvr *yyParserImpl) setNodes(nodes []Node) {
    yyrcvr.lval.mutex.RLock()
    defer yyrcvr.lval.mutex.RUnlock()
	yyrcvr.lval.nodes = nodes
}

func (yyrcvr *yyParserImpl) GetNodes() []Node {
    yyrcvr.lval.mutex.RLock()
    defer yyrcvr.lval.mutex.RUnlock()
	return yyrcvr.lval.nodes
}
