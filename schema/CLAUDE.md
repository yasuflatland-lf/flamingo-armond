# schema/

Shared GraphQL schema. Source of truth for both gqlgen (backend) and graphql-codegen (frontend).

## Regeneration flow

- Backend: `cd backend && go tool gqlgen generate`
- Frontend: `pnpm --filter frontend codegen`

Both consumers must stay in sync with `schema/*.graphql`. Generated output lands in `backend/graph/{generated,model}/`, `backend/graph/resolver/*.resolvers.go`, and `frontend/src/generated/`.

## Linting (CI-gated — run after any `schema/*.graphql` edit)

The CI job **Lint, typecheck, build** runs `pnpm lint:schema` (root `package.json` → `eslint --max-warnings=0 schema`, graphql-eslint). It is **separate** from `backend/cmd/schema-lint` (the outcome-union gate) and from `pnpm codegen` — passing those two does NOT mean the schema lints. Run it locally on any schema change:

```bash
pnpm lint:schema   # from the repo root
```

Two rules bite new types most often:

- **`@graphql-eslint/require-description`** — every `type` / `enum` / `input` needs a leading `"""description"""`. Field-level descriptions are optional.
- **`@graphql-eslint/no-typename-prefix`** — a field whose name starts with its parent type's name is rejected. A foreign-key id like `MasterCard.masterCardgroupId` (names the related type, not a stutter) must carry a `"""description"""` plus an inline `# eslint-disable-next-line @graphql-eslint/no-typename-prefix` on the line above the field — mirror `Card.cardgroupId` in `schema/card.graphql`.

## Topic docs

- [`docs/backend-graphql.md`](../docs/backend-graphql.md) — gqlgen wiring, resolver layer, DataLoader, authorization, validation.
- [`.claude/rules/error-wrapping.md`](../.claude/rules/error-wrapping.md) § "Errors as data — detailed cases" — outcome-union enforcement.

## Cross-cutting rules already in context

Error-wrapping (resolver translation) and outcome-union enforcement are auto-loaded via `.claude/rules/`. Do not duplicate them here.
