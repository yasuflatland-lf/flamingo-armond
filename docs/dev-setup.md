# Dev setup

## Tools

- mise (`curl https://mise.run | sh`) — manages Go (backend/.tool-versions) and Node (./.tool-versions).
- Corepack — bundled with Node. Activates the pnpm version pinned in root package.json. **Required**.
- Supabase CLI — local Postgres / Auth emulation. Used once Supabase integration lands.

## First-time setup

```bash
# 1. Install Go 1.26.2 (from backend/.tool-versions) and Node 24.x (from ./.tool-versions).
mise install

# 2. Enable Corepack so that the pnpm version in package.json is honored.
#    If you previously installed pnpm globally (npm i -g pnpm / brew install pnpm),
#    uninstall it first — a PATH-level global pnpm shadows the Corepack shim and
#    silently breaks version pinning.
corepack enable

# 3. Install workspace deps. The frontend workspace is populated with a Next.js 16 App Router scaffold (see `docs/frontend.md`).
pnpm install
```

Verify:

```bash
node --version        # v24.x.y
pnpm --version        # 9.15.0  (resolved via Corepack from packageManager field)
which pnpm            # should NOT point to a global install (npm i -g / brew)
```

`which pnpm` may return `~/.local/share/mise/shims/pnpm` on a mise-managed machine — that is a mise shim delegating to the Corepack-managed binary, not a global install, and is not drift. The resolved version (`pnpm --version`) is what matters.

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

mise resolves `.tool-versions` files hierarchically: `backend/.tool-versions` (Go) and `./.tool-versions` (Node) are both honored without conflict. Backend CI sets `working_directory: backend` and sees only the Go version. When a frontend workflow that needs Node is added, that workflow must run from the repo root (`.`) — NOT `working_directory: frontend` — because the Node version is declared in the root `.tool-versions`.

## Why Corepack, not global pnpm

The `packageManager` field in root `package.json` is the single source of truth for the pnpm version. Corepack reads it and downloads the exact version on demand, so every contributor and every CI runner uses the same pnpm — no drift, no "works on my machine".

Do **not** install pnpm via `npm i -g pnpm` or `brew install pnpm`. Those paths compete with the Corepack shim on PATH, and whichever wins is timing-dependent.

## Supabase CLI

Local development is self-contained behind `supabase start` (no production Supabase project required; the production project is wired up separately).

### Auth flow (read this first)

Google sign-in is brokered by Supabase Auth. The browser hits Google → Google redirects to **Supabase's** `/auth/v1/callback` → Supabase exchanges the code and redirects to the Next.js app's `/auth/callback`. The redirect URI registered with Google is therefore the Supabase host, not the Next.js host:

| Environment | Redirect URI to register on Google | JavaScript origin |
|---|---|---|
| Local | `http://127.0.0.1:54321/auth/v1/callback` | `http://127.0.0.1:3000` |
| Production | `https://<project-ref>.supabase.co/auth/v1/callback` | Vercel domain |

Whether to use **one** Google OAuth client with both URIs or **two separate clients** (one per environment) is a judgment call. This repo treats them as separate: local credentials live in `frontend/.env.local`, production credentials live in Terraform variables. The boundary keeps a leaked local secret from impacting production. See `docs/deployment.md` § 8 for the production client.

### First-time setup

1. Install the Supabase CLI (`brew install supabase/tap/supabase` or `mise use supabase@latest`).
2. Create an OAuth 2.0 client ID in Google Cloud Console (Application type: **Web application**). Use `127.0.0.1`, not `localhost` — Google validates these as distinct origins:
   - Add `http://127.0.0.1:54321/auth/v1/callback` to **Authorized redirect URIs**.
   - Add `http://127.0.0.1:3000` to **Authorized JavaScript origins**.
3. Run `supabase start` from the repository root. The first run pulls Docker images and takes a few minutes. The output prints the anon key and service role key.
4. Append the following to `frontend/.env.local` (copy `anon key` from the `supabase start` output). `supabase/config.toml` references the Google credentials via `env()` placeholders, so the secrets stay in `.env.local` (gitignored) and never enter the repo:
   ```
   NEXT_PUBLIC_SUPABASE_URL=http://127.0.0.1:54321
   NEXT_PUBLIC_SUPABASE_ANON_KEY=<anon key from supabase start output>
   SUPABASE_AUTH_EXTERNAL_GOOGLE_CLIENT_ID=<Google OAuth client ID>
   SUPABASE_AUTH_EXTERNAL_GOOGLE_SECRET=<Google OAuth secret>
   ```
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
