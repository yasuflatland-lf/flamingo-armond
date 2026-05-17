package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// repoRoot walks up from the directory of this source file to find the
// repository root (the directory that contains go.mod for the backend module).
// It uses runtime.Caller so the path is evaluated at compile time relative
// to the source file, which is reliable regardless of the working directory
// the test binary is run from.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../backend/cmd/schema-lint/schema_lint_integration_test.go
	// Go up three levels: schema-lint/ -> cmd/ -> backend/ -> repo root
	root := filepath.Join(filepath.Dir(file), "..", "..", "..")
	root = filepath.Clean(root)
	// Sanity check: schema/schema.graphql should exist at the root.
	if _, err := os.Stat(filepath.Join(root, "schema", "schema.graphql")); err != nil {
		t.Fatalf("repoRoot: schema/schema.graphql not found under %s: %v", root, err)
	}
	return root
}

// liveInputs resolves the canonical live-tree paths for all walkers.
func liveInputs(t *testing.T) (schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath string) {
	t.Helper()
	root := repoRoot(t)
	backend := filepath.Join(root, "backend")
	schemaPath = filepath.Join(root, "schema", "schema.graphql")
	resolverPath = filepath.Join(backend, "graph", "resolver", "schema.resolvers.go")
	resolverStructPath = filepath.Join(backend, "graph", "resolver", "resolver.go")
	usecaseDir = filepath.Join(backend, "internal", "usecase")
	allowlistPath = filepath.Join(backend, "cmd", "schema-lint", "allowlist.txt")
	return
}

// runLivePipeline runs all walkers + classifier against the live tree and
// returns violations, drifts, and the loaded allowlist (or a fatal).
func runLivePipeline(t *testing.T, al Allowlist) ([]Violation, []Drift) {
	t.Helper()
	schemaPath, resolverPath, resolverStructPath, usecaseDir, _ := liveInputs(t)

	schemaMutations, err := SchemaWalk([]string{schemaPath})
	if err != nil {
		t.Fatalf("SchemaWalk: %v", err)
	}
	resolverResult, err := ResolverWalk(resolverPath)
	if err != nil {
		t.Fatalf("ResolverWalk: %v", err)
	}
	usecaseMethods, err := UsecaseWalk(usecaseDir)
	if err != nil {
		t.Fatalf("UsecaseWalk: %v", err)
	}
	fieldMap, err := LoadResolverFieldMap(resolverStructPath)
	if err != nil {
		t.Fatalf("LoadResolverFieldMap: %v", err)
	}

	cfg := ClassifierConfig{InterfaceToImpl: interfaceToImpl}
	return Classify(schemaMutations, resolverResult, usecaseMethods, fieldMap, al, cfg)
}

// TestIntegration_LiveTree_NoViolations asserts that the full lint pipeline
// reports zero violations and zero drifts against the current working tree
// when the production allowlist is applied.
func TestIntegration_LiveTree_NoViolations(t *testing.T) {
	_, _, _, _, allowlistPath := liveInputs(t)

	al, err := LoadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("LoadAllowlist: %v", err)
	}

	violations, drifts := runLivePipeline(t, al)

	if diff := cmp.Diff([]Violation(nil), violations); diff != "" {
		t.Errorf("unexpected violations (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]Drift(nil), drifts); diff != "" {
		t.Errorf("unexpected drifts (-want +got):\n%s", diff)
	}
}

// TestIntegration_AllowlistReleaseValve confirms that removing a known
// bare-emit entry from the allowlist causes the classifier to report a
// violation for that mutation. The test operates on an in-memory copy of the
// allowlist, leaving the on-disk file untouched.
func TestIntegration_AllowlistReleaseValve(t *testing.T) {
	_, _, _, _, allowlistPath := liveInputs(t)

	al, err := LoadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("LoadAllowlist: %v", err)
	}

	// "updateProfile" is a known bare-emit mutation frozen in the allowlist.
	// Remove it from the in-memory copy to simulate a line deletion.
	const targetMutation = "updateProfile"
	if _, present := al[targetMutation]; !present {
		t.Fatalf("test precondition failed: %q not found in allowlist; update the test if the mutation was promoted", targetMutation)
	}
	delete(al, targetMutation)

	violations, _ := runLivePipeline(t, al)

	found := false
	for _, v := range violations {
		if v.Mutation == targetMutation {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a violation for %q after removing it from the allowlist, but none was reported; violations: %v", targetMutation, violations)
	}
}

// TestIntegration_LiveTree_NoAllowlistRot is a tripwire that asserts every
// entry in the on-disk allowlist still corresponds to a bare-emit mutation in
// the live tree. A future PR that promotes a mutation without removing the
// allowlist entry will fail this test.
func TestIntegration_LiveTree_NoAllowlistRot(t *testing.T) {
	schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath := liveInputs(t)

	al, err := LoadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("LoadAllowlist: %v", err)
	}

	schemaMutations, err := SchemaWalk([]string{schemaPath})
	if err != nil {
		t.Fatalf("SchemaWalk: %v", err)
	}
	resolverResult, err := ResolverWalk(resolverPath)
	if err != nil {
		t.Fatalf("ResolverWalk: %v", err)
	}
	usecaseMethods, err := UsecaseWalk(usecaseDir)
	if err != nil {
		t.Fatalf("UsecaseWalk: %v", err)
	}
	fieldMap, err := LoadResolverFieldMap(resolverStructPath)
	if err != nil {
		t.Fatalf("LoadResolverFieldMap: %v", err)
	}

	cfg := ClassifierConfig{InterfaceToImpl: interfaceToImpl}
	rotten := AllowlistRotEntries(schemaMutations, resolverResult, usecaseMethods, fieldMap, al, cfg)

	if len(rotten) != 0 {
		t.Errorf("allowlist rot detected; remove these stale entries from allowlist.txt: %v", rotten)
	}
}

// TestIntegration_PromotedMutations_NotInAllowlist_NotViolation verifies that
// every mutation promoted to a result-union return type satisfies two
// invariants:
//   - The mutation name must NOT appear in the on-disk allowlist (it was
//     removed when the return type became a union).
//   - Running the classifier with the production allowlist must NOT produce a
//     violation for the mutation (the union return type eliminates the bare
//     object that triggers the lint rule).
func TestIntegration_PromotedMutations_NotInAllowlist_NotViolation(t *testing.T) {
	_, _, _, _, allowlistPath := liveInputs(t)

	al, err := LoadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("LoadAllowlist: %v", err)
	}

	violations, _ := runLivePipeline(t, al)

	promoted := []string{"updateRole", "revokeRole", "adminUpdateUser"}
	for _, mutation := range promoted {
		mutation := mutation
		t.Run(mutation, func(t *testing.T) {
			if _, present := al[mutation]; present {
				t.Errorf("%s is in the allowlist but should have been promoted; remove it from allowlist.txt", mutation)
			}
			for _, v := range violations {
				if v.Mutation == mutation {
					t.Errorf("unexpected violation for %s: %+v (promotion should have eliminated the bare return type)", mutation, v)
				}
			}
		})
	}
}
