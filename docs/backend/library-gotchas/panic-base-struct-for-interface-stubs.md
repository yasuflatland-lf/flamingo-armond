# Embed a `panic` base struct to eliminate interface-stub boilerplate

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a large interface (e.g. `RoleRepository` with 12 methods) needs multiple test
doubles that each override only 1–2 methods, embedding a shared "panic base" struct
cuts boilerplate by ~55 lines per stub and keeps each double focused on the methods
under test.

```go
// panicRoleRepo implements every method of repository.RoleRepository by panicking.
// Embed it in test doubles that only need to override a subset of methods.
type panicRoleRepo struct{}

func (panicRoleRepo) Create(ctx context.Context, r model.Role) (model.Role, error) {
    panic("panicRoleRepo: Create not expected in this test")
}
// ... one method per interface member, all panicking ...

// Stub that only cares about FindByName:
type stubFindByNameRepo struct {
    panicRoleRepo
    result model.Role
    err    error
}
func (s stubFindByNameRepo) FindByName(ctx context.Context, name string) (model.Role, error) {
    return s.result, s.err
}
```

**Why:** any call to a method that was not intentionally overridden panics
immediately, surfacing the unexpected call in the test output rather than silently
returning a zero value that could mask a production logic bug. The panic message
names the method, making the gap obvious without inspecting the stub.

**How to apply:** define `panicXxx` once per interface in a `_test.go` file adjacent
to the tests. Each scenario-level stub embeds it and overrides only the methods the
scenario exercises. Do not share the base across packages — keep it local to the
test file so the panic message stays readable.

**Adding a method to an interface breaks test stubs in OTHER packages silently at the test level.** The production build (`go build ./...`) succeeds because production callers use the concrete implementation. But hand-rolled test doubles in packages such as `loader/` or `graph/resolver/` that embed the interface type stop compiling. Run `go build ./...` *before* `go test ./...` after any interface-method addition to surface the cascade before any test-run noise masks it. Panic-base stubs make the failure loud once the build passes: a missing override panics immediately rather than returning a silent zero-value that can mask a logic bug. Audit every hand-rolled stub for the new method on the same change.
