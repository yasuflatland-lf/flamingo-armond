package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/rotisserie/eris"
)

// UsecaseMethod describes a single method on a usecase type and whether it
// emits any typed ucerr value in its body.
type UsecaseMethod struct {
	// File is the base filename, e.g. "admin_role.go".
	File string
	// ReceiverType is the source-form receiver type, e.g. "*adminRoleUsecase"
	// or "adminRoleUsecase".
	ReceiverType string
	// Method is the declared method name, e.g. "Update".
	Method string
	// EmitsTypedError is true when the function body contains at least one of:
	//   - ucerr.NewValidationError(...)
	//   - ucerr.NewForbiddenError(...)
	//   - ucerr.ErrUnauthenticated (identifier reference)
	EmitsTypedError bool
}

// UsecaseWalk parses every non-test *.go file in usecaseDir (non-recursive:
// sub-packages such as ucerr/ are intentionally skipped) and returns one
// UsecaseMethod per declared method.
func UsecaseWalk(usecaseDir string) ([]UsecaseMethod, error) {
	fset := token.NewFileSet()
	// Parse only the immediate directory; do not recurse into sub-packages.
	pkgs, err := parser.ParseDir(fset, usecaseDir, func(info fs.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, 0)
	if err != nil {
		return nil, eris.Wrapf(err, "usecasewalk: parse dir %s", usecaseDir)
	}

	var results []UsecaseMethod
	for _, pkg := range pkgs {
		for filePath, f := range pkg.Files {
			baseName := filepath.Base(filePath)
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
					continue
				}
				recvType := formatReceiverType(fn.Recv.List[0])
				um := UsecaseMethod{
					File:            baseName,
					ReceiverType:    recvType,
					Method:          fn.Name.Name,
					EmitsTypedError: methodEmitsTypedError(fn.Body),
				}
				results = append(results, um)
			}
		}
	}
	return results, nil
}

// formatReceiverType formats the receiver type as a source-form string.
// *ast.StarExpr over *ast.Ident → "*Name"; bare *ast.Ident → "Name".
func formatReceiverType(field *ast.Field) string {
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// methodEmitsTypedError returns true if the function body contains any of:
//   - a call to ucerr.NewValidationError or ucerr.NewForbiddenError
//   - a reference to ucerr.ErrUnauthenticated (SelectorExpr, not necessarily a call)
func methodEmitsTypedError(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	emits := false
	ast.Inspect(body, func(n ast.Node) bool {
		if emits {
			return false // short-circuit once found
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		xIdent, ok := sel.X.(*ast.Ident)
		if !ok || xIdent.Name != "ucerr" {
			return true
		}
		switch sel.Sel.Name {
		case "NewValidationError", "NewForbiddenError", "ErrUnauthenticated":
			emits = true
		}
		return true
	})
	return emits
}
