package textdic

import (
	"backend/pkg/logger"
	"encoding/base64"
	"fmt"
	"golang.org/x/net/context"
)

// textDictionaryService struct definition
type textDictionaryService struct{}

// NewTextDictionaryService creates and returns a new instance of textDictionaryService
func NewTextDictionaryService() *textDictionaryService {
	return &textDictionaryService{}
}

// Process processes a given dictionary string and returns the parsed Nodes or an error
func (tds *textDictionaryService) Process(ctx context.Context, dic string) ([]Node, error) {

	// Use the new parser to parse the input
	l := newLexer(dic)

	// Parse the input using the new parser
	yyErrorVerbose = true
	parser := NewParser(l)
	parsedNodes := parser.GetNodes()

	if len(parsedNodes) == 0 {
		err := fmt.Errorf("no nodes were parsed")
		logger.Logger.ErrorContext(ctx, err.Error())
		return nil, err
	}

	return parsedNodes, nil
}

// decodeBase64 decodes a Base64 encoded string
func (tds *textDictionaryService) decodeBase64(s string) (string, error) {

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
