# flamingo-armond

[![backend CI](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/backend.yml/badge.svg?branch=main)](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/backend.yml)
[![frontend CI](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/frontend.yml/badge.svg?branch=main)](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/frontend.yml)
[![codecov backend](https://codecov.io/gh/yasuflatland-lf/flamingo-armond/branch/main/graph/badge.svg?flag=backend)](https://codecov.io/gh/yasuflatland-lf/flamingo-armond)
[![codecov frontend](https://codecov.io/gh/yasuflatland-lf/flamingo-armond/branch/main/graph/badge.svg?flag=frontend)](https://codecov.io/gh/yasuflatland-lf/flamingo-armond)

🦩 Swiping Flashcard app — a Go (Echo) backend, a Next.js 16 frontend, and a shared GraphQL schema, with Supabase for Postgres + Auth.

## Quick Start

A first-time contributor should be able to go from a fresh clone to a logged-in local app in well under an hour by following the four phases below. Each phase has a single L2 doc as its source of truth — this section is the launchpad, not the manual.

### 1. Prerequisites

| Tool | Why | How to install |
|---|---|---|
| **mise** | Pins Go, Node, pnpm, and the Supabase CLI to the versions in `.tool-versions` files (no global drift) | `curl https://mise.run \| sh` |
| **Docker** | Backs `supabase start` (Postgres + Auth running locally) | Docker Desktop / OrbStack / colima |

Versions resolved by `mise install` at the repo root:

- Go `1.26.2` — from `backend/.tool-versions`
- Node `24.x`, pnpm `10.33.2`, Supabase CLI — from `./.tool-versions`

Do not install pnpm via `npm i -g pnpm` or `brew install pnpm` — a PATH-level binary shadows the mise shim and silently breaks version pinning. If you previously installed it that way, uninstall it first.

Verify after installation:

```bash
node --version    # v24.x.y
pnpm --version    # 10.33.2 (from .tool-versions via mise)
which pnpm        # ~/.local/share/mise/shims/pnpm
```

See `docs/dev-setup.md` § "Tools" for rationale (why mise-managed pnpm, mise hierarchy).

### 2. Initial setup

Once the prerequisites above are installed (and Docker is running), one command does everything reproducible:

```bash
make setup
# → check-docker → mise install → pnpm install → supabase start → sync-env → check-google-oauth
```

The final `check-google-oauth` step inspects the root `./.env` (created from `.env.example` on the first run) and prints either:

- a **green confirmation** if `SUPABASE_AUTH_EXTERNAL_GOOGLE_CLIENT_ID` / `_SECRET` are real values, or
- a **red warning** with exactly which two lines to edit and the next command to run (`make supabase-restart`).

The **only manual step** is editing `./.env` to add the Google OAuth client credentials. Everything else (frontend `.env.local`, backend `.env.local`, mise tool installs, Supabase boot, key derivation) is handled by `make setup`.

> **Single source of truth.** The root `./.env` is the only file you hand-edit; `frontend/.env.local` and `backend/.env.local` are auto-generated and bear a `# managed-by: sync-env` marker on line 1. Re-running `make setup` is safe — managed files are regenerated, user-owned files (marker removed) are skipped with a notice. Full layout in `docs/dev-setup.md` § "Env file layout (single source of truth)".

The Google OAuth procedure (Cloud Console steps, `127.0.0.1`-vs-`localhost` rule, JavaScript origin & redirect URI) lives in `docs/dev-setup.md` § "Supabase CLI" — start at the "Auth flow (read this first)" subsection.

> Local and production each use a **separate Google OAuth client**. Local credentials live in the root `./.env`; production credentials are configured directly in the production Supabase Auth settings. See `docs/dev-setup.md` and `docs/deployment.md` section "Manual prerequisites" → Google OAuth client.

### 3. Run locally

Two long-running processes; run them in separate terminals (Supabase is already up from step 2):

```bash
make dev-backend     # Go server on http://127.0.0.1:1323
make dev-frontend    # Next.js dev server on http://127.0.0.1:3000
```

Sign in via `http://127.0.0.1:3000/login` (Google) → land on `/profile`. The profile page issues an authenticated GraphQL `me` query through the Next rewrite, exercising the full Frontend → Backend → Supabase chain.

Use **`127.0.0.1`**, not `localhost`. Google OAuth treats them as distinct origins and will reject the redirect otherwise (`docs/dev-setup.md` § "Gotchas").

### 4. Production deployment

Production runs across three providers — Supabase (Postgres + Auth), Render (Go backend), Vercel (Next.js frontend) — and is provisioned manually through each provider's dashboard. The first-time bring-up walks through:

1. Creating a Supabase project, capturing the project ref / API keys / DSN.
2. Creating a Render web service against `backend/`, wiring its env vars.
3. Creating a Vercel project against `frontend/`, wiring its env vars.
4. Looping the Vercel domain back into Supabase Auth (Site URL + redirect allow list) and replacing the Google OAuth redirect URI placeholder.

The full checklist with prerequisites (Supabase PAT, Render / Vercel API tokens, Google OAuth client credentials), env var matrices, and post-bring-up smoke tests lives in `docs/deployment.md`. For a guided run that wraps the manual procedure with prerequisite checks, value-derivation, GitHub Secret registration, and smoke tests, use `make setup-prod`.

## Makefile reference

The repo's Makefile is a thin convenience layer over `go`, `pnpm`, `gqlgen`, and an Ansible playbook (`playbooks/setup.yml`) that owns env-file generation and Supabase lifecycle. Running `make` with no argument prints the same table:

```bash
make                  # shows this help (DEFAULT_GOAL)
```

| Target | What it does | Underlying command |
|---|---|---|
| `make setup` | One-shot first-time setup (assumes prerequisites are installed) | `check-docker` → `mise install` → `ansible-playbook playbooks/setup.yml` |
| `make sync-env` | Idempotent env sync: seed `./.env` from example, derive `frontend/.env.local` keys from `supabase status -o env`, seed `backend/.env.local` | `ansible-playbook ... --tags sync-env` |
| `make check-google-oauth` | Verify root `./.env` has real Google OAuth credentials (warns if placeholder) | `ansible-playbook ... --tags google-check` |
| `make supabase-restart` | Stop and re-boot Supabase so it picks up edits to `./.env` | `supabase stop && ansible-playbook ... --tags supabase` |
| `make install` | Install all workspace dependencies | `ansible-playbook ... --tags pnpm` |
| `make dev-backend` | Start the Go / Echo backend on port 1323 | `cd backend && go run ./cmd/server` |
| `make dev-frontend` | Start the Next.js 16 dev server on port 3000 | `pnpm --filter frontend dev` |
| `make codegen` | Regenerate GraphQL bindings on both sides from `schema/*.graphql` | `go tool gqlgen generate` (backend) + `pnpm --filter frontend codegen` |
| `make test` | Run backend Go tests (with race + coverage) and frontend Vitest suite | `go test -race -covermode=atomic ./...` + `pnpm --filter frontend test` |

Tips:

- **`make dev-backend` and `make dev-frontend` are foreground processes.** Run them in separate terminals (or under `tmux` / your editor's tasks panel). They do not background themselves.
- **`make codegen` is required after editing `schema/*.graphql`.** Generated outputs are gitignored on purpose — see `docs/dev-setup.md` § "Policy on generated files". CI regenerates and verifies on every push.
- **`make test` mirrors CI.** If `make test` passes locally and CI fails, suspect environment drift (Go / Node / pnpm version, or a global pnpm shadowing the mise shim) before assuming a CI bug.
- **The Makefile lives at the repo root.** Backend-only targets still `cd backend &&` internally so you can invoke `make` from anywhere in the tree.
- **Direct invocation is fine.** `make dev-backend` is identical to `cd backend && go run ./cmd/server`. Use whichever you prefer; the Makefile is documentation, not a wrapper that adds behavior.

## Repository layout

```
backend/            Go / Echo backend (the only end-to-end-wired surface today)
frontend/           Next.js 16 App Router frontend
schema/             Shared GraphQL schema (source of truth for both codegen tools)
docs/               L2 architecture & topic docs
.claude/rules/      L3 cross-cutting conventions (language policy, etc.)
```

Project-wide guidance for AI assistants lives in `CLAUDE.md`.

## Further reading

| Doc | Topic |
|---|---|
| `docs/dev-setup.md` | Local toolchain, Supabase CLI, Google OAuth for local, codegen policy |
| `docs/backend.md` | Backend architecture, JWT verification, migrations |
| `docs/frontend.md` | Frontend architecture, backend rewrite contract, Profile page |
| `docs/ci.md` | CI workflows and what each one verifies |
| `docs/deployment.md` | Production bring-up across Supabase + Render + Vercel |
| `.claude/rules/language-policy.md` | English-only rule for committed text, no PR-order references |
