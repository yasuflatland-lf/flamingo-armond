# Dev setup

## Tools

- mise (`curl https://mise.run | sh`) — manages Go (`backend/.tool-versions`), Node + pnpm + Supabase CLI (`./.tool-versions`), and dev runtime tooling (`./mise.toml`: Rust + mprocs).
- Supabase CLI — local Postgres / Auth emulation. Pinned to a specific version in `./.tool-versions` (do not switch back to `supabase latest`: the CLI breaks `supabase/config.toml` across major upgrades, so the version that everyone runs must be exact).

## First-time setup

```bash
# Install Go 1.26.3 (backend/.tool-versions), Node 24.x + pnpm 11.1.3 + Supabase CLI (./.tool-versions).
mise install

# Install workspace deps. The frontend workspace is populated with a Next.js 16 App Router scaffold (see `frontend/CLAUDE.md`).
pnpm install
```

Verify:

```bash
node --version        # v24.x.y
pnpm --version        # 11.1.3  (resolved by mise from .tool-versions)
which pnpm            # ~/.local/share/mise/shims/pnpm
```

`which pnpm` returning `~/.local/share/mise/shims/pnpm` is the expected mise shim. A `/usr/local/bin/pnpm` or `/opt/homebrew/bin/pnpm` path indicates a global install that will shadow the mise shim — uninstall it (`npm uninstall -g pnpm` / `brew uninstall pnpm`) before running `pnpm` again.

## Day-to-day

| Task | Command |
|---|---|
| Run backend + frontend together | `make dev` |
| Run backend (alone) | `make dev-backend` or `cd backend && go run ./cmd/server` |
| Run frontend (alone) | `make dev-frontend` |
| Regenerate GraphQL code | `make codegen` |
| Regenerate dictionary parser (goyacc) | `make codegen-yacc` |
| Run all tests | `make test` |

`make dev` launches `preflight`, backend, and frontend together inside an `mprocs` TUI. The `preflight` panel (top of the proc list) reports whether Supabase is reachable; if all three panels go red, check `preflight` first for the cause. Arrow keys switch panels, `r` restarts one, `x` stops one. Supabase must already be running — `make dev` does not start it; run `make supabase-start` first (or `make setup` for full bring-up). If you prefer separate terminals, `make dev-backend` / `make dev-frontend` still work as before.

## Policy on generated files

The two GraphQL codegen outputs are **gitignored**; the goyacc parser output is **committed**. The split follows determinism, not symmetry:

| Tool | Input | Output | Tracked? | Regeneration command |
|---|---|---|---|---|
| gqlgen | `schema/*.graphql`, `backend/gqlgen.yml`, `backend/go.mod` (`tool` directive) | `backend/graph/generated/`, `backend/graph/model/models_gen.go` | gitignored | `cd backend && go tool gqlgen generate` |
| graphql-codegen | `schema/*.graphql`, `frontend/codegen.ts`, `frontend/src/**/*.{ts,tsx}` | `frontend/src/generated/` | gitignored | `pnpm --filter frontend codegen` |
| Next.js | (n/a — emitted by `next dev` / `next build`) | `frontend/next-env.d.ts` | gitignored | `pnpm --filter frontend dev` or `... build` |
| goyacc | `backend/internal/textdic/grammar.y` | `backend/internal/textdic/parser.go` | **committed** | `make codegen-yacc` |

For gqlgen / graphql-codegen, determinism relies on pinned tool versions (in `go.mod` and `package.json`) plus the committed schema. CI runs the backend regeneration before `go vet` and `go test` (`.github/workflows/backend.yml`). Frontend CI runs `pnpm --filter frontend codegen` before Biome check / typecheck / build (`.github/workflows/frontend.yml`), mirroring the backend contract. No `git diff --exit-code` step is needed because the outputs are not tracked.

Rationale: keeps PR diffs to hand-written code only and removes the merge-conflict churn that committing thousand-line generated files causes. Applied symmetrically to both stacks for consistency.

### `next-env.d.ts` is gitignored

Next.js 16 rewrites the file's single `import` statement based on which command was just run:

- After `next dev` (Turbopack default in Next 16): `import "./.next/dev/types/routes.d.ts";`
- After `next build`: `import "./.next/types/routes.d.ts";`

The toggle is intentional — it is the visible side-effect of the `experimental.isolatedDevBuild` feature (default `true` in Next.js 16) which keeps a running dev server's type cache from being clobbered by a concurrent `next build` (e.g. CI or AI agents validating the project alongside development). Upstream issues [vercel/next.js#85738](https://github.com/vercel/next.js/issues/85738) and [#86001](https://github.com/vercel/next.js/issues/86001) were both closed as working-as-intended, and the official [Next.js TypeScript docs](https://nextjs.org/docs/app/api-reference/config/typescript) now recommend adding `next-env.d.ts` to `.gitignore` (a reversal of the older "do not edit, do commit" guidance still embedded in the file's own comment).

Frontend CI regenerates the file (and the matching `.next/types/routes.d.ts` it imports) via [`next typegen`](https://nextjs.org/docs/app/api-reference/cli/next) — introduced in Next.js 15.5, dedicated to type emission, and faster than running a full `next build` solely to refresh types. The step runs before Biome / typecheck / build in `.github/workflows/frontend.yml`, and a follow-up `git ls-files --error-unmatch` guard fails CI if anyone re-stages the file with `git add -f`.

Do **not** reach for `experimental.isolatedDevBuild: false` to "fix" the toggle: it silences the churn but opts out of the dev/build isolation that Next.js 16 added on purpose, which matters in this monorepo (CI builds, dev server, and agents may run concurrently against the same checkout).

`goyacc` (added via `go get -tool golang.org/x/tools/cmd/goyacc`, which appends to the `tool ( ... )` block in `go.mod`) is the exception: the generated `parser.go` is committed because the goyacc emitter is not byte-for-byte stable across Go and tool versions, and the test suite — not CI regeneration — is the contract that catches grammar drift. Edit `grammar.y` only; never hand-edit `parser.go`. After regenerating with `make codegen-yacc`, commit both files together.

> **Note**: `backend/graph/resolver/*.resolvers.go` are resolver stubs, not generated output. They are **committed** and CI verifies they are up-to-date via `git diff --exit-code -- graph/resolver/*.resolvers.go` in `backend.yml`. This is orthogonal to the "generated files are ignored" policy above.

## `.tool-versions` hierarchy (mise)

mise resolves tool config hierarchically. Three scopes coexist without conflict:

| Scope | File | Tools |
|---|---|---|
| Backend | `backend/.tool-versions` | Go |
| Repo root (frontend + dev) | `./.tool-versions` | Node, pnpm, Supabase CLI, Python + Ansible |
| Repo root (dev runtime) | `./mise.toml` | Rust + mprocs (paired with `[env]`) |

Backend CI sets `working_directory: backend` and sees only Go. Frontend CI runs from the repo root — NOT `working_directory: frontend` — because Node and pnpm are declared in the root `.tool-versions`.

## `mise` exports `.env` into the shell

`mise.toml` declares `_.file = [".env"]`, so activating mise (via `mise install` or any `mise exec` invocation) exports every key in root `.env` into the process environment. Make targets that depend on `mise install` therefore inherit a fully populated `.env` automatically — including `ansible-playbook` invocations, where `lookup('env', 'KEY')` reads the same values without any `vars_files` directive.

Practical consequence: when adding new required env vars consumed by a playbook (e.g. `NOTION_TOKEN`), set them in root `.env` and the playbook will see them on the next `make` invocation. No additional dotenv loader is needed.

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

Whether to use **one** Google OAuth client with both URIs or **two separate clients** (one per environment) is a judgment call. This repo treats them as separate: local credentials live in the **root `.env`** (gitignored), production credentials are configured directly in the production Supabase Auth settings. The boundary keeps a leaked local secret from impacting production. See `docs/deployment.md` section "Manual prerequisites" → Google OAuth client for the production client.

### Env file layout (single source of truth)

Three env files exist for local development; **only the root `.env` is hand-edited**. The rest are generated by `make sync-env` (which runs `playbooks/setup.yml --tags sync-env`) and bear a marker line at the top:

```
# managed-by: sync-env (delete this line to opt out of regeneration)
```

| File | Owner | What it holds |
|---|---|---|
| **`./.env`** | **You** (Google OAuth credentials) | `SUPABASE_AUTH_EXTERNAL_GOOGLE_CLIENT_ID/SECRET`. Auto-exported into the shell by `mise.toml` so `supabase start` resolves the `env()` placeholders in `supabase/config.toml`. |
| `frontend/.env.local` | `make sync-env` | `NEXT_PUBLIC_SUPABASE_URL` + `NEXT_PUBLIC_SUPABASE_ANON_KEY` derived from `supabase status -o env`. Regenerated on every sync while the marker is present. Loaded natively by Next.js from its own CWD. |
| `backend/.env.local` | `make sync-env` (seeded once) | JWKS / JWT audience / issuer / pool tuning. Seeded from `backend/.env.example`; not regenerated after that **except** for `PING_TOKEN`, which `make sync-env` auto-generates (64 hex chars via `openssl rand -hex 32`) when empty or missing — subsequent runs are a no-op. Loaded by `cmd/server/main.go` itself via `godotenv` — **not** auto-exported into the shell, so backend env names (`PORT`, `SUPABASE_*`) cannot leak into sibling processes such as `next dev`. |

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

### First admin (local Supabase)

Two complementary paths grant the `admin` role to any email listed in `SUPER_USER_EMAILS`. Both are sanctioned; pick whichever fits the situation.

- **Middleware auto-promotion** (zero-touch). On any authenticated `/query` request, the post-auth middleware grants `admin` to a matching user **when the JWT carries `email_verified=true`**. Subsequent requests short-circuit on the cached role.
- **`make sync-env` seed task** (idempotent INSERT). The setup playbook runs `INSERT INTO public.user_roles ... ON CONFLICT DO NOTHING` for every email in `SUPER_USER_EMAILS` that already has an `auth.users` row. Use this when the middleware path has not fired — typically because the current browser session's JWT predates the email confirmation, or the user signed up but has not yet issued a GraphQL request.

Both paths require `public.roles` to exist, which means the backend must have run migrations at least once (i.e. `make dev` / `make dev-backend` started successfully).

#### Steps

1. Open `backend/.env.local` and add (or uncomment) the line:
   ```
   SUPER_USER_EMAILS=you@example.com
   ```
   Comma-separate multiple emails. Whitespace and case are normalised by the backend.
2. Restart the backend (`make dev-backend` or the `backend` panel in `make dev`). On startup, look for the JSON log line confirming the bootstrap is armed:
   ```
   {"level":"INFO","msg":"super-user bootstrap enabled","email_count":1}
   ```
   If `email_count` is `0`, your edit did not take effect — re-check the file path and restart.
3. Sign in at `http://127.0.0.1:3000/login` with one of the listed accounts. The Admin pill appears in the global header and `/admin/*` routes become reachable.

**If the Admin pill does not appear** after sign-in, the middleware did not promote — almost always because the active session's JWT was issued before email confirmation (so `email_verified` is `false` in the token). Run:

```bash
make sync-env
```

The playbook's seed-admin task INSERTs the `(user_id, admin_role_id)` row directly. After that, sign out and sign back in (or wait for a token refresh) so the next request carries the new role. The task is idempotent — running it when the user is already admin is a no-op.

**After `make db-reset`:** `public.user_roles` is wiped. The middleware re-promotes on the next authenticated request from a JWT with `email_verified=true`; otherwise run `make sync-env` once the backend has been restarted (migrations must have re-created `public.roles`).

**Manual SQL fallback** (only if `make sync-env` is unavailable and you are debugging — e.g. the Authorization-header propagation gap, or you need an admin without a successful login round-trip):

```bash
psql "$(supabase status -o env | grep '^DB_URL=' | cut -d= -f2- | tr -d '"')" <<'SQL'
INSERT INTO public.user_roles (user_id, role_id)
SELECT u.id, r.id
  FROM auth.users u, public.roles r
  WHERE u.email = 'you@example.com' AND r.name = 'admin'
ON CONFLICT DO NOTHING;
SQL
```

The `make seed-admin EMAIL=you@example.com` target wraps this SQL — see [`backend-auth.md` § "Bootstrap admin via `SUPER_USER_EMAILS`"](backend-auth.md#bootstrap-admin-via-super_user_emails).

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
- **`backend/.env.example` drift**: `make setup` / `make sync-env` seeds `backend/.env.local` only when that file is entirely absent (`env_ownership[item.target] == 'missing'` in `playbooks/setup.yml`); subsequent runs leave the existing file untouched. When `backend/.env.example` gains a new key (e.g. `SUPER_USER_EMAILS`), operators who already have a seeded `backend/.env.local` will not receive the new line automatically. After pulling from main, check for newly added keys with `git diff main -- backend/.env.example` and add any missing blocks to `backend/.env.local` by hand. See [§ "First admin (local Supabase)"](#first-admin-local-supabase) for the bootstrap path that depends on this key.

For the production setup of the same Google sign-in path (Supabase project, Vercel, Render, Supabase Auth settings), see `docs/deployment.md`.

## Git tooling

### Working in a git worktree

`backend/graph/generated/` (gqlgen output) and `frontend/src/generated/` (graphql-codegen output) are both gitignored. CI always runs codegen before build and typecheck, so missing generated files are invisible on CI. A `git worktree add` of any branch starts without these directories, and the first build or test run fails immediately:

- `go build ./...` — "no required module provides package backend/graph/generated"
- `tsc --noEmit` / `pnpm vitest` — "Cannot find module '@/generated/...'"

A fresh worktree also starts with the root `mise.toml` untrusted, so any `mise`-mediated command (including the `pnpm` shim) refuses to load its `[env]` block until you trust it once from the worktree root:

```bash
mise trust          # required once per new worktree before any pnpm / mise exec command
```

Run codegen once after creating or switching to a worktree:

```bash
# Backend — from the worktree root
cd backend && go tool gqlgen generate

# Frontend — from the worktree root
pnpm --filter frontend codegen
```

`make codegen` at the repo root runs both steps together. The same applies whenever the `schema/*.graphql` source changes on the active branch, since the generated output is not updated by `git checkout`.

See [§ "Policy on generated files"](#policy-on-generated-files) for the full rationale behind gitignoring codegen output, and [`docs/backend/library-gotchas/go-arch-lint-testdata-and-generated-exclusions.md`](backend/library-gotchas/go-arch-lint-testdata-and-generated-exclusions.md) for the same requirement applied to `go-arch-lint`.

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
