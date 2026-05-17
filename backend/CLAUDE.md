# backend/

Go / Echo v5 backend. The L1 (`CLAUDE.md` at repo root) applies; this file adds backend-specific orientation.

## Layout

- `cmd/` — entrypoints (server, seed, schema-lint)
- `internal/` — domain, usecase, repository, auth, middleware
- `graph/` — gqlgen output and resolver layer

## Quickstart

Go is pinned via mise (`backend/.tool-versions`). Run from `backend/`:

```bash
go mod download && go vet ./... && go build ./...
go test -v -race -covermode=atomic -coverprofile=coverage.out ./...
go run ./cmd/server   # PORT defaults to 1323
```

## Topic docs

- `docs/backend.md` — runtime notes (Echo v5, GORM, graceful shutdown)
- `docs/backend-auth.md` — Supabase JWT, RLS, role management
- `docs/backend-db.md` — migrations, recover, Custom Access Token Hook
- `docs/backend-graphql.md` — gqlgen, resolver, schema-lint
- `docs/observability.md` — logging contracts (shared with frontend)

## Cross-cutting rules already in context

Layer dependency, error wrapping, library gotchas, language policy are auto-loaded via `.claude/rules/`. Do not duplicate them here.
