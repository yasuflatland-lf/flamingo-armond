# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

## Repo layout

Monorepo for a swiping flashcard app (`🦩 flamingo-armond`):

- `backend/` — Go / Echo v5. The only area wired end-to-end (build, tests, CI, deploy).
- `frontend/` — TypeScript / GraphQL client. See `docs/frontend.md` before inventing commands.
- `schema/` — Shared GraphQL schema. Source of truth for both `gqlgen` (backend) and `codegen` (frontend); outputs land in `backend/graph/{generated,model}/` and `frontend/src/generated/`.

## Backend quickstart

Go is pinned via mise (`backend/.tool-versions`). Run from `backend/`:

```bash
go mod download && go vet ./... && go build ./...
go test -v -race -covermode=atomic -coverprofile=coverage.out ./...
go run ./cmd/server   # PORT defaults to 1323
```

## Doc tiers

This file is **L1**: keep it ≤35 lines, top-level orientation only. Overflow goes into:

- **L2 — `docs/`** — architecture & topic docs (source of truth for technical detail).
  - `backend.md`, `frontend.md`, `ci.md`, `dev-setup.md`, `deployment.md`, `playbook-patterns.md`, `pagination.md`.
- **L3 — `.claude/rules/`** — cross-cutting rules and conventions.
  - `language-policy.md` — English-only rule, no PR-order references, verification commands.

Prefer updating an L2 or L3 doc over expanding this file. When L1 grows past 35 lines, move the newest section down a tier and leave a one-line pointer here.
