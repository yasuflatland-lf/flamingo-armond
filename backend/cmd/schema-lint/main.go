// Package main is the schema-lint CLI that enforces the outcome-union pattern
// for new error-returning GraphQL mutations.
//
// Usage (run from backend/):
//
//	go run ./cmd/schema-lint [flags]
//
// Flags:
//
//	-mode={error,warn}        Exit non-zero on violations (default: error).
//	-schema=path              Path, glob, or comma-list for schema files (default: ../schema/*.graphql).
//	-resolver=path            Path, glob, or comma-list for resolver files (default: graph/resolver/*.resolvers.go).
//	-resolver-struct=path     Path to resolver.go struct file (default: graph/resolver/resolver.go).
//	-usecase=dir              Path to usecase directory (default: internal/usecase).
//	-allowlist=path           Path to allowlist.txt (default: cmd/schema-lint/allowlist.txt).
//
// Exit codes:
//
//	0   No violations and no drifts (or -mode=warn).
//	1   At least one violation or drift in -mode=error.
//	2   Walker or configuration error.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// interfaceToImpl maps each Resolver field whose type is a usecase
// interface to the concrete impl type's lowercase identifier (the
// receiver type used by methods on that impl). For example:
//
//	"AdminUserUsecase": "adminUserUsecase"
//
// The value must be the receiver-type identifier exactly as it appears
// in the usecase impl files. Maintenance: add an entry for each new
// interface-typed field added to Resolver.
//
// See backend/cmd/schema-lint/README.md § "Maintenance" for guidance.
var interfaceToImpl = map[string]string{
	"AdminUserUsecase":           "adminUserUsecase",
	"AdminRoleUsecase":           "adminRoleUsecase",
	"CardImportUsecase":          "cardImportUsecase",
	"LastViewedCardgroupUsecase": "lastViewedCardgroupUsecase",
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point. It accepts the CLI args slice and explicit
// stdout/stderr writers so integration tests can invoke it without exec'ing
// the binary.
//
// Returns the desired exit code (0, 1, or 2).
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("schema-lint", flag.ContinueOnError)
	fs.SetOutput(stderr)

	mode := fs.String("mode", "error", "lint mode: 'error' (exit 1 on violations) or 'warn' (print but exit 0)")
	schemaPath := fs.String("schema", "../schema/*.graphql", "path, glob, or comma-list for GraphQL SDL files")
	resolverPath := fs.String("resolver", "graph/resolver/*.resolvers.go", "path, glob, or comma-list for generated resolver files")
	resolverStructPath := fs.String("resolver-struct", "graph/resolver/resolver.go", "path to resolver.go struct file")
	usecaseDir := fs.String("usecase", "internal/usecase", "path to usecase directory")
	allowlistPath := fs.String("allowlist", "cmd/schema-lint/allowlist.txt", "path to allowlist.txt")

	if err := fs.Parse(args); err != nil {
		// flag.ContinueOnError already printed the error.
		return 2
	}

	if *mode != "error" && *mode != "warn" {
		fmt.Fprintf(stderr, "schema-lint: -mode must be 'error' or 'warn', got %q\n", *mode)
		return 2
	}

	// --- Walk schema ---
	schemaPaths, err := expandPathList(*schemaPath)
	if err != nil {
		fmt.Fprintf(stderr, "schema-lint: schema path expansion failed: %v\n", err)
		return 2
	}
	schemaMutations, err := SchemaWalk(schemaPaths)
	if err != nil {
		fmt.Fprintf(stderr, "schema-lint: schema walk failed: %v\n", err)
		return 2
	}

	// --- Walk resolver ---
	resolverPaths, err := expandPathList(*resolverPath)
	if err != nil {
		fmt.Fprintf(stderr, "schema-lint: resolver path expansion failed: %v\n", err)
		return 2
	}
	resolverResult, err := ResolverWalkFiles(resolverPaths)
	if err != nil {
		fmt.Fprintf(stderr, "schema-lint: resolver walk failed: %v\n", err)
		return 2
	}

	// --- Walk usecase ---
	usecaseMethods, err := UsecaseWalk(*usecaseDir)
	if err != nil {
		fmt.Fprintf(stderr, "schema-lint: usecase walk failed: %v\n", err)
		return 2
	}

	// --- Load resolver field map ---
	fieldMap, err := LoadResolverFieldMap(*resolverStructPath)
	if err != nil {
		fmt.Fprintf(stderr, "schema-lint: resolver struct parse failed: %v\n", err)
		return 2
	}

	// --- Load allowlist ---
	allowlist, err := LoadAllowlist(*allowlistPath)
	if err != nil {
		fmt.Fprintf(stderr, "schema-lint: allowlist load failed: %v\n", err)
		return 2
	}

	cfg := ClassifierConfig{InterfaceToImpl: interfaceToImpl}

	// --- Allowlist rot check (informational, does not affect exit code) ---
	rotten := AllowlistRotEntries(schemaMutations, resolverResult, usecaseMethods, fieldMap, allowlist, cfg)
	for _, entry := range rotten {
		fmt.Fprintf(stderr, "allowlist-rot: %s is allowlisted but no longer violates; remove the entry\n", entry)
	}

	// --- Classify ---
	violations, drifts := Classify(schemaMutations, resolverResult, usecaseMethods, fieldMap, allowlist, cfg)

	// --- Print drifts ---
	for _, d := range drifts {
		fmt.Fprintf(stderr, "drift: %s\n", d.Description)
	}

	// --- Print violations ---
	for _, v := range violations {
		fmt.Fprintf(stderr,
			"violation: %s returns bare '%s' but '%s' emits typed ucerr.* errors; "+
				"add to allowlist.txt or promote to a *Result union "+
				"(see docs/backend/error-wrapping/outcome-union-enforcement.md)\n",
			v.Mutation, v.ReturnType, v.UsecaseTarget,
		)
	}

	if *mode == "warn" {
		return 0
	}

	// error mode: exit 1 if any violation or drift.
	if len(violations) > 0 || len(drifts) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "schema-lint: 0 violations, 0 drifts\n")
	return 0
}

func expandPathList(patterns string) ([]string, error) {
	parts := strings.Split(patterns, ",")
	seen := make(map[string]struct{}, len(parts))
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var matches []string
		if hasGlobMeta(part) {
			globbed, err := filepath.Glob(part)
			if err != nil {
				return nil, fmt.Errorf("bad glob %q", part)
			}
			if len(globbed) == 0 {
				return nil, fmt.Errorf("glob %q matched no files", part)
			}
			matches = globbed
		} else {
			matches = []string{part}
		}

		for _, match := range matches {
			clean := filepath.Clean(match)
			if _, ok := seen[clean]; ok {
				continue
			}
			seen[clean] = struct{}{}
			paths = append(paths, clean)
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no paths provided")
	}
	sort.Strings(paths)
	return paths, nil
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}
