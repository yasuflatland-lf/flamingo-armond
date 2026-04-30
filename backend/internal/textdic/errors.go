// Package textdic error helpers: structured-error contract and a token
// translation table used to render goyacc syntax messages in English.
package textdic

import (
	"regexp"
	"strings"
)

// StructuredError is the contract every textdic error satisfies. It
// extends the standard error interface with line metadata and an
// open-ended details map so callers (resolvers, logs) can present rich
// diagnostics without parsing raw strings.
type StructuredError interface {
	error
	GetLine() int
	GetMessage() string
	GetType() string                    // e.g. "syntax", "semantic", "general".
	GetDetails() map[string]interface{} // Extra structured information.
}

// syntaxErrorRegex extracts the unexpected/expected token names from the
// canonical goyacc syntax-error message.
var syntaxErrorRegex = regexp.MustCompile(`syntax error: unexpected (\S+)(?:, expecting (\S+))?`)

// Compile-time check that parseError satisfies StructuredError.
var _ StructuredError = (*parseError)(nil)

// GetLine returns the source line number where the error was detected.
func (e parseError) GetLine() int {
	return e.Line
}

// GetMessage returns the raw, untranslated error text.
func (e parseError) GetMessage() string {
	return e.Message
}

// GetType classifies the error so UI layers can group similar diagnostics.
func (e parseError) GetType() string {
	if strings.Contains(e.Message, "syntax error") {
		return "syntax"
	}
	if strings.Contains(e.Message, "no nodes were parsed") {
		return "empty"
	}
	return "general"
}

// GetDetails extracts structured fields from the message body. For syntax
// errors it pulls out the offending token and (when present) the expected
// token, so callers can highlight them without re-parsing the message.
func (e parseError) GetDetails() map[string]interface{} {
	details := make(map[string]interface{})

	if matches := syntaxErrorRegex.FindStringSubmatch(e.Message); matches != nil {
		details["unexpected"] = matches[1]
		if len(matches) > 2 && matches[2] != "" {
			details["expected"] = matches[2]
		}
	}

	return details
}

// tokenTranslations maps internal goyacc token names to human-readable
// English labels. The repository policy mandates English-only committed
// text, so all user-visible token names live here.
var tokenTranslations = map[string]string{
	"$end":       "end of input",
	"WORD":       "word (front)",
	"DEFINITION": "definition (back)",
	"NEWLINE":    "newline",
	"$unk":       "unknown token",
	"error":      "error",
}

// TranslateTokenName returns the English label for an internal token name,
// falling back to the raw name when no translation is registered.
func TranslateTokenName(token string) string {
	if trans, ok := tokenTranslations[token]; ok {
		return trans
	}
	return token
}
