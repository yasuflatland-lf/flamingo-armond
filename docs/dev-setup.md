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

ローカル開発は `supabase start` で完結する（本番 Supabase プロジェクト不要、PR9 で接続）。

### 初回セットアップ

1. Supabase CLI をインストール（`brew install supabase/tap/supabase` または `mise use supabase@latest`）。
2. Google Cloud Console で OAuth 2.0 client ID を作成:
   - Authorized redirect URIs に `http://127.0.0.1:54321/auth/v1/callback` を追加。
   - Authorized JavaScript origins に `http://127.0.0.1:3000` を追加。
3. リポジトリルートで `supabase start` を実行。初回のみ Docker イメージ取得で数分かかる。出力に anon key / service role key が表示される。
4. `frontend/.env.local` に以下を追記（`supabase start` の出力から `anon key` をコピー）:
   ```
   NEXT_PUBLIC_SUPABASE_URL=http://127.0.0.1:54321
   NEXT_PUBLIC_SUPABASE_ANON_KEY=<supabase start の出力から anon key>
   SUPABASE_AUTH_EXTERNAL_GOOGLE_CLIENT_ID=<Google OAuth client ID>
   SUPABASE_AUTH_EXTERNAL_GOOGLE_SECRET=<Google OAuth secret>
   ```
5. Supabase Studio: http://127.0.0.1:54323

### 日常運用

| タスク | コマンド |
|---|---|
| Supabase 起動 | `supabase start` |
| Supabase 停止 | `supabase stop` |
| DB リセット | `supabase db reset` |
| ステータス確認 | `supabase status` |

### Backend JWT verification (local)

To run backend JWT verification locally, run `supabase status` to confirm the JWKS URL (typically `http://127.0.0.1:54321/auth/v1/.well-known/jwks.json`) and copy it into `backend/.env.local`. The defaults in `backend/.env.example` should already match. The three required variables are:

```
SUPABASE_JWKS_URL=http://127.0.0.1:54321/auth/v1/.well-known/jwks.json
SUPABASE_JWT_AUDIENCE=authenticated
SUPABASE_JWT_ISSUER=http://127.0.0.1:54321/auth/v1
```

The backend fails to start if any of these is missing — check `supabase status` output if startup fails with a config error.

### Gotchas

- **`localhost` ではなく `127.0.0.1` を使う**: Google OAuth の redirect URI 検証は `localhost` と `127.0.0.1` を別ホスト扱いする。`supabase start` のデフォルト出力に合わせて `127.0.0.1:3000` でアクセス。
- **シークレットは `.env.local` (gitignored)**: `supabase/config.toml` は `env()` プレースホルダで参照するだけで、実値はコミットしない。
