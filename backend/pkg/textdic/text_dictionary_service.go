package textdic

import (
	"encoding/base64"
	"fmt"
	"sync"
)

// textDictionaryService struct definition
type textDictionaryService struct {
	mu sync.RWMutex
}

// TextDictionaryService defines the methods for processing text dictionaries.
type TextDictionaryService interface {
	Process(dic string) ([]Node, []error)
	DecodeBase64(s string) (string, error)
}

// NewTextDictionaryService creates and returns a new instance of textDictionaryService
func NewTextDictionaryService() TextDictionaryService {
	return &textDictionaryService{}
}

// Process processes a given dictionary string and returns the parsed Nodes or an error
func (tds *textDictionaryService) Process(dic string) ([]Node, []error) {
	tds.mu.RLock()
	defer tds.mu.RUnlock()

	// Use the new parser to parse the input
	l := newLexer(dic)

	// Parse the input using the new parser
	//yyErrorVerbose = true

	parser := NewParser(l)
	parsedNodes := parser.GetNodes()

	if len(l.GetErrors()) != 0 {
		return nil, l.GetErrors()
	}
	if len(parsedNodes) == 0 {
		err := fmt.Errorf("no nodes were parsed")
		return nil, []error{err}
	}

	return parsedNodes, nil
}

// DecodeBase64 decodes a Base64 encoded string
func (tds *textDictionaryService) DecodeBase64(s string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
