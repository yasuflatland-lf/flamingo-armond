# schema/

Shared GraphQL schema. Source of truth for both gqlgen (backend) and graphql-codegen (frontend).

## Regeneration flow

- Backend: `cd backend && go tool gqlgen generate`
- Frontend: `pnpm --filter frontend codegen`

Both consumers must stay in sync with `schema/*.graphql`. Generated output lands in `backend/graph/{generated,model}/`, `backend/graph/resolver/*.resolvers.go`, and `frontend/src/generated/`.

## Topic docs

- [`docs/backend-graphql.md`](../docs/backend-graphql.md) — gqlgen wiring, resolver layer, DataLoader, authorization, validation.
- [`.claude/rules/error-wrapping.md`](../.claude/rules/error-wrapping.md) § "Errors as data — detailed cases" — outcome-union enforcement.

## Cross-cutting rules already in context

Error-wrapping (resolver translation) and outcome-union enforcement are auto-loaded via `.claude/rules/`. Do not duplicate them here.
