package main

import (
	"os"

	"github.com/rotisserie/eris"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// MutationReturn describes the return type of a single mutation field.
type MutationReturn struct {
	// Field is the camelCase GraphQL mutation field name, e.g. "updateRole".
	Field string
	// ReturnType is the full GraphQL type signature, e.g. "Role!", "[Card!]!".
	ReturnType string
	// IsBare is true when the underlying named type is an Object or Interface.
	// Unions, scalars, enums, and list-wrapped types set IsBare = false.
	IsBare bool
}

// SchemaWalk parses one or more GraphQL SDL files and returns one MutationReturn
// per mutation field found on the Mutation type (including extend blocks).
// If no Mutation type is defined, it returns an error: a schema with no
// Mutation type is treated as degenerate (likely a truncated file or a wrong
// -schema= flag) to prevent the lint from silently passing.
func SchemaWalk(schemaPaths []string) ([]MutationReturn, error) {
	sources := make([]*ast.Source, 0, len(schemaPaths))
	for _, p := range schemaPaths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, eris.Wrapf(err, "schemawalk: read %s", p)
		}
		sources = append(sources, &ast.Source{Name: p, Input: string(raw)})
	}

	schema, err := gqlparser.LoadSchema(sources...)
	if err != nil {
		return nil, eris.Wrap(err, "schemawalk: parse schema")
	}

	mutation := schema.Mutation
	if mutation == nil {
		return nil, eris.New("schemawalk: schema has no Mutation type (likely wrong schema path or truncated file)")
	}

	results := make([]MutationReturn, 0, len(mutation.Fields))
	for _, field := range mutation.Fields {
		mr := MutationReturn{
			Field:      field.Name,
			ReturnType: field.Type.String(),
			IsBare:     classifyBare(schema, field.Type),
		}
		results = append(results, mr)
	}
	return results, nil
}

// classifyBare returns true when the field type, after stripping non-null
// markers and list wrappers, resolves to an Object or Interface named type.
func classifyBare(schema *ast.Schema, t *ast.Type) bool {
	// List-wrapped types are never bare, regardless of element kind.
	if t.Elem != nil {
		return false
	}
	// t.NamedType is the underlying name (e.g. "Role", "Boolean", "CreateCardResult").
	named := t.NamedType
	if named == "" {
		// Defensive: should not happen for a valid schema field type.
		return false
	}
	def, ok := schema.Types[named]
	if !ok {
		// Unknown type — treat as not bare.
		return false
	}
	switch def.Kind {
	case ast.Object:
		return true
	case ast.Interface:
		return true
	default:
		// Union, Scalar, Enum, InputObject — not bare.
		return false
	}
}
