%{
package textdic

import (
	"fmt"
)

// Define Node and Nodes types
type Node struct {
	Word       string
	Definition string
}

type Nodes []Node

func ParseAndGetNodes(yylex yyLexer) []Node {
	yyparser := &yyParserImpl{}
	yyparser.Parse(yylex)
	return yyparser.getNodes()
}

%}

%union {
	str  string
	node Node
	nodes Nodes
}

%token<str> WORD DEFINITION NEWLINE EOF
%type<node> entry
%type<nodes> entries
%type<nodes> start

%right DEFINITION
%right WORD

%%
start
	: entries EOF { $$ = $1; yyrcvr.setNodes($1); }
	;

entries
	: entries entry { if $2.Word != "" { $$ = append($1, $2) } else { $$ = $1 } }
	| entry { if $1.Word != "" { $$ = []Node{$1} } else { $$ = Nodes{} } }
	;

entry
	: WORD DEFINITION NEWLINE { $$ = Node{Word: $1, Definition: $2} }
	| NEWLINE { $$ = Node{} } // 空行を無視する
	;

%%

func yyError(s string) {
	fmt.Println("Error:", s)
}

func (yyrcvr *yyParserImpl) setNodes(nodes []Node) {
	yyrcvr.lval.nodes = nodes
}

func (yyrcvr *yyParserImpl) getNodes() []Node {
	return yyrcvr.lval.nodes
}