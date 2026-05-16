package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"unicode"
	"unicode/utf8"

	"github.com/rotisserie/eris"
)

// UsecaseCall captures a single r.<Selector>.<Method>(...) call expression
// inside a resolver method body.
type UsecaseCall struct {
	// Selector is the field name on the resolver struct, e.g. "AdminRoleUC".
	Selector string
	// Method is the method invoked on the usecase, e.g. "Update".
	Method string
}

// ResolverMapping maps a single GraphQL mutation field to the usecase
// call(s) made in its resolver method.
type ResolverMapping struct {
	// MutationField is the camelCase GraphQL field name, e.g. "updateRole".
	MutationField string
	// ResolverMethod is the PascalCase method name on *mutationResolver, e.g. "UpdateRole".
	ResolverMethod string
	// UsecaseCalls is every r.<Selector>.<Method>(...) call found in the
	// resolver method body, in source order. len(UsecaseCalls) == 0 when the
	// resolver method makes no usecase call at all.
	UsecaseCalls []UsecaseCall
}

// ResolverWalkResult holds all mappings extracted from the resolver file.
type ResolverWalkResult struct {
	// Mappings contains one ResolverMapping per method on *mutationResolver.
	Mappings []ResolverMapping
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

	var result ResolverWalkResult

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
		mapping := ResolverMapping{
			MutationField:  toCamelCase(methodName),
			ResolverMethod: methodName,
			UsecaseCalls:   collectUsecaseCalls(fn.Body, recvName),
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
// form <recv>.<Selector>.<Method>(...), in source order. Returns nil when the
// body has no such call (so an empty slice is preserved as nil for cmp.Diff).
func collectUsecaseCalls(body *ast.BlockStmt, recvVarName string) []UsecaseCall {
	if body == nil {
		return nil
	}
	var calls []UsecaseCall
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
		calls = append(calls, UsecaseCall{
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
