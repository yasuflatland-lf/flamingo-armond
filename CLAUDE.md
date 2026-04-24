# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

## Repo layout

Monorepo for a swiping flashcard app (`🦩 flamingo-armond`):

- `backend/` — Go / Echo v5. The only area wired end-to-end (build, tests, CI, deploy).
- `frontend/` — TypeScript / GraphQL client. Scaffold only — see `docs/frontend.md` before inventing commands.
- `schema/` — Shared GraphQL schema (empty).

The architectural pivot is shared `schema/` → codegen on both sides (`gqlgen` for backend, `codegen` for frontend). Treat `backend/graph/{generated,model,resolver}/` and `frontend/src/` as the planned codegen sinks even while empty.

## Backend quickstart

Go is pinned via mise (`backend/.tool-versions`). Run from `backend/`:

```bash
go mod download
go vet ./...
go build ./...
go test -v -race -covermode=atomic -coverprofile=coverage.out ./...
go run ./cmd/server                          # PORT defaults to 1323
PORT=8080 SHUTDOWN_TIMEOUT=10s go run ./cmd/server
```

## Topic docs (source of truth)

- `docs/backend.md` — Echo v5 constraints, graceful shutdown contract, HTTP timeouts, env vars, testing patterns.
- `docs/ci.md` — coverage flags, Codecov fail-safe, Actions versioning, workflow triggers, Render deploy gating.
- `docs/frontend.md` — scaffolding caveats for `frontend/` and `schema/`.

Prefer updating a topic doc over expanding this file.
