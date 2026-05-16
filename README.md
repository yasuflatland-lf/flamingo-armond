# Flamingo Armond

[![backend CI](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/backend.yml/badge.svg?branch=main)](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/backend.yml)
[![frontend CI](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/frontend.yml/badge.svg?branch=main)](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/frontend.yml)
[![codecov backend](https://codecov.io/gh/yasuflatland-lf/flamingo-armond/branch/main/graph/badge.svg?flag=backend)](https://codecov.io/gh/yasuflatland-lf/flamingo-armond)
[![codecov frontend](https://codecov.io/gh/yasuflatland-lf/flamingo-armond/branch/main/graph/badge.svg?flag=frontend)](https://codecov.io/gh/yasuflatland-lf/flamingo-armond)

🦩 Swiping Flashcard app — a Go (Echo) backend, a Next.js 16 frontend, and a shared GraphQL schema, with Supabase for Postgres + Auth, and optional Notion page sync that imports cards from Notion pages on a 6-hourly schedule.

## Architecture

![Production architecture](docs/images/architecture.png)

## Pre-conditions

Production deployment uses three external services. Create accounts before running `make setup-prod` — free tiers cover everything this repo provisions:

- **Supabase** — managed Postgres + Auth: https://supabase.com
- **Render** — hosts the Go backend: https://render.com
- **Vercel** — hosts the Next.js frontend: https://vercel.com

Local development does not require any of these — `make setup` boots a local Supabase via Docker. Provider tokens and the env-var matrix live in `docs/deployment.md`.

**Notion sync (optional)** — set up a Notion integration token and page IDs to enable automated card imports. See `docs/notion-sync.md` for the full env-var matrix, local testing workflow, and production operational runbook.

## Quick Start

### 1. Prerequisites

| Tool | Why | How to install |
|---|---|---|
| **mise** | Pins Go, Node, pnpm, and the Supabase CLI to the versions in `.tool-versions` files | `curl https://mise.run \| sh` |
| **Docker** | Backs `supabase start` (Postgres + Auth running locally) | Docker Desktop / OrbStack / colima |

Do not install pnpm via `npm i -g pnpm` or `brew install pnpm` — a PATH-level binary shadows the mise shim and silently breaks version pinning. See `docs/dev-setup.md` § "Tools".

### 2. Initial setup

With Docker running:

```bash
make setup
# → check-docker → mise install → pnpm install → supabase start → sync-env → check-google-oauth
```

The **only manual step** is editing `./.env` to add Google OAuth client credentials. `frontend/.env.local` and `backend/.env.local` are auto-generated and re-running `make setup` is safe. Full procedure (Cloud Console steps, `127.0.0.1`-vs-`localhost` rule) in `docs/dev-setup.md` § "Supabase CLI".

### 3. Run locally

```bash
make dev-backend     # Go server on http://127.0.0.1:1323
make dev-frontend    # Next.js dev server on http://127.0.0.1:3000
```

Sign in via `http://127.0.0.1:3000/login` (Google) → land on `/profile`. Use **`127.0.0.1`**, not `localhost` — Google OAuth treats them as distinct origins.

### 4. Production deployment

Production runs across Supabase (Postgres + Auth), Render (Go backend), and Vercel (Next.js frontend), provisioned manually through each provider's dashboard. Full checklist in `docs/deployment.md`; `make setup-prod` wraps it with prerequisite checks and smoke tests.

## Setup For Production

`make setup-prod` is a guided wrapper around the manual `docs/deployment.md` runbook: it validates Supabase / Render / Vercel / GitHub tokens, hands off to the provider dashboards, and runs smoke tests once each service is up. Prerequisites (PATs, Google OAuth client, GitHub App installations) and the env var matrix live in `docs/deployment.md`.

```bash
make setup-prod-preflight   # validate tokens & GitHub App installations only (unattended)
make setup-prod             # full guided bring-up
make setup-prod-postapply   # re-run first Render deploy + smoke tests from .state.yml
```

`make teardown-prod` is the destructive inverse — only use it on environments created by `setup-prod`.

## Makefile reference

| Target | What it does |
|---|---|
| `make setup` | One-shot first-time local setup |
| `make sync-env` | Idempotent env sync from `supabase status -o env` |
| `make supabase-restart` | Re-boot Supabase to pick up `./.env` edits |
| `make dev-backend` | Start the Go / Echo backend on port 1323 |
| `make dev-frontend` | Start the Next.js 16 dev server on port 3000 |
| `make codegen` | Regenerate GraphQL bindings on both sides from `schema/*.graphql` |
| `make test` | Run backend Go tests (race + coverage) and frontend Vitest suite |
| `make setup-prod` | Guided production bring-up across Supabase + Render + Vercel (see § "Setup For Production") |
| `make notion-local-setup` | Write `NOTION_*` keys into `backend/.env.local` from root `.env` (one-time, Notion sync) |
| `make notion-local-run` | Fire a single authenticated POST to local `/internal/notion-sync` |
| `make sync-notion-preflight` | Verify root `.env` contains all required `NOTION_*` keys (read-only) |
| `make sync-notion-secrets` | Push `NOTION_*` keys to Render env and GitHub Actions secrets |

`make dev-backend` / `make dev-frontend` are foreground processes — run them in separate terminals. `make codegen` is required after editing `schema/*.graphql` (generated outputs are gitignored). `make test` mirrors CI.

## Repository layout

```
backend/            Go / Echo backend (the only end-to-end-wired surface today)
frontend/           Next.js 16 App Router frontend
schema/             Shared GraphQL schema (source of truth for both codegen tools)
docs/               L2 architecture & topic docs
.claude/rules/      L3 cross-cutting conventions
```

Project-wide guidance for AI assistants lives in `CLAUDE.md`.

## Further reading

| Document | Contents |
|---|---|
| `docs/notion-sync.md` | Notion page sync — env vars, local testing, GitHub Actions setup, operational runbook |
| `docs/deployment.md` | Production deployment checklist and full env-var matrix |
| `docs/doc-organization.md` | L1/L2/L3 tier conventions and doc-split rules |
| `docs/dev-setup.md` | Local development setup details (OAuth, tool versions, Supabase CLI) |
| `docs/backend.md` | Backend runtime notes (Echo v5, GORM, graceful shutdown, env vars) |
| `docs/frontend.md` | Frontend notes (commands, routing, Apollo cache patterns) |
| `docs/ci.md` | CI configuration and workflow reference |
| `docs/observability.md` | Logging and structured error reporting |
| `docs/e2e.md` | End-to-end test setup and conventions |
