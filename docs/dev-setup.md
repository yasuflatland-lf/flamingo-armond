# Dev setup

## Tools

- mise (`curl https://mise.run | sh`) — manages Go (`backend/.tool-versions`), Node + pnpm + Supabase CLI (`./.tool-versions`), and Terraform + tflint (`ops/terraform/mise.toml`).
- Supabase CLI — local Postgres / Auth emulation. Pinned to a specific version in `./.tool-versions` (do not switch back to `supabase latest`: the CLI breaks `supabase/config.toml` across major upgrades, so the version that everyone runs must be exact).

## First-time setup

```bash
# Install Go 1.26.2 (backend/.tool-versions), Node 24.x + pnpm 10.33.2 + Supabase CLI (./.tool-versions).
mise install

# Install workspace deps. The frontend workspace is populated with a Next.js 16 App Router scaffold (see `docs/frontend.md`).
pnpm install
```

Verify:

```bash
node --version        # v24.x.y
pnpm --version        # 10.33.2  (resolved by mise from .tool-versions)
which pnpm            # ~/.local/share/mise/shims/pnpm
```

`which pnpm` returning `~/.local/share/mise/shims/pnpm` is the expected mise shim. A `/usr/local/bin/pnpm` or `/opt/homebrew/bin/pnpm` path indicates a global install that will shadow the mise shim — uninstall it (`npm uninstall -g pnpm` / `brew uninstall pnpm`) before running `pnpm` again.

## Day-to-day

| Task | Command |
|---|---|
| Run backend | `make dev-backend` or `cd backend && go run ./cmd/server` |
| Run frontend | `make dev-frontend` |
| Regenerate GraphQL code | `make codegen` |
| Run all tests | `make test` |

## Policy on generated files

Both codegen outputs are **gitignored** — neither is committed:

| Tool | Input | Output (gitignored) | Regeneration command |
|---|---|---|---|
| gqlgen | `schema/*.graphql`, `backend/gqlgen.yml`, `backend/go.mod` (`tool` directive) | `backend/graph/generated/`, `backend/graph/model/models_gen.go` | `cd backend && go tool gqlgen generate` |
| graphql-codegen | `schema/*.graphql`, `frontend/codegen.ts`, `frontend/src/**/*.{ts,tsx}` | `frontend/src/generated/` | `pnpm --filter frontend codegen` |

Determinism relies on pinned tool versions (in `go.mod` and `package.json`) plus the committed schema. CI runs the backend regeneration before `go vet` and `go test` (`.github/workflows/backend.yml`). Frontend CI runs `pnpm --filter frontend codegen` before Biome check / typecheck / build (`.github/workflows/frontend.yml`), mirroring the backend contract. No `git diff --exit-code` step is needed because the outputs are not tracked.

Rationale: keeps PR diffs to hand-written code only and removes the merge-conflict churn that committing thousand-line generated files causes. Applied symmetrically to both stacks for consistency.

> **Note**: `backend/graph/resolver/*.resolvers.go` are resolver stubs, not generated output. They are **committed** and CI verifies they are up-to-date via `git diff --exit-code -- graph/resolver/*.resolvers.go` in `backend.yml`. This is orthogonal to the "generated files are ignored" policy above.

## `.tool-versions` hierarchy (mise)

mise resolves tool config hierarchically. Three scopes coexist without conflict:

| Scope | File | Tools |
|---|---|---|
| Backend | `backend/.tool-versions` | Go |
| Repo root (frontend + dev) | `./.tool-versions` | Node, pnpm, Supabase CLI |
| Ops (production IaC) | `ops/terraform/mise.toml` | Terraform, tflint |

Backend CI sets `working_directory: backend` and sees only Go. Frontend CI runs from the repo root — NOT `working_directory: frontend` — because Node and pnpm are declared in the root `.tool-versions`. Terraform CI sets `working_directory: ops/terraform` and picks up `mise.toml` (Terraform + tflint + the shared `TF_VAR_*` env block).

`mise.toml` is used in `ops/terraform/` (instead of `.tool-versions`) so the same file can declare both `[tools]` and `[env]` for shared `TF_VAR_*` defaults. The two formats are equivalent for tool pinning.

## Why mise-managed pnpm, not global pnpm or Corepack

The `[tools]` entries in `.tool-versions` are the single source of truth for pnpm in local dev and GitHub Actions. mise downloads the exact pinned version on demand, so every contributor and every CI runner uses the same pnpm — no drift, no "works on my machine". The `packageManager` field in root `package.json` is kept aligned and is **load-bearing for Vercel and for pnpm itself**: Vercel does not run mise and reads this field to install the matching pnpm on its build image, and pnpm 10 uses it as a self-consistency check that refuses execution when the declared and running versions disagree. Treat the two pins as one unit — bump them together.

Do **not** install pnpm via `npm i -g pnpm` or `brew install pnpm`. Those paths compete with the mise shim on PATH, and whichever wins is timing-dependent. Corepack is no longer used in this repo — `corepack enable` is unnecessary and can be skipped or disabled.

## Supabase CLI

Local development is self-contained behind `supabase start` (no production Supabase project required; the production project is wired up separately).

### Auth flow (read this first)

Google sign-in is brokered by Supabase Auth. The browser hits Google → Google redirects to **Supabase's** `/auth/v1/callback` → Supabase exchanges the code and redirects to the Next.js app's `/auth/callback`. The redirect URI registered with Google is therefore the Supabase host, not the Next.js host:

| Environment | Redirect URI to register on Google | JavaScript origin |
|---|---|---|
| Local | `http://127.0.0.1:54321/auth/v1/callback` | `http://127.0.0.1:3000` |
| Production | `https://<project-ref>.supabase.co/auth/v1/callback` | Vercel domain |

Whether to use **one** Google OAuth client with both URIs or **two separate clients** (one per environment) is a judgment call. This repo treats them as separate: local credentials live in the **root `.env`** (gitignored), production credentials live in Terraform variables. The boundary keeps a leaked local secret from impacting production. See `docs/deployment.md` section "Manual prerequisites" → Google OAuth client for the production client.

### Env file layout (single source of truth)

Three env files exist for local development; **only the root `.env` is hand-edited**. The rest are generated by `make sync-env` (which runs `playbooks/setup.yml --tags sync-env`) and bear a marker line at the top:

```
# managed-by: sync-env (delete this line to opt out of regeneration)
```

| File | Owner | What it holds |
|---|---|---|
| **`./.env`** | **You** (Google OAuth credentials) | `SUPABASE_AUTH_EXTERNAL_GOOGLE_CLIENT_ID/SECRET`. Auto-exported into the shell by `mise.toml` so `supabase start` resolves the `env()` placeholders in `supabase/config.toml`. |
| `frontend/.env.local` | `make sync-env` | `NEXT_PUBLIC_SUPABASE_URL` + `NEXT_PUBLIC_SUPABASE_ANON_KEY` derived from `supabase status -o env`. Regenerated on every sync while the marker is present. |
| `backend/.env.local` | `make sync-env` (seeded once) | JWKS / JWT audience / issuer / pool tuning. Seeded from `backend/.env.example`; not regenerated after that. |

Removing the marker line from a managed file marks it as user-owned; subsequent `make sync-env` runs will skip it and warn that it may be stale.

Sync logic lives in `playbooks/setup.yml` as declarative Ansible tasks. To wire a new derived env var, add one row to the `env_mappings` list (`<src key in supabase status>:<target key>:<target file>`).

### First-time setup

1. The Supabase CLI is already installed by `mise install` from the root `.tool-versions`. No separate step is needed.
2. Create an OAuth 2.0 client ID in Google Cloud Console (Application type: **Web application**). Use `127.0.0.1`, not `localhost` — Google validates these as distinct origins:
   - Add `http://127.0.0.1:54321/auth/v1/callback` to **Authorized redirect URIs**.
   - Add `http://127.0.0.1:3000` to **Authorized JavaScript origins**.
3. Run `make setup` from the repository root. The first run pulls Docker images and takes a few minutes. Internally this calls `supabase start`, which prints an `Authentication Keys` block with **`Publishable`** (formerly `anon key` — safe to bundle into the client; RLS gates real access) and **`Secret`** (formerly `service_role key` — server-only, bypasses RLS). `make sync-env` then writes the Publishable value into `frontend/.env.local` automatically; you never have to copy it by hand.
4. After setup finishes, edit **`./.env`** (created from `.env.example` if missing) and replace the placeholders with the Google OAuth client values from step 2:
   ```
   SUPABASE_AUTH_EXTERNAL_GOOGLE_CLIENT_ID=<Google OAuth client ID>
   SUPABASE_AUTH_EXTERNAL_GOOGLE_SECRET=<Google OAuth secret>
   ```
   Then run `make supabase-restart` so the new credentials are picked up by the auth server. (Editing values in this file is the **only** manual env step.)
5. Sign in via `http://127.0.0.1:3000/login` once the frontend is running (`make dev-frontend`). The `/profile` page exercises the full sign-in path end to end.
6. Supabase Studio: http://127.0.0.1:54323

### Day-to-day

| Task | Command |
|---|---|
| Start Supabase | `supabase start` |
| Stop Supabase | `supabase stop` |
| Reset DB | `supabase db reset` |
| Check status | `supabase status` |

### Backend JWT verification (local)

To run backend JWT verification locally, run `supabase status` to confirm the JWKS URL (typically `http://127.0.0.1:54321/auth/v1/.well-known/jwks.json`) and copy it into `backend/.env.local`. The defaults in `backend/.env.example` should already match. The three required variables are:

```
SUPABASE_JWKS_URL=http://127.0.0.1:54321/auth/v1/.well-known/jwks.json
SUPABASE_JWT_AUDIENCE=authenticated
SUPABASE_JWT_ISSUER=http://127.0.0.1:54321/auth/v1
```

The backend fails to start if any of these is missing — check `supabase status` output if startup fails with a config error.

### Gotchas

- **`127.0.0.1` only, never `localhost`**: Google OAuth treats them as distinct hosts. Access the app via `127.0.0.1:3000` so the origin matches what was registered with Google and the `supabase start` output.
- **Secrets stay in `.env.local`**: never paste them into `supabase/config.toml`. The toml only contains `env()` placeholders.

For the production setup of the same Google sign-in path (Supabase project, Vercel, Render, Terraform-managed Supabase Auth settings), see `docs/deployment.md`.

## Git tooling

### Extracting a single file's hunk from a mixed-purpose commit

When a commit touches multiple concerns (e.g. a backend fix and a frontend
feature) and you need only one file's changes on a different branch, use:

```bash
git show <sha> -- path/to/file | git apply
```

`git show <sha> -- <path>` outputs the patch for that file only; piping it to
`git apply` applies just that hunk without touching the rest of the commit.
Useful when rewinding a feature branch but keeping an unrelated fix that landed
in the same commit.
