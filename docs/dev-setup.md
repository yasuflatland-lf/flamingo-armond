# Dev setup

## Tools

- mise (`curl https://mise.run | sh`) — manages Go (backend/.tool-versions) and Node (./.tool-versions).
- Corepack — bundled with Node. Activates the pnpm version pinned in root package.json. **Required**.
- Supabase CLI — local Postgres / Auth emulation. Used from PR6 onward.

## First-time setup

```bash
# 1. Install Go 1.26.2 (from backend/.tool-versions) and Node 22.x (from ./.tool-versions).
mise install

# 2. Enable Corepack so that the pnpm version in package.json is honored.
#    If you previously installed pnpm globally (npm i -g pnpm / brew install pnpm),
#    uninstall it first — a PATH-level global pnpm shadows the Corepack shim and
#    silently breaks version pinning.
corepack enable

# 3. Install workspace deps. The frontend workspace is empty until PR3 — that is fine.
pnpm install
```

Verify:

```bash
node --version        # v22.x.y
pnpm --version        # 9.15.0  (resolved via Corepack from packageManager field)
which pnpm            # should NOT point to a global install (npm i -g / brew)
```

`which pnpm` may return `~/.local/share/mise/shims/pnpm` on a mise-managed machine — that is a mise shim delegating to the Corepack-managed binary, not a global install, and is not drift. The resolved version (`pnpm --version`) is what matters.

## Day-to-day

| Task | Command |
|---|---|
| Run backend | `make dev-backend` or `cd backend && go run ./cmd/server` |
| Run frontend | `make dev-frontend` (PR3+) |
| Regenerate GraphQL code | `make codegen` (PR2 / PR5+) |
| Run all tests | `make test` |

## Policy on generated files

`backend/graph/generated/` (gqlgen output) and `frontend/src/generated/` (graphql-codegen output) are **committed**. CI verifies that running codegen produces no diff via `git diff --exit-code` (added in PR2 / PR5). Not gitignoring generated files is an intentional design choice.

## `.tool-versions` hierarchy (mise)

mise resolves `.tool-versions` files hierarchically: `backend/.tool-versions` (Go) and `./.tool-versions` (Node) are both honored without conflict. Backend CI sets `working_directory: backend` and sees only the Go version. When PR4 adds a frontend workflow that needs Node, that workflow must run from the repo root (`.`) — NOT `working_directory: frontend` — because the Node version is declared in the root `.tool-versions`.

## Why Corepack, not global pnpm

The `packageManager` field in root `package.json` is the single source of truth for the pnpm version. Corepack reads it and downloads the exact version on demand, so every contributor and every CI runner uses the same pnpm — no drift, no "works on my machine".

Do **not** install pnpm via `npm i -g pnpm` or `brew install pnpm`. Those paths compete with the Corepack shim on PATH, and whichever wins is timing-dependent.

## Supabase CLI (PR6+)

TBD — PR6 で `supabase start` 手順を追記。
