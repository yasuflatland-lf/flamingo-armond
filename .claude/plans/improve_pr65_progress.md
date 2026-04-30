# PR-06.2 — Issue #65 progress (deferred review items from validateDictionary)

Branch: `feature/improve_pr6_2`
Source: triage of items A2 / A5 / A6 / A8 / A9 / A10 deferred from [#57] review.
Sibling: master plan PR-06 entry in `improve_master_plan.md`.

## Scope decisions

| Item | Decision | Rationale |
|------|----------|-----------|
| A2 | Already removed in [#57] simplifier sweep — close as completed by deletion | StructuredError + GetType() are gone; nothing to ship |
| A5 | Decline constructors; document the invariant inline | Issue recommendation; Go-idiomatic to keep value types open |
| A6 | Keep product type for now; document the trade-off near `DictionaryValidationResult` and `ValidateDictionary` resolver; revisit in [#59] | Issue says "decide alongside #59"; union type refactor itself belongs in #59 |
| A8 | Implement `NewResolver(...)` constructor; migrate all resolver tests | Removes a footgun; small enough to land here |
| A9 | Implement `gqlerr.Cancelled()`; apply to `ValidateDictionary` | Cross-cutting error shape; aligns with rest of resolvers |
| A10 | Make lexer `Peek` non-destructive via offset-track | Robustness improvement; protects against future reader swap |

## Out of scope (belongs to #59 — do NOT include here)

- The actual `DictionaryValidationResult` union type refactor (A6 implementation).
- `upsertDictionary` mutation, repository upsert, migration, admin import page.

## Parallel execution waves

```
Wave 1 (parallel; independent files)
├── Cluster W1-A: A5  — textdic/service.go doc comment                [Haiku]
├── Cluster W1-B: A10 — textdic/lexer.go non-destructive Peek + test  [Sonnet]
└── Cluster W1-C: A9 + A6 — gqlerr.Cancelled + apply + A6 decision    [Sonnet]
                  (gqlerr/errors.go, schema.resolvers.go, schema.graphql comment)

Wave 2 (sequential, after W1-C; modifies same resolver.go area)
└── Cluster W2-D: A8 — NewResolver(...) + migrate tests               [Opus high]
                  (resolver.go, cmd/server/main.go, dictionary_resolver_test.go,
                   card_resolvers_test.go, user_test.go, schema.resolvers.go)

Wave 3 (sequential, after W2)
├── Verify: go vet + build + race tests + language-policy grep        [Haiku]
└── Commit per cluster via dedicated commit subagent                  [Haiku]
```

## Files touched

| File | Cluster | Edit type |
|------|---------|-----------|
| `backend/internal/textdic/service.go` | W1-A | doc comment |
| `backend/internal/textdic/lexer.go` | W1-B | refactor Peek + doc |
| `backend/internal/textdic/lexer_test.go` (new or extend) | W1-B | regression test |
| `backend/internal/gqlerr/errors.go` | W1-C | add Cancelled() |
| `backend/internal/gqlerr/errors_test.go` (extend) | W1-C | helper unit test |
| `backend/graph/resolver/schema.resolvers.go` | W1-C, W2-D | use Cancelled(); A6 comment; constructor migration |
| `schema/schema.graphql` | W1-C | A6 decision comment |
| `backend/graph/resolver/resolver.go` | W2-D | NewResolver |
| `backend/cmd/server/main.go` | W2-D | use NewResolver |
| `backend/graph/resolver/dictionary_resolver_test.go` | W2-D | use NewResolver |
| `backend/graph/resolver/card_resolvers_test.go` | W2-D | use NewResolver |
| `backend/graph/resolver/user_test.go` | W2-D | use NewResolver |

## Commit granularity (one-line, no Co-Author footer)

1. `docs(plan): record PR-06.2 progress plan for issue #65`
2. `docs(textdic): note ParsedWord/ValidationError invariant (A5)`
3. `refactor(textdic): make lexer Peek non-destructive (A10)`
4. `feat(gqlerr): add Cancelled helper and apply to validateDictionary (A9)`
5. `docs(schema): record A6 decision to keep DictionaryValidationResult product type`
6. `refactor(graph): introduce NewResolver constructor and migrate tests (A8)`

## Status tracking

Each task sub-agent must update the matching row in the table below by editing this file
when it completes — set status to "done" and add commit SHA short hash.

| Task | Status | Commit |
|------|--------|--------|
| Plan + master pointer | done | (this commit) |
| A5 — service.go doc | done | — |
| A10 — lexer Peek | done | — |
| A9 — Cancelled helper | done | — |
| A6 — decision doc | done | — |
| A8 — NewResolver migration | done | — |
| Verification | pending | — |

## Cross-cutting rules

- English-only in committed text (per `.claude/rules/language-policy.md`).
- Conventional Commits, one line, no Co-Author footer.
- `go vet`, `go test -race`, language-policy grep clean before commit.
