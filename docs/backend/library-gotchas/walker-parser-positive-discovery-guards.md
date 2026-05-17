# Walker / parser positive-discovery guards — silent zero disables enforcement

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Two libraries used by the `backend/cmd/schema-lint/` program return "silent zero" when their input is missing or degenerate, and either failure mode quietly disables downstream enforcement when the walker is plumbed into a CI gate:

- `go/parser.ParseDir(fset, dir, ...)` returns `(nil, nil)` — empty map, no error — for a directory that does not exist or contains no Go files matching the filter. Callers that range over the result simply produce zero `UsecaseMethod` entries and the classifier reports zero violations.
- `vektah/gqlparser/v2`'s `schema.Mutation` is `nil` when the parsed schema declares no `type Mutation`. `range mutation.Fields` over a `nil` pointer panics; defensively guarding with `if mutation == nil { return nil, nil }` produces the same silent-zero failure as above.

Both failure modes look like "the lint correctly found nothing to flag" from CI's vantage point. The wrong `-schema=` flag, a truncated file, or a typo in a directory path all produce a green CI run with no signal that the walker never inspected the source it was supposed to inspect.

**The rule:** guard every walker with **positive discovery** — fail loud when the input is missing or degenerate, rather than returning an empty result. The two patterns from `schemawalk.go` and `usecasewalk.go`:

```go
// schemawalk: an empty Mutation type means the schema was not what we expected.
if mutation == nil {
    return nil, eris.New(
        "schemawalk: schema has no Mutation type " +
        "(likely wrong schema path or truncated file)",
    )
}

// usecasewalk: stat the directory before parser.ParseDir; check for zero packages after.
if _, err := os.Stat(usecaseDir); err != nil {
    return nil, eris.Wrap(err, "usecasewalk: usecase dir")
}
// ... parser.ParseDir ...
if len(pkgs) == 0 {
    return nil, eris.Errorf(
        "usecasewalk: no Go packages found in %q (likely wrong dir path)",
        usecaseDir,
    )
}
```

**How to apply:** when adding a new walker that backs a CI gate, ask "what does this return for a missing or empty input?" If the answer is "empty slice, no error", insert a positive-discovery guard that fails fast with an actionable message (mention the wrong-flag / truncated-file hypothesis explicitly — the reader is debugging a CI failure and benefits from the most likely cause being named). The walker's unit tests must include a missing-input case that asserts the error is raised, not absorbed.

The same pattern applies anywhere else a library returns silent zero for a missing input: `os.ReadDir` returns an error and is safe; `filepath.Glob` returns `(nil, nil)` for "no matches" and is **not** safe and must be guarded the same way.
