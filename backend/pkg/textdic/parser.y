%{
package textdic

// Define Node and Nodes types
type Node struct {
	Word       string
	Definition string
}

type Nodes []Node

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

type Parser interface {
	Parse(yyLexer) int
	GetNodes() []Node
}

func NewParser(yylex yyLexer) Parser {
	yyparser := &yyParserImpl{}
	yyparser.Parse(yylex)
	return yyparser
}

func (yyrcvr *yyParserImpl) setNodes(nodes []Node) {
	yyrcvr.lval.nodes = nodes
}

func (yyrcvr *yyParserImpl) GetNodes() []Node {
	return yyrcvr.lval.nodes
}
