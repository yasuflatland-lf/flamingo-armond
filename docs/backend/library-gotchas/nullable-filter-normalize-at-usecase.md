# Nullable filter fields: normalize `nil` / empty / whitespace at the usecase boundary

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A usecase input struct with a `Search *string` field has two representations of "no filter": `nil` (absent) and a pointer to an empty or whitespace-only string. Leaving both representations in circulation forces every downstream layer — repository, future callers, tests — to re-implement the same `if x != nil && strings.TrimSpace(*x) != ""` guard. The duplication spreads silently; a new caller that forgets the check passes a whitespace-only pointer to the repository, which runs a spurious `ILIKE '%  %'` predicate and returns unexpected results.

Normalize once at the layer that owns the input struct (the usecase, when the input is populated from the resolver). Collapse `nil`, `&""`, and `&"   "` to `nil`; trim leading/trailing whitespace from non-empty strings before storing. The repository then receives a simple invariant: nil means no filter, non-nil means a pre-trimmed, non-empty pattern ready for `escapeLike` + `ILIKE`.

```go
// usecase, before calling FindPageByCardgroup:
search := in.Search
if search != nil {
    trimmed := strings.TrimSpace(*search)
    if trimmed == "" {
        search = nil
    } else {
        search = &trimmed
    }
}
// repo receives: nil OR a trimmed, non-empty *string — never whitespace-only.
```

**Why:** the usecase knows what a blank search means to the user; the repository does not. Pushing the guard down into the repo mixes business semantics (blank = no filter) with persistence mechanics (ILIKE predicate). Pushing it up into the resolver leaks persistence knowledge (the `*string` pointer convention) into the GraphQL layer. The usecase is the right seam.

**How to apply:** add the normalization block at the top of any usecase method that accepts an optional filter `*string`. Keep a single defensive "empty-search returns all rows" integration test directly against the repository so the repo's contract is independently verified. The usecase test should enumerate all four input shapes — `nil`, `&""`, `&"   "`, and `&"  apple  "` — and assert the repository received the expected normalized value. This rule pairs with [GORM exact-match `FindBy*` helpers: callers own trimming, repos own nothing](gorm-exact-match-findby-trimming.md): that rule covers trimming for exact-match lookups; this rule covers nullable-filter normalization for substring lookups. Both push normalization to the layer with the strongest knowledge of caller intent.

Reference: `backend/internal/usecase/card.go` `ListCardsByCardgroupConnection`; the simplified repository predicate in `backend/internal/repository/card.go` `FindPageByCardgroup`.
