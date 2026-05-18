package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// liveArgsForRun builds the explicit flag slice that points run() at the live
// working-tree paths. It mirrors liveInputs() from schema_lint_integration_test.go
// but returns the run() args slice directly.
func liveArgsForRun(t *testing.T, extraArgs ...string) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../backend/cmd/schema-lint/main_test.go
	// Up three levels: schema-lint/ -> cmd/ -> backend/ -> repo root.
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	backend := filepath.Join(root, "backend")

	args := []string{
		"-schema=" + filepath.Join(root, "schema", "*.graphql"),
		"-resolver=" + filepath.Join(backend, "graph", "resolver", "*.resolvers.go"),
		"-resolver-struct=" + filepath.Join(backend, "graph", "resolver", "resolver.go"),
		"-usecase=" + filepath.Join(backend, "internal", "usecase"),
		"-allowlist=" + filepath.Join(backend, "cmd", "schema-lint", "allowlist.txt"),
	}
	return append(args, extraArgs...)
}

// syntheticFixtures writes the minimal set of files needed for one violation
// into dir and returns the paths expected by run().
//
// Schema: bareRole mutation returns Role! (bare object → IsBare=true).
// Resolver: calls r.RoleUC.Manage(...).
// Resolver struct: RoleUC field typed as *roleUsecase.
// Usecase: Manage method emits ucerr.NewValidationError → EmitsTypedError=true.
// Allowlist: empty.
//
// This combination produces exactly one violation in -mode=error.
func syntheticFixtures(t *testing.T, dir string) (schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath string) {
	t.Helper()

	// schema.graphql — bare return type
	schemaContent := `type Query { health: String! }
type Role { id: ID! name: String! }
type Mutation { bareRole(id: ID!): Role! }
`
	schemaPath = filepath.Join(dir, "schema.graphql")
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	// schema.resolvers.go — three-segment usecase call r.RoleUC.Manage(...)
	resolverContent := `package resolver
import "context"
type mutationResolver struct { RoleUC roleUsecaseIface }
type roleUsecaseIface interface { Manage(ctx context.Context, id string) (interface{}, error) }
func (r *mutationResolver) BareRole(ctx context.Context, id string) (interface{}, error) {
	return r.RoleUC.Manage(ctx, id)
}
`
	resolverPath = filepath.Join(dir, "schema.resolvers.go")
	if err := os.WriteFile(resolverPath, []byte(resolverContent), 0o644); err != nil {
		t.Fatalf("write resolver: %v", err)
	}

	// resolver.go — Resolver struct with RoleUC field
	resolverStructContent := `package resolver
type Resolver struct { RoleUC *roleUsecase }
`
	resolverStructPath = filepath.Join(dir, "resolver.go")
	if err := os.WriteFile(resolverStructPath, []byte(resolverStructContent), 0o644); err != nil {
		t.Fatalf("write resolver struct: %v", err)
	}

	// usecase dir with one .go file
	usecaseDir = filepath.Join(dir, "usecase")
	if err := os.MkdirAll(usecaseDir, 0o755); err != nil {
		t.Fatalf("mkdir usecase: %v", err)
	}
	usecaseContent := `package usecase
import (
	"context"
	"backend/internal/usecase/ucerr"
)
type roleUsecase struct{}
func (u *roleUsecase) Manage(ctx context.Context, id string) (interface{}, error) {
	if id == "" { return nil, ucerr.NewValidationError("id", "required") }
	return nil, nil
}
`
	if err := os.WriteFile(filepath.Join(usecaseDir, "role.go"), []byte(usecaseContent), 0o644); err != nil {
		t.Fatalf("write usecase: %v", err)
	}

	// empty allowlist
	allowlistPath = filepath.Join(dir, "allowlist.txt")
	if err := os.WriteFile(allowlistPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	return
}

// buildSyntheticArgs assembles the -flag=value slice for run() from synthetic fixture paths.
func buildSyntheticArgs(mode, schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath string) []string {
	return []string{
		"-mode=" + mode,
		"-schema=" + schemaPath,
		"-resolver=" + resolverPath,
		"-resolver-struct=" + resolverStructPath,
		"-usecase=" + usecaseDir,
		"-allowlist=" + allowlistPath,
	}
}

// ---------------------------------------------------------------------------
// Test 1: run() orchestrator tests
// ---------------------------------------------------------------------------

// TestRun_DefaultFlags_LiveTree_ExitsZero calls run with explicit live-tree
// paths (mirroring the defaults but resolved to absolute paths so the test
// binary's working directory does not matter). The live tree is lint-clean, so
// the exit code must be 0 and stdout must contain "0 violations, 0 drifts".
func TestRun_DefaultFlags_LiveTree_ExitsZero(t *testing.T) {
	// Not parallel: reads the live working tree; no mutation.
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	args := liveArgsForRun(t)
	code := run(args, stdout, stderr)
	if code != 0 {
		t.Fatalf("run with live-tree flags: want exit 0, got %d\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "0 violations, 0 drifts") {
		t.Errorf("stdout does not contain '0 violations, 0 drifts':\n%s", stdout.String())
	}
}

// TestRun_WarnMode_SyntheticViolation_ExitsZero verifies that -mode=warn
// exits 0 even when there is one violation, and that the violation is printed
// to stderr with the "violation:" prefix.
func TestRun_WarnMode_SyntheticViolation_ExitsZero(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath := syntheticFixtures(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	args := buildSyntheticArgs("warn", schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath)
	code := run(args, stdout, stderr)

	if code != 0 {
		t.Fatalf("run -mode=warn with violation: want exit 0, got %d\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "violation:") {
		t.Errorf("stderr does not contain 'violation:' prefix:\n%s", stderr.String())
	}
}

// TestRun_ErrorMode_SyntheticViolation_ExitsOne verifies that -mode=error
// exits 1 when there is at least one violation.
func TestRun_ErrorMode_SyntheticViolation_ExitsOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath := syntheticFixtures(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	args := buildSyntheticArgs("error", schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath)
	code := run(args, stdout, stderr)

	if code != 1 {
		t.Fatalf("run -mode=error with violation: want exit 1, got %d\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
}

func TestRun_GlobInputs_SyntheticViolation_ExitsOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath := syntheticFixtures(t, dir)

	// Add a second file for each input kind so the glob path exercises
	// multi-file expansion rather than a single matched path.
	if err := os.WriteFile(filepath.Join(dir, "base.graphql"), []byte("extend type Query { noop: String }\n"), 0o644); err != nil {
		t.Fatalf("write extra schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "base.resolvers.go"), []byte("package resolver\n"), 0o644); err != nil {
		t.Fatalf("write extra resolver: %v", err)
	}
	if err := os.Rename(schemaPath, filepath.Join(dir, "feature.graphql")); err != nil {
		t.Fatalf("rename schema: %v", err)
	}
	if err := os.Rename(resolverPath, filepath.Join(dir, "feature.resolvers.go")); err != nil {
		t.Fatalf("rename resolver: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	args := buildSyntheticArgs(
		"error",
		filepath.Join(dir, "*.graphql"),
		filepath.Join(dir, "*.resolvers.go"),
		resolverStructPath,
		usecaseDir,
		allowlistPath,
	)
	code := run(args, stdout, stderr)

	if code != 1 {
		t.Fatalf("run -mode=error with glob inputs: want exit 1, got %d\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "violation:") {
		t.Errorf("stderr does not contain 'violation:' prefix:\n%s", stderr.String())
	}
}

// TestRun_BadFlag_ExitsTwo verifies that an unknown flag produces exit code 2.
func TestRun_BadFlag_ExitsTwo(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := run([]string{"-not-a-real-flag"}, stdout, stderr)
	if code != 2 {
		t.Fatalf("run with bad flag: want exit 2, got %d\nstderr: %s", code, stderr.String())
	}
}

// TestRun_MissingSchemaFile_ExitsTwo verifies that pointing -schema= at a
// non-existent path produces exit code 2 and an error message on stderr.
func TestRun_MissingSchemaFile_ExitsTwo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	nonExistent := filepath.Join(dir, "no-such-schema.graphql")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := run([]string{"-schema=" + nonExistent}, stdout, stderr)
	if code != 2 {
		t.Fatalf("run with missing schema: want exit 2, got %d\nstderr: %s", code, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Error("expected non-empty stderr on missing schema file")
	}
}

// TestRun_DriftOnly_ErrorMode_ExitsOne verifies that a mutation present in the
// schema but with no corresponding resolver method (drift only, no violation)
// still produces exit code 1 in -mode=error. This confirms that drift alone is
// sufficient to fail CI.
func TestRun_DriftOnly_ErrorMode_ExitsOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Schema declares "ghostMutation"; resolver has no such method.
	schemaContent := `type Query { health: String! }
type Ghost { id: ID! }
type Mutation { ghostMutation(id: ID!): Ghost! }
`
	schemaPath := filepath.Join(dir, "schema.graphql")
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	// Resolver file that declares a mutationResolver with no methods at all.
	resolverContent := `package resolver
type mutationResolver struct{}
`
	resolverPath := filepath.Join(dir, "schema.resolvers.go")
	if err := os.WriteFile(resolverPath, []byte(resolverContent), 0o644); err != nil {
		t.Fatalf("write resolver: %v", err)
	}

	resolverStructContent := `package resolver
type Resolver struct{}
`
	resolverStructPath := filepath.Join(dir, "resolver.go")
	if err := os.WriteFile(resolverStructPath, []byte(resolverStructContent), 0o644); err != nil {
		t.Fatalf("write resolver struct: %v", err)
	}

	usecaseDir := filepath.Join(dir, "usecase")
	if err := os.MkdirAll(usecaseDir, 0o755); err != nil {
		t.Fatalf("mkdir usecase: %v", err)
	}
	usecaseContent := `package usecase
type stubUsecase struct{}
func (u *stubUsecase) Noop() {}
`
	if err := os.WriteFile(filepath.Join(usecaseDir, "stub.go"), []byte(usecaseContent), 0o644); err != nil {
		t.Fatalf("write usecase: %v", err)
	}

	allowlistPath := filepath.Join(dir, "allowlist.txt")
	if err := os.WriteFile(allowlistPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	args := buildSyntheticArgs("error", schemaPath, resolverPath, resolverStructPath, usecaseDir, allowlistPath)
	code := run(args, stdout, stderr)

	if code != 1 {
		t.Fatalf("drift-only run -mode=error: want exit 1, got %d\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
	stderrStr := stderr.String()
	if strings.Contains(stderrStr, "violation:") {
		t.Errorf("expected drift-only stderr without 'violation:' prefix, got: %s", stderrStr)
	}
	if !strings.Contains(stderrStr, "drift:") {
		t.Errorf("expected drift-only stderr to contain 'drift:' prefix, got: %s", stderrStr)
	}
}
