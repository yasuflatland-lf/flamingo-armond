package textdic

import (
	"sync"
	"testing"
)

func TestParserService(t *testing.T) {
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
	var inputEOF = `leeway 〔自分の好きなように行動・思考できる〕自由（裁量）度◆不可〔時間・金などの〕余裕、ゆとり
There is no leeway to provide services free of charge for the sake of others. 他人のために無償でサービスをする余裕はない。`

	var inputMix = `get on with ～に急がせる、Get on with it. : 急げ。／さっさとやれ。
Hold me accountable for 自分の行動の結果を受け入れ、罰を受け、または自分が引き起こした損害を修復することを意味します。`

	// Define the test cases
	var testCases = []struct {
		name     string
		input    string
		expected []Node
	}{
		{
			name:  "Valid input",
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
		{
			name:  "Valid input EOF",
			input: inputEOF,
			expected: []Node{
				{Word: "leeway", Definition: "〔自分の好きなように行動・思考できる〕自由（裁量）度◆不可〔時間・金などの〕余裕、ゆとり"},
				{Word: "There is no leeway to provide services free of charge for the sake of others.", Definition: "他人のために無償でサービスをする余裕はない。"},
			},
		},
		{
			name:  "Valid input MIX",
			input: inputMix,
			expected: []Node{
				{Word: "get on with", Definition: "～に急がせる、Get on with it. : 急げ。／さっさとやれ。"},
				{Word: "Hold me accountable for", Definition: "自分の行動の結果を受け入れ、罰を受け、または自分が引き起こした損害を修復することを意味します。"},
			},
		},
	}

	// Run TestParserService
	t.Run("TestParserService", func(t *testing.T) {
		for _, tc := range testCases {
			tc := tc // capture range variable to avoid issues in parallel tests
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel() // Mark the test to run in parallel

				// Create a new parser service
				service := NewTextDictionaryService()

				yyDebug = 5
				yyErrorVerbose = true
				// Process the dictionary input
				parsedNodes, err := service.Process(tc.input)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				// Compare the result with the expected output
				if len(parsedNodes) != len(tc.expected) {
					t.Errorf("expected %d nodes, but got %d", len(tc.expected), len(parsedNodes))
				}

				for i, node := range parsedNodes {
					if node.Word != tc.expected[i].Word || node.Definition != tc.expected[i].Definition {
						t.Errorf("expected node %d to be %+v, but got %+v", i, tc.expected[i], node)
					}
				}
			})
		}
	})

	// Run TestParserService_ErrorCases
	t.Run("TestParserService_ErrorCases", func(t *testing.T) {
		// Define error test cases
		errorTestCases := []struct {
			name        string
			input       string
			wantErr     bool
			expectNodes bool // Expect valid nodes even when there is an error
		}{
			{
				name:        "Empty input",
				input:       "",
				wantErr:     true,
				expectNodes: false,
			},
			{
				name:        "Malformed input",
				input:       "trot out 自慢げに話題に持ち出す\n jarring 気に障る\n不正なデータ",
				wantErr:     true, // Expect an error
				expectNodes: false,
			},
		}

		for _, tc := range errorTestCases {
			tc := tc // capture range variable to avoid issues in parallel tests
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel() // Mark the test to run in parallel

				// Create a new parser service
				service := NewTextDictionaryService()

				// Process the dictionary input
				parsedNodes, err := service.Process(tc.input)

				if tc.wantErr {
					if err == nil {
						t.Errorf("expected an error but got none")
					}
				}

				if tc.expectNodes && len(parsedNodes) == 0 {
					t.Errorf("expected parsed nodes but got none")
				}
			})
		}
	})

	// Run concurrent tests
	t.Run("TestParserService_ConcurrentAccess", func(t *testing.T) {
		var wg sync.WaitGroup
		numRoutines := 10 // Number of concurrent goroutines

		for _, tc := range testCases {
			wg.Add(numRoutines)
			for i := 0; i < numRoutines; i++ {
				go func(tc struct {
					name     string
					input    string
					expected []Node
				}) {
					defer wg.Done()

					// Create a new parser service
					service := NewTextDictionaryService()

					// Process the dictionary input
					parsedNodes, err := service.Process(tc.input)
					if err != nil {
						t.Errorf("unexpected error: %v", err)
						return
					}

					// Compare the result with the expected output
					if len(parsedNodes) != len(tc.expected) {
						t.Errorf("expected %d nodes, but got %d", len(tc.expected), len(parsedNodes))
					}

					for i, node := range parsedNodes {
						if node.Word != tc.expected[i].Word || node.Definition != tc.expected[i].Definition {
							t.Errorf("expected node %d to be %+v, but got %+v", i, tc.expected[i], node)
						}
					}
				}(tc)
			}
			wg.Wait() // Wait for all goroutines to finish
		}
	})

	t.Run("Test_decodeBase64", func(t *testing.T) {
		service := NewTextDictionaryService()

		// Define test cases
		testCases := []struct {
			name     string
			input    string
			wantErr  bool
			expected string
		}{
			{"Valid Base64", "SGVsbG8gd29ybGQ=", false, "Hello world"},
			{"Empty String", "", false, ""},
			{"Invalid Base64", "Hello, World!", true, ""},
			{"Invalid Base64 Hello World", "SGVsbG8aaagd29ybGQ", true, ""},
			{"Base64 with Newlines", "SGVsbG8gd29ybGQ=\n", false, "Hello world"},
			{"Base64 with Multiple Newlines", "SGVsbG8g\nd29y\nbGQ=", false, "Hello world"},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel() // Mark the test to run in parallel

				result, err := service.DecodeBase64(tc.input)
				if tc.wantErr {
					if err == nil {
						t.Errorf("expected error but got none")
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %v", err)
					}
					if result != tc.expected {
						t.Errorf("expected %s, but got %s", tc.expected, result)
					}
				}
			})
		}
	})
}
