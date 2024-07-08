package notion

import (
	"testing"
)

// Test input
var input = `
trot out 自慢げに話題に持ち出す

jarring 気に障る

rube 田舎者

out of touch 情報に疎い、

opaque 不透明な

trot up 自慢げに歩かせて見せる、出して見せる、披露(ひろう)する、持ち出す、口にする

wriggle out of ～からうまく［何とか］切り抜ける

get under someone's skin 「（人）の気［癇］に障る、（人）をひどく怒らせる、（人）をイライラ

leeway 〔自分の好きなように行動・思考できる〕自由（裁量）度◆不可〔時間・金などの〕余裕、ゆとり
There is no leeway to provide services free of charge for the sake of others. 他人のために無償でサービスをする余裕はない。

`

// Define the test cases
var testCases = []struct {
	input    string
	expected []Node
}{
	{
		input: input,
		expected: []Node{
			{Word: "trot out", Definition: "自慢げに話題に持ち出す"},
			{Word: "jarring", Definition: "気に障る"},
			{Word: "rube", Definition: "田舎者"},
			{Word: "out of touch", Definition: "情報に疎い、"},
			{Word: "opaque", Definition: "不透明な"},
			{Word: "trot up", Definition: "自慢げに歩かせて見せる、出して見せる、披露(ひろう)する、持ち出す、口にする"},
			{Word: "wriggle out of", Definition: "～からうまく［何とか］切り抜ける"},
			{Word: "get under someone's skin", Definition: "「（人）の気［癇］に障る、（人）をひどく怒らせる、（人）をイライラ"},
			{Word: "leeway", Definition: "〔自分の好きなように行動・思考できる〕自由（裁量）度◆不可〔時間・金などの〕余裕、ゆとり"},
			{Word: "There is no leeway to provide services free of charge for the sake of others.", Definition: "他人のために無償でサービスをする余裕はない。"},
		},
	},
}

func TestParser(t *testing.T) {
	for _, tc := range testCases {
		// Create a new parser
		parser := NewParser()

		// Reset and set the current parser
		currentParser = parser

		// Create a new lexer with the input
		l := newLexer(tc.input)

		// Parse the input
		yyParse(l)

		// Compare the result with the expected output
		parsedNodes := parser.getNodes()
		if len(parsedNodes) != len(tc.expected) {
			t.Errorf("expected %d nodes, but got %d", len(tc.expected), len(parsedNodes))
		}

		for i, node := range parsedNodes {
			if node.Word != tc.expected[i].Word || node.Definition != tc.expected[i].Definition {
				t.Errorf("expected node %d to be %+v, but got %+v", i, tc.expected[i], node)
			}
		}
	}
}
