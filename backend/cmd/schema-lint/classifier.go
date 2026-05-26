package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"

	"github.com/rotisserie/eris"
)

// Violation represents one bare-emit mutation that is not allowlisted.
type Violation struct {
	// Mutation is the GraphQL field name, e.g. "updateRole".
	Mutation string
	// ReturnType is the GraphQL return type string, e.g. "Role!".
	ReturnType string
	// ResolverMethod is the PascalCase resolver method name, e.g. "UpdateRole".
	ResolverMethod string
	// UsecaseTarget is the resolved usecase receiver+method, e.g. "AdminRoleUC.Update".
	UsecaseTarget string
}

// Drift represents a schema/resolver-Go drift: a mutation declared in
// schema.graphql with no matching resolver method, or a resolver method
// with no matching mutation field.
type Drift struct {
	// Mutation is the camelCase GraphQL mutation name. Empty if resolver-side drift.
	Mutation string
	// ResolverMethod is the PascalCase resolver method name. Empty if schema-side drift.
	ResolverMethod string
	// Description is a human-readable explanation of the drift.
	Description string
}

// Allowlist is a set of mutation names that are exempt from the lint.
// Loaded from backend/cmd/schema-lint/allowlist.txt.
type Allowlist map[string]struct{}

// ClassifierConfig carries optional interface-to-implementation mappings for
// usecase types that are declared as interfaces in the resolver struct.
type ClassifierConfig struct {
	// InterfaceToImpl maps an interface type name to its unexported implementation
	// type name, e.g. "AdminRoleUsecase" -> "adminRoleUsecase".
	InterfaceToImpl map[string]string
}

// LoadAllowlist reads allowlist.txt and returns the set of allowlisted
// mutation names. Blank lines and lines starting with "#" are ignored.
// Inline comments after a "#" on a value line are also stripped.
func LoadAllowlist(path string) (Allowlist, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, eris.Wrapf(err, "classifier: open allowlist %s", path)
	}
	defer f.Close()

	al := make(Allowlist)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Strip inline comments.
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		al[line] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, eris.Wrapf(err, "classifier: scan allowlist %s", path)
	}
	return al, nil
}

// LoadResolverFieldMap parses the resolver struct file and returns a map from
// resolver struct field name (e.g. "AdminRoleUC") to the underlying concrete
// usecase type name (e.g. "adminRoleUsecase"). The value strips any leading "*"
// and any "usecase." package prefix so callers can match directly against
// UsecaseMethod.ReceiverType.
func LoadResolverFieldMap(resolverPath string) (map[string]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, resolverPath, nil, 0)
	if err != nil {
		return nil, eris.Wrapf(err, "classifier: parse resolver struct %s", resolverPath)
	}

	result := make(map[string]string)
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typeSpec.Name.Name != "Resolver" {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}
				fieldName := field.Names[0].Name
				typeName := extractTypeName(field.Type)
				if typeName != "" {
					result[fieldName] = typeName
				}
			}
		}
	}
	return result, nil
}

// extractTypeName extracts the base type name from a field type expression,
// stripping any leading "*" and any "usecase." package prefix.
func extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		// Pointer type: *usecase.UserUsecase or *UserUsecase
		return extractTypeName(t.X)
	case *ast.SelectorExpr:
		// Qualified name: usecase.CardImportUsecase
		// Return only the selector (type name), strip package prefix.
		return t.Sel.Name
	case *ast.Ident:
		// Unqualified name: someLocalType
		return t.Name
	}
	return ""
}

// Classify joins schema, resolver, and usecase walker outputs and applies
// the allowlist. Returns violations and drifts in deterministic (sorted) order.
//
// resolverFieldToUsecaseType maps resolver struct field names to usecase type
// names (as returned by LoadResolverFieldMap), e.g. {"AdminRoleUC": "AdminRoleUsecase"}.
//
// The cfg parameter carries optional InterfaceToImpl mappings for resolving
// interface type names to concrete implementation names. A zero ClassifierConfig
// is safe (all fields are nil-tolerant).
func Classify(
	schema []MutationReturn,
	resolver ResolverWalkResult,
	usecase []UsecaseMethod,
	resolverFieldToUsecaseType map[string]string,
	allowlist Allowlist,
	cfg ClassifierConfig,
) (violations []Violation, drifts []Drift) {
	// Build a lookup: camelCase mutation name -> ResolverMapping.
	resolverByMutation := make(map[string]ResolverMapping, len(resolver.Mappings))
	for _, m := range resolver.Mappings {
		resolverByMutation[m.MutationField] = m
	}

	// Build a lookup: camelCase mutation name from schema -> MutationReturn.
	schemaByMutation := make(map[string]MutationReturn, len(schema))
	for _, mr := range schema {
		schemaByMutation[mr.Field] = mr
	}

	// Build a lookup: (receiverType, method) -> UsecaseMethod.
	// Key is "ReceiverType:Method".
	usecaseByKey := make(map[string]UsecaseMethod, len(usecase))
	for _, um := range usecase {
		key := um.ReceiverType + ":" + um.Method
		usecaseByKey[key] = um
		// Also index without leading "*" for pointer receivers.
		stripped := strings.TrimPrefix(um.ReceiverType, "*")
		if stripped != um.ReceiverType {
			usecaseByKey[stripped+":"+um.Method] = um
		}
	}

	// Drift detection — schema-side: mutations with no resolver method.
	for _, mr := range schema {
		if _, found := resolverByMutation[mr.Field]; !found {
			drifts = append(drifts, Drift{
				Mutation:    mr.Field,
				Description: "mutation '" + mr.Field + "' has no resolver method",
			})
		}
	}

	// Drift detection — resolver-side: resolver methods with no mutation field.
	for _, rm := range resolver.Mappings {
		if _, found := schemaByMutation[rm.MutationField]; !found {
			drifts = append(drifts, Drift{
				ResolverMethod: rm.ResolverMethod,
				Description:    "resolver method '" + rm.ResolverMethod + "' has no mutation field in schema",
			})
		}
	}

	// Drift detection — resolver method has a usecase selector whose
	// resolver-struct field type is not resolvable (neither directly in the
	// field-map nor via InterfaceToImpl). A new interface-typed Resolver field
	// without a corresponding interfaceToImpl entry would silently disable
	// emission detection otherwise.
	for _, rm := range resolver.Mappings {
		for _, c := range rm.UsecaseCalls {
			if isSelectorResolvable(c.Selector, resolverFieldToUsecaseType, cfg.InterfaceToImpl) {
				continue
			}
			drifts = append(drifts, Drift{
				ResolverMethod: rm.ResolverMethod,
				Description: fmt.Sprintf(
					"classifier: resolver method '%s' has usecase selector 'r.%s.%s' but no corresponding usecase type found (add to interfaceToImpl map?)",
					rm.ResolverMethod, c.Selector, c.Method,
				),
			})
		}
	}

	// Violation detection — for each schema mutation, check if it is bare and emits.
	for _, mr := range schema {
		if !mr.IsBare {
			continue
		}
		rm, found := resolverByMutation[mr.Field]
		if !found {
			// Already reported as drift above.
			continue
		}

		// OR-aggregate emission: if any reachable usecase method emits, mark true.
		emits := false
		usecaseTarget := ""
		for _, c := range rm.UsecaseCalls {
			// Resolve the resolver field type.
			fieldType, ok := resolverFieldToUsecaseType[c.Selector]
			if !ok {
				continue
			}
			// Resolve interface -> impl if applicable.
			implType := resolveImplType(fieldType, cfg.InterfaceToImpl)

			// Try matching usecase methods with both pointer and value receiver.
			for _, candidate := range []string{"*" + implType, implType} {
				key := candidate + ":" + c.Method
				if um, found := usecaseByKey[key]; found {
					if um.EmitsTypedError {
						emits = true
					}
					if usecaseTarget == "" {
						usecaseTarget = c.Selector + "." + c.Method
					}
					break
				}
			}
		}

		if !emits {
			continue
		}

		// Check the allowlist.
		if _, allowlisted := allowlist[mr.Field]; allowlisted {
			continue
		}

		if usecaseTarget == "" && len(rm.UsecaseCalls) > 0 {
			usecaseTarget = rm.UsecaseCalls[0].Selector + "." + rm.UsecaseCalls[0].Method
		}

		violations = append(violations, Violation{
			Mutation:       mr.Field,
			ReturnType:     mr.ReturnType,
			ResolverMethod: rm.ResolverMethod,
			UsecaseTarget:  usecaseTarget,
		})
	}

	// Sort violations by mutation name for deterministic output.
	sort.Slice(violations, func(i, j int) bool {
		return violations[i].Mutation < violations[j].Mutation
	})

	// Sort drifts by a stable key (mutation name, then resolver method).
	sort.Slice(drifts, func(i, j int) bool {
		ki := drifts[i].Mutation + drifts[i].ResolverMethod
		kj := drifts[j].Mutation + drifts[j].ResolverMethod
		return ki < kj
	})

	return violations, drifts
}

// resolveImplType looks up an interface type name in the InterfaceToImpl map
// and returns the concrete implementation name. If no mapping exists, the
// input name is returned unchanged.
func resolveImplType(typeName string, interfaceToImpl map[string]string) string {
	if interfaceToImpl == nil {
		return typeName
	}
	if impl, ok := interfaceToImpl[typeName]; ok {
		return impl
	}
	return typeName
}

// isSelectorResolvable returns true when the resolver-struct field selector
// is present in resolverFieldToUsecaseType. The selector resolves to a
// type name; whether that type name eventually matches a concrete usecase
// receiver is a separate concern handled by emission detection.
func isSelectorResolvable(selector string, resolverFieldToUsecaseType, interfaceToImpl map[string]string) bool {
	_, ok := resolverFieldToUsecaseType[selector]
	if ok {
		return true
	}
	// As a defensive fallback, if the selector is itself a known interface name
	// in InterfaceToImpl, treat it as resolvable. In practice the selector is
	// a field name (e.g. "AdminRoleUC"), not a type name, so this branch is
	// rarely exercised — but it keeps the lint quiet when an operator adds an
	// interfaceToImpl entry that happens to share a name with a field.
	if interfaceToImpl != nil {
		if _, ok := interfaceToImpl[selector]; ok {
			return true
		}
	}
	return false
}

// AllowlistRotEntries returns the subset of allowlist entries that no longer
// correspond to a bare-emit mutation. These are "rotten" entries that should
// be removed.
//
// An entry rots when the mutation is no longer bare (returned a union, scalar,
// list, or the mutation was removed from the schema), or when the usecase
// no longer emits typed errors on the path the resolver calls.
func AllowlistRotEntries(
	schema []MutationReturn,
	resolver ResolverWalkResult,
	usecase []UsecaseMethod,
	resolverFieldToUsecaseType map[string]string,
	allowlist Allowlist,
	cfg ClassifierConfig,
) []string {
	// Build schema lookup.
	schemaByMutation := make(map[string]MutationReturn, len(schema))
	for _, mr := range schema {
		schemaByMutation[mr.Field] = mr
	}

	// Build resolver lookup.
	resolverByMutation := make(map[string]ResolverMapping, len(resolver.Mappings))
	for _, m := range resolver.Mappings {
		resolverByMutation[m.MutationField] = m
	}

	// Build usecase lookup.
	usecaseByKey := make(map[string]UsecaseMethod, len(usecase))
	for _, um := range usecase {
		key := um.ReceiverType + ":" + um.Method
		usecaseByKey[key] = um
		stripped := strings.TrimPrefix(um.ReceiverType, "*")
		if stripped != um.ReceiverType {
			usecaseByKey[stripped+":"+um.Method] = um
		}
	}

	var rotten []string
	for entry := range allowlist {
		mr, inSchema := schemaByMutation[entry]
		if !inSchema {
			// Mutation was removed entirely.
			rotten = append(rotten, entry)
			continue
		}
		if !mr.IsBare {
			// Mutation now returns a union/scalar/list.
			rotten = append(rotten, entry)
			continue
		}
		rm, inResolver := resolverByMutation[entry]
		if !inResolver {
			// Resolver method removed.
			rotten = append(rotten, entry)
			continue
		}

		// Check emission status across every usecase call in the resolver method.
		emits := false
		for _, c := range rm.UsecaseCalls {
			fieldType, ok := resolverFieldToUsecaseType[c.Selector]
			if !ok {
				continue
			}
			implType := resolveImplType(fieldType, cfg.InterfaceToImpl)
			for _, candidate := range []string{"*" + implType, implType} {
				key := candidate + ":" + c.Method
				if um, found := usecaseByKey[key]; found {
					if um.EmitsTypedError {
						emits = true
					}
					break
				}
			}
		}
		if !emits {
			rotten = append(rotten, entry)
		}
	}

	sort.Strings(rotten)
	return rotten
}
