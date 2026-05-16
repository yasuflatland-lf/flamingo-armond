package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"unicode"
	"unicode/utf8"

	"github.com/rotisserie/eris"
)

// ResolverMapping maps a single GraphQL mutation field to the usecase call
// made in its resolver method.
type ResolverMapping struct {
	// MutationField is the camelCase GraphQL field name, e.g. "updateRole".
	MutationField string
	// ResolverMethod is the PascalCase method name on *mutationResolver, e.g. "UpdateRole".
	ResolverMethod string
	// UsecaseSelector is the field name on the resolver struct, e.g. "AdminRoleUC".
	// Empty when no usecase call was found in the method body.
	UsecaseSelector string
	// UsecaseMethod is the method invoked on the usecase, e.g. "Update".
	// Empty when no usecase call was found in the method body.
	UsecaseMethod string
}

// usecaseCall captures a single r.<Selector>.<Method> call expression.
type usecaseCall struct {
	Selector string
	Method   string
}

// ResolverWalkResult holds all mappings extracted from the resolver file.
type ResolverWalkResult struct {
	// Mappings contains one ResolverMapping per method on *mutationResolver.
	// When a method has multiple usecase calls, the first call is reflected
	// here; the rest appear in AdditionalCalls.
	Mappings []ResolverMapping

	// AdditionalCalls maps a ResolverMethod name to any usecase calls beyond
	// the first. Only populated for methods with two or more calls.
	AdditionalCalls map[string][]struct{ Selector, Method string }
}

// ResolverWalk parses the Go source file at resolverPath and extracts every
// method on *mutationResolver (or mutationResolver) together with the
// usecase call(s) found in its body.
func ResolverWalk(resolverPath string) (ResolverWalkResult, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, resolverPath, nil, 0)
	if err != nil {
		return ResolverWalkResult{}, eris.Wrapf(err, "resolverwalk: parse %s", resolverPath)
	}

	result := ResolverWalkResult{
		AdditionalCalls: make(map[string][]struct{ Selector, Method string }),
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}

		recvName, recvType := extractReceiver(fn.Recv.List[0])
		if recvType != "mutationResolver" {
			continue
		}

		methodName := fn.Name.Name
		calls := collectUsecaseCalls(fn.Body, recvName)

		mapping := ResolverMapping{
			MutationField:  toCamelCase(methodName),
			ResolverMethod: methodName,
		}
		if len(calls) >= 1 {
			mapping.UsecaseSelector = calls[0].Selector
			mapping.UsecaseMethod = calls[0].Method
		}
		if len(calls) > 1 {
			extra := make([]struct{ Selector, Method string }, 0, len(calls)-1)
			for _, c := range calls[1:] {
				extra = append(extra, struct{ Selector, Method string }{c.Selector, c.Method})
			}
			result.AdditionalCalls[methodName] = extra
		}

		result.Mappings = append(result.Mappings, mapping)
	}

	return result, nil
}

// extractReceiver returns the receiver variable name and the receiver type
// name (with any pointer star stripped) from a receiver field.
func extractReceiver(field *ast.Field) (varName, typeName string) {
	if len(field.Names) > 0 {
		varName = field.Names[0].Name
	}
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			typeName = ident.Name
		}
	case *ast.Ident:
		typeName = t.Name
	}
	return
}

// collectUsecaseCalls walks the function body and returns every call of the
// form <recv>.<Selector>.<Method>(...), in source order.
func collectUsecaseCalls(body *ast.BlockStmt, recvVarName string) []usecaseCall {
	if body == nil {
		return nil
	}
	var calls []usecaseCall
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// We are looking for <recv>.<Selector>.<Method>(...)
		// which parses as:
		//   CallExpr.Fun = SelectorExpr{
		//     X:   SelectorExpr{ X: Ident(recv), Sel: Ident(Selector) },
		//     Sel: Ident(Method),
		//   }
		outer, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		inner, ok := outer.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recvIdent, ok := inner.X.(*ast.Ident)
		if !ok || recvIdent.Name != recvVarName {
			return true
		}
		calls = append(calls, usecaseCall{
			Selector: inner.Sel.Name,
			Method:   outer.Sel.Name,
		})
		return true
	})
	return calls
}

// toCamelCase lowercases the first Unicode rune of s, leaving the rest intact.
// "UpdateRole" → "updateRole", "HandleSwipe" → "handleSwipe".
func toCamelCase(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToLower(r)) + s[size:]
}
