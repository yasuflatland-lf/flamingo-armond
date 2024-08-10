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

// Modify yyParserImpl to hold a reference to the current Parser
type yyParserImpl struct {
	lval  yySymType
	stack [yyInitialStackSize]yySymType
	char  int
	parser *Parser  // Add parser field
}

// Modify yyNewParser to accept a Parser instance
func yyNewParser(parser *Parser) yyParser {
	return &yyParserImpl{parser: parser}
}

func yyParse(yylex yyLexer, parser *Parser) int {
	return yyNewParser(parser).Parse(yylex)
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
	: entries EOF { $$ = $1; yyrcvr.parser.setNodes($1); }
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

// Parser struct to encapsulate parsedNodes
type Parser struct {
	parsedNodes Nodes
}

// NewParser initializes and returns a new Parser instance
func NewParser() *Parser {
	return &Parser{}
}

// getNodes returns the parsed nodes
func (p *Parser) getNodes() Nodes {
	return p.parsedNodes
}

// setNodes sets the parsed nodes
func (p *Parser) setNodes(nodes Nodes) {
	p.parsedNodes = nodes
}
