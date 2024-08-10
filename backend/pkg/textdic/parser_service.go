package textdic

import (
	"fmt"
)

// parserService struct definition
type parserService struct{}

// NewParserService creates and returns a new instance of parserService
func NewParserService() *parserService {
	return &parserService{}
}

// ProcessDictionary processes a given dictionary string and returns the parsed Nodes or an error
func (ps *parserService) ProcessDictionary(dic string) ([]Node, error) {

	// Use the new parser to parse the input
	l := newLexer(dic)

	// Parse the input using the new parser
	parsedNodes := ParseAndGetNodes(l)

	if len(parsedNodes) == 0 {
		return nil, fmt.Errorf("no nodes were parsed")
	}

	return parsedNodes, nil
}
