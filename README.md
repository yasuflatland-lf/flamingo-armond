# flamingo-armond

🦩 Swiping Flashcard app — a Go (Echo) backend, a Next.js 16 frontend, and a shared GraphQL schema, with Supabase for Postgres + Auth.

## Quick Start

A first-time contributor should be able to go from a fresh clone to a logged-in local app in well under an hour by following the four phases below. Each phase has a single L2 doc as its source of truth — this section is the launchpad, not the manual.

### 1. Prerequisites

| Tool | Why | How to install |
|---|---|---|
| **mise** | Pins Go, Node, pnpm, and the Supabase CLI to the versions in `.tool-versions` files (no global drift) | `curl https://mise.run \| sh` |
| **Docker** | Backs `supabase start` (Postgres + Auth running locally) | Docker Desktop / OrbStack / colima |
| **Terraform + tflint** | Production-only; pinned in `ops/terraform/mise.toml`. Skip for local dev | `cd ops/terraform && mise install` |

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

```bash
# Install pinned Go + Node + pnpm + Supabase CLI (reads .tool-versions hierarchy)
mise install

# Install workspace dependencies (root + frontend)
pnpm install

# Pull Docker images and start local Supabase (Postgres + Auth on 127.0.0.1:54321)
supabase start
```

Then create a **Google OAuth client** for local dev and write `frontend/.env.local` and `backend/.env.local`. Both files are gitignored. The full procedure (Google Cloud Console steps, `127.0.0.1`-vs-`localhost` rule, exact env var names) lives in `docs/dev-setup.md` § "Supabase CLI" — start at the "Auth flow (read this first)" subsection.

> Local and production each use a **separate Google OAuth client**. Local credentials live in `frontend/.env.local`; production credentials live in Terraform variables. See `docs/dev-setup.md` and `docs/deployment.md` § 8.

### 3. Run locally

Two long-running processes; run them in separate terminals (Supabase is already up from step 2):

```bash
make dev-backend     # Go server on http://127.0.0.1:1323
make dev-frontend    # Next.js dev server on http://127.0.0.1:3000
```

Sign in via `http://127.0.0.1:3000/login` (Google) → land on `/profile`. The profile page issues an authenticated GraphQL `me` query through the Next rewrite, exercising the full Frontend → Backend → Supabase chain.

Use **`127.0.0.1`**, not `localhost`. Google OAuth treats them as distinct origins and will reject the redirect otherwise (`docs/dev-setup.md` § "Gotchas").

### 4. Production deployment

Production is **Terraform-driven** under `ops/terraform/envs/prod/`. A first-time bring-up is two `terraform apply` invocations:

```bash
cd ops/terraform/envs/prod/initial    # Supabase project + Render service + Vercel project
terraform init && terraform apply

cd ../settings                        # wires Vercel hostname into Supabase Auth
terraform init && terraform apply
```

Before applying, you collect a handful of human-only inputs (Supabase PAT, Render / Vercel API tokens, Google OAuth client credentials, GitHub App installs). After applying, you replace a placeholder Google redirect URI and trigger the first Render / Vercel deploy.

The complete checklist — including which steps Terraform handles automatically vs. which require human consent flows — is in `docs/deployment.md`. Read § "What Terraform handles vs. what you do by hand" first to set expectations.

## Makefile reference

The repo's Makefile is a thin convenience layer over `go`, `pnpm`, and `gqlgen` — every target is a one- or two-line shell recipe. Running `make` with no argument prints the same table:

```bash
make                  # shows this help (DEFAULT_GOAL)
```

| Target | What it does | Underlying command |
|---|---|---|
| `make install` | Install all workspace dependencies | `pnpm install` (root, hoists frontend) |
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
ops/terraform/      Production IaC (Supabase + Render + Vercel modules)
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
| `docs/deployment.md` | Production bring-up via Terraform (Supabase + Render + Vercel) |
| `.claude/rules/language-policy.md` | English-only rule for committed text, no PR-order references |
