# CI / CD notes

Operational decisions around GitHub Actions and external services that are not obvious from the workflow YAML alone.

## Workflow scope and concurrency

Three independent workflows: `.github/workflows/backend.yml`, `.github/workflows/frontend.yml`, and `.github/workflows/e2e.yml`.

- **Triggers are `paths:`-scoped** per workflow — backend to `backend/**` + workflow file; frontend to `frontend/**` + `schema/**` + the root pnpm/workspace/tool-version manifests + the frontend workflow file. When adding a third service, **add its own workflow** — do not broaden an existing one. Mixing scopes breaks CI granularity and responsibility.
- **`concurrency` groups are per-workflow** (`backend-${{ github.ref }}`, `frontend-${{ github.ref }}`, `e2e-${{ github.ref }}`) with `cancel-in-progress: true` — rapid pushes on the same ref supersede in-flight runs per service (important for feature-branch iteration). The three workflows do not cancel each other, except that E2E uses a PR-only `cancel-in-progress` split; see § "E2E workflow".
- **General rule for `cancel-in-progress`:** jobs whose effects are confined to the runner (lint, test, build artifacts in transit) are safe for `cancel-in-progress: true`. Jobs that have already committed external state (deploys, releases, side-effecting API calls) must override with `cancel-in-progress: false` to avoid leaving external systems in an indeterminate state. The backend `deploy` job in `backend.yml` follows this rule; see § "Deploy gating".

## E2E workflow

`.github/workflows/e2e.yml` runs Playwright against a local Supabase-backed stack. It triggers on frontend/schema/Supabase-config changes, **backend changes**, pushes to `main`, and a nightly `0 4 * * *` UTC cron.

The workflow uses split concurrency: pull-request runs use `cancel-in-progress: true`, while push and cron runs do not. Post-merge and scheduled runs must produce a definitive pass/fail signal for `main` and the nightly cadence; cancellation by a later push would mask regressions on the integration boundary. PR runs do not have this constraint and prefer `cancel-in-progress: true` to free the queue for newer pushes.

The job exports Supabase local keys from `supabase status -o env`; no repository secret is required for the service-role key. On failure it uploads the Playwright report, raw test results, and backend log with 14-day retention.

### Include backend changes in E2E triggers

The E2E workflow must trigger on backend file changes in addition to frontend and schema changes. A backend-only change (new endpoint, changed query shape, migration) can silently break the full integration while individual backend unit tests keep passing. Relying on the nightly cron to catch this introduces up to a 24-hour detection window. Add `backend/**` to the workflow's `paths:` filter so any backend push also queues an E2E run.

### Start only the Supabase containers the suite uses

`supabase start` boots every service enabled in `supabase/config.toml`, and on a cold runner the Docker image pulls dominate the E2E job's wall-clock. The suite needs four of them: **postgres** (migrations, seed data, and the `supabase db query --local` role seeding), **kong** (the `API_URL` gateway every Supabase call goes through), **gotrue** (`auth.admin.*` and `signInWithPassword` in `frontend/e2e/_auth.ts`, plus the app's own `auth.getUser()`), and **postgrest** (every `adminClient.from(...)`). The rest are handed to `supabase start -x`.

**`realtime` and `storage-api` must stay out of the exclusion list even though no spec uses them.** When the database volume does not exist — always true on a fresh runner — the CLI runs a one-shot schema-init job built from the realtime and storage images. Those jobs are gated on `config.toml`'s `[realtime] enabled` / `[storage] enabled` flags, **not** on `-x`, so excluding the containers removes the images from the parallel pre-pull while still requiring them moments later. The pull then happens serially, mid-startup, on the critical path. Excluding them is a pessimisation, not a saving.

Keep the exclusion in the workflow only. Flipping `enabled = false` in `supabase/config.toml` would strip Studio, Mailpit and Realtime from every developer's local stack too, which is not the intent — and for realtime/storage it would also change what schema the init jobs install.

`supabase status -o env` still reports `API_URL`, `ANON_KEY`, `SERVICE_ROLE_KEY` and `DB_URL` with the exclusions applied, and `DB_URL` is built from `[db] port` (54322) rather than from a pooler port, so `supavisor` can be excluded without affecting the `Export Supabase local env` step. `supabase db query --local` connects to Postgres directly and does not need `postgres-meta`.

If the stack ever fails to come up, narrow the list rather than reverting it, in this order: `mailpit` (gotrue is configured with an SMTP host pointing at the excluded container; harmless while no spec sends mail, since users are seeded with `email_confirm: true`), then `postgres-meta`, then `supavisor`. Each removal costs back only that image's pull time, so a partial exclusion still banks most of the win.

### Pin the local Supabase identity when the Next build runs early

The E2E job overlaps its slow setup work with `supabase start` using `background: true` steps that reconverge at `wait-all`. The Next.js production build belongs in that shadow too, but it cannot simply be moved there: `frontend/src/env.ts` declares `NEXT_PUBLIC_SUPABASE_URL` and `NEXT_PUBLIC_SUPABASE_ANON_KEY` as client vars, and `next build` **inlines** them into the served bundle. Unlike the frontend workflow's `lint-test-build` job, which builds with dummy values it never serves, this bundle is handed to a real browser that authenticates a seeded session against GoTrue with that key. Reading the values from `supabase status` is exactly what would keep the build on the critical path.

They are therefore pinned as job-level constants (`E2E_LOCAL_API_URL`, `E2E_LOCAL_ANON_KEY`). Both are deterministic: the API URL comes from `supabase/config.toml`'s `[api] port`, and because that file declares neither `auth.jwt_secret` nor signing keys, the CLI signs the anon JWT with its built-in local secret and a fixed expiry constant rather than a wall-clock-derived one. The pin is valid for the CLI version in `mise.toml`; a CLI bump is the thing most likely to invalidate it.

`Export Supabase local env` re-reads the stack's real values and fails the job if either constant disagrees, printing each key's prefix and length rather than the key itself. Without that assertion a drifted key would surface as all 14 specs failing at the auth gate, with nothing pointing at the build step that baked in the wrong value.

The build is safe this early because it touches neither service: `graphql-codegen` reads `../schema/*.graphql` from disk, and no route fetches at build time.

### Backend migration step ordering

When the Go backend applies migrations via `golang-migrate` on startup (or via an explicit migrate step), tables created in those migrations — for example a `roles` table — do not exist immediately after `supabase start`. Any CI step that seeds canonical rows into backend-managed tables (e.g. `INSERT INTO roles ...`) must be placed **after** the backend has started and migrations have completed, not immediately after `supabase start`. Use a health-check or wait-on step to confirm the backend is ready before running seed SQL.

### Build the binary before starting the backend

`go run ./cmd/server` recompiles from source on every invocation. On a cold CI cache this adds 30–60 seconds of compile time inside the background-start step, and the wait-on timer starts counting before compilation finishes. Build the binary in a separate step (`go build -o /tmp/server ./cmd/server`) and then start the pre-built binary in the background-start step. This makes compile time visible as its own step duration and keeps the wait-on timer honest.

### Inline log tail on wait-on failure

When a `wait-on` step times out, the root cause is almost always in the backend or Supabase logs. The `if: failure()` artifact upload at job end uploads the log file, but reviewers must navigate to the artifact tab to see it — it is not inline in the step output. Add a conditional log-tail step directly after the wait-on:

```yaml
- name: Dump backend log on wait failure
  if: failure()
  run: |
    echo "::group::backend log (last 200 lines)"
    tail -n 200 /tmp/flamingo-backend.log || true
    echo "::endgroup::"
```

This makes the failure reason visible inline in the Actions UI without opening a separate artifact.

### Shell safety: `set -euo pipefail` and non-empty env validation

Multi-command `run:` blocks in `e2e.yml` must open with `set -euo pipefail` so an intermediate command failure does not silently continue to the next line. When exporting environment variables derived from `supabase status -o env` output, validate that the value is non-empty before writing to `$GITHUB_ENV` (e.g. `${EXTRACTED_VALUE:?variable is empty}`). An empty value written to `$GITHUB_ENV` propagates as a blank string to downstream steps, producing confusing failures far from the extraction site.

### Job-level vs step-level env redundancy

When an env var is declared at the `job:` level and a `step:` under the same job re-declares the identical var, the step-level declaration is noise — the step already inherits the job-level value. Remove step-level re-declarations that duplicate a job-level declaration without changing the value; they add a maintenance surface (two places to update on rename) without any benefit.

## Deploy gating

- The `deploy` job is `needs: test` and `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` — doubly restricted.
- If `RENDER_DEPLOY_HOOK_URL` is missing, the step **explicitly exits 1** rather than silently skipping. Missing secrets are misconfiguration and should fail loudly. **Do not replace this with a silent skip.**
- `RENDER_DEPLOY_HOOK_URL` is registered automatically by `make setup-prod` (Phase 3) via `gh secret set` — see `docs/deployment.md` § "Guided bring-up via `make setup-prod`" for the bring-up flow that wires this secret.
- The Render service settings are configured manually in the Render dashboard: `root_directory = "backend"`, build `./cmd/server` to `main`, `auto_deploy = false` (deploys are push-triggered via the hook, not Render's auto-deploy), `health_check_path = "/health"`.

## Coverage requires `-covermode=atomic`

Coverage collected together with `-race` must use `-covermode=atomic` — the default `count` mode is non-atomic and the race detector flags **the coverage counters themselves** as a data race. `-race` should never be skipped locally or in CI, so the canonical command is:

```
go test -race -covermode=atomic -coverprofile=coverage.out ./...
```

## Codecov upload must not be a silent failure

Using `codecov/codecov-action` with `fail_ci_if_error: false` alone is a **silent failure** — a Codecov outage or misconfigured token leaves CI green while coverage stops being published. That builds a false sense that "CI green = coverage is tracked."

The workflow uses three pieces together so "failure doesn't block, but is always visible":

1. The step has `id: codecov` and `continue-on-error: true`.
2. The Codecov action itself uses `fail_ci_if_error: true` (so its outcome records as failure).
3. A follow-up step gated on `if: steps.codecov.outcome == 'failure'` emits a GitHub `::warning::` annotation.

PR checks stay green, but failures surface as warnings in the Actions UI.

## Double publish coverage artifacts

Coverage goes to **both Codecov and a GHA artifact**:

- Codecov — trend visualization, PR comments.
- GHA artifact (`coverage.out` + `coverage.html`, 14-day retention) — backup for Codecov outages, and a human-readable HTML report via `go tool cover -html`.
- The frontend coverage artifact (`frontend-coverage`, lcov + HTML) follows the same `retention-days: 14` rule.

`retention-days: 14` is tighter than the 90-day default to save storage; extend it if needed.

## pnpm workspace filter exits 0 for missing scripts

`pnpm --filter <workspace> <script>` emits nothing and exits 0 when the target package has no matching script — it is treated as a no-op, not an error (unlike `npm run`). CI steps that rely on this behavior to catch missing setup will silently pass. When adding a frontend workflow, use `--if-present` to make intent explicit, or add a stub script that `exit 1`s if the script must exist.

## GitHub Actions versioning

Actions are pinned to **major tags (`@vN`)**, not SHAs:

- `@vN` auto-follows patch / minor — security fixes land without intervention.
- Major bumps are reviewed manually — breaking changes stay visible.

When skipping majors (e.g. `upload-artifact@v4 → @v7`), verify the breaking changes of each skipped version:

- `actions/upload-artifact@v5+` — implicit merging of same-named artifacts was removed; uploading the same name from a matrix job now errors immediately.
- `actions/checkout@v5+` — runtime Node.js version was bumped (may not run on older self-hosted runners).
- `codecov-action@v6` — switched internals to the Codecov CLI (auth flow changed).

Pin to commit SHAs when you need stronger supply-chain guarantees, at the cost of maintenance burden. The project uses major tags for now.

**In-band npm tool installs follow the same policy.** When a CI step installs a global npm package, pin to a major version (e.g. `npm install --global some-cli@<major>`) for the same reason: security fixes auto-follow at minor/patch level, and a major bump requires an explicit, reviewable diff. When adding any in-band `npm install` for a global tool, apply this principle by default.

## Codegen must run before Vet and Build

`backend/graph/generated/` and `backend/graph/model/models_gen.go` are git-ignored (see `docs/dev-setup.md`). On a fresh CI checkout these files do not exist, so `go vet ./...` — which type-checks the entire module — fails unless the runtime has been regenerated first. Hence the `test` job runs `go tool gqlgen generate` between `Verify modules` and `Vet`, not after. Any future codegen added to the pipeline must land in the same position relative to its consumers.

## govulncheck runs after codegen and gates the merge

The `test` job runs `go tool govulncheck ./...` (step "Scan for known vulnerabilities") after `Build`. govulncheck is pinned as a `go` tool dependency in `backend/go.mod` — the same `tool (...)` directive that pins gqlgen, go-arch-lint, and goyacc — so its version is locked by `go.sum` and installs with `go mod download`. No separate `go install @latest` or marketplace action is used, matching the repo's existing tool-pinning convention.

Two non-obvious points:

- **Positioning.** govulncheck type-checks and builds a call graph over the whole module, so it needs `backend/graph/generated/` and `backend/graph/model/` on disk. Those are gitignored (see § "Codegen must run before Vet and Build"), so the scan must sit **after** the gqlgen generate step — placing it earlier fails to load those packages with `invalid package name: ""`.
- **Reachability gate.** govulncheck reports only vulnerabilities reachable from our code's call graph, not every advisory touching a dependency in `go.sum`. A non-zero exit therefore means an actually-reachable vuln, so the step is a hard merge gate with no `continue-on-error`. Unreachable advisories do not fail CI, which keeps the signal actionable.

## Frontend codegen step positioning

`frontend/src/generated/` is gitignored (see `.gitignore`). On a fresh CI checkout the directory does not exist. The `@/generated` import used in `src/app/page.tsx` and `src/lib/apollo/server.test.ts` must resolve before `TypeScript typecheck`, `Build`, or `Vitest` run. Therefore the workflow places a dedicated `Codegen (graphql-codegen)` step immediately after `Install dependencies` and before `Biome check` / `TypeScript typecheck` / `Build` / `Vitest`.

This mirrors the backend rule (§"Codegen must run before Vet and Build"). `pnpm codegen` takes under 5s on a warm pnpm store, so an independent step adds negligible overhead. A `prebuild` lifecycle hook also runs codegen locally (`pnpm --filter frontend build` triggers it automatically); the two mechanisms coexist because CI runs `TypeScript typecheck` before `Build`, and typecheck alone does not trigger `prebuild`. We deliberately do NOT rely on `git diff --exit-code` for validation — `src/generated/` is gitignored so the diff is always empty; the real signal is `pnpm codegen` exiting 0.

## Shell command paths under `working-directory:` are cwd-relative

This applies to any shell command in a step, not only `git` pathspecs. When a step runs a `grep` or similar command and the path argument was copied from a repo-root perspective (e.g. `backend/internal`), the shell resolves it relative to the step cwd, producing `backend/backend/internal` (nonexistent). `grep` then prints an error to stderr but exits 0 on an empty match — the `if grep ...` condition evaluates false without any visible failure. Always write paths relative to the declared `working-directory`.

**Chained `grep` exit-code masking.** `if grep -X | grep -Y` only checks the exit code of the last `grep`. A missing-path error on the first `grep` is masked — the pipeline returns 0 and the lint step appears to pass. Prefer running each `grep` independently or use `pipefail` (`set -o pipefail`) when piping.

## "No imports of X in Y" CI gates: scope the grep to the import-path string literal

A grep gate that enforces "package X must not be imported under directory Y" must match the **import-path string literal**, not a substring of the package name. The literal form survives both new doc-only `*.go` files in the target tree (e.g. comments referencing the forbidden package) and any future identifier collision (a struct or variable that happens to embed the package name). The substring form produces permanent false positives that must be repeatedly waived.

```bash
# Wrong — matches the package name in any comment, identifier, or doc string.
if grep -rln 'gqlerr' internal/usecase --include='*.go' --exclude='*_test.go'; then ...

# Right — matches only the actual import declaration string literal.
if grep -rln '"backend/internal/gqlerr"' internal/usecase --include='*.go' --exclude='*_test.go'; then ...
```

The same principle applies to any "no use of X under Y" gate that targets a structural Go construct: anchor the regex on the syntactic form (`"` for import paths, `func ... ` for function declarations, etc.) rather than the bare identifier.

The `extensions.code` literal gate (`grep -rnE '"code"\s*:\s*"(...)"'`) in `.github/workflows/backend.yml` is the same pattern applied to a different syntactic shape — it matches the JSON map literal, not the string `"code"` in arbitrary positions. Both gates are line-oriented; multi-line constructions or values assembled via `fmt.Sprintf` slip through. The grep is a fast first defense, not a substitute for code review.

## Git pathspec under `working-directory:` is cwd-relative

The `test` job declares `defaults.run.working-directory: backend`, so every `run:` step starts with cwd in `backend/`. When a step invokes `git diff -- <pathspec>`, the pathspec is resolved relative to the shell cwd, **not** the repository root. Writing `git diff -- backend/graph/resolver/*.resolvers.go` would be interpreted as `backend/backend/graph/...`, which matches nothing; `git diff --exit-code` then returns 0 and the check silently passes regardless of actual drift.

Two safe forms:

- cwd-relative: `git diff --exit-code -- graph/resolver/*.resolvers.go` (current form).
- Repo-root-anchored: `git diff --exit-code -- :/backend/graph/resolver/*.resolvers.go` (the `:/` magic signature).

Do **not** mix the two by keeping the full `backend/...` path when `working-directory` is already `backend/`.

## `curl --retry` does not retry 5xx by default — add `--retry-all-errors`

`curl --retry N` retries only on connection-level failures (timeouts, refused connections). It does **not** retry HTTP 5xx responses by default. A workflow step that uses `curl -fsS --retry 3` will appear to succeed (exit 0) even if the server returns 503 or 504 on every attempt, because `-f` causes curl to exit non-zero only on 4xx/5xx **after** exhausting all retries — and retries only fire when the failure is at the transport layer, not the HTTP layer.

The fix is `--retry-all-errors`, which instructs curl to treat any failure, including HTTP error codes, as a retry trigger:

```bash
curl -fsS --retry 3 --retry-delay 5 --retry-all-errors --max-time 60 "$URL"
```

This is the canonical form used in `.github/workflows/readiness-ping.yml`. Apply it to any future workflow step that must detect transient 5xx responses rather than silently treating them as success.

## Frontend workflow

`.github/workflows/frontend.yml` mirrors the backend workflow's structure — per-service scope, major-tag pinning, per-ref concurrency — but has a different install/verify pipeline because the frontend is a pnpm workspace rooted at the repo root.

### `pnpm install --frozen-lockfile` is the gate

CI always runs `pnpm install --frozen-lockfile` (never plain `pnpm install`). With the `--frozen-lockfile` flag, pnpm refuses to mutate `pnpm-lock.yaml` and exits non-zero if the lockfile and the declared dependencies disagree. This is the only mechanism that catches "I edited `package.json` but forgot to re-run `pnpm install` locally" — without it, CI would silently regenerate the lockfile in-place and the drift would reach main.

For the same reason, the frontend workflow's `paths:` filter includes `pnpm-lock.yaml`, `pnpm-workspace.yaml`, root `package.json`, and `mise.toml` alongside `frontend/**` and `schema/**`. A change to any of those can invalidate the frozen-lockfile invariant, so the workflow must run on those edits even when no file under `frontend/` changed.

### `--if-present` on the test step (revisit when Vitest lands)

Per "pnpm workspace filter exits 0 for missing scripts" above, a missing `test` script in `frontend/package.json` would silently pass with plain `pnpm --filter frontend test`. The current workflow runs `pnpm --filter frontend --if-present test` specifically so the step becomes a documented no-op today and **automatically activates** once the `test` script plus a Vitest config are added — no workflow edit needed at that point. When Vitest lands, do not drop the `--if-present` flag: it stays as a guard against future script renames.

### Node/pnpm provisioning via mise

The workflow uses the same `jdx/mise-action@v4` step that `backend.yml` uses, relying on the repo-root `mise.toml` to pin both Node (`node 24.20.0`) and pnpm (`pnpm 11.25.0`). mise installs both directly, so for local dev and GitHub Actions the pnpm version is pinned by the repo — not by the CI runner's preinstalled toolchain and not by Corepack. The `packageManager` field in root `package.json` is kept aligned for two reasons that are NOT informational: (1) Vercel does not run mise, so it reads `packageManager` to choose which pnpm version to install on its build image, and (2) pnpm 11 itself uses the field as a self-consistency check and refuses to run when the executing binary disagrees with the declared version. Together these keep local, CI, and Vercel pnpm versions in lockstep with a single source of truth.

### Build-time env vars: server and client

`frontend/src/env.ts` uses `@t3-oss/env-nextjs` to Zod-validate **all** declared vars at build time. `next build` fails if any required var is unset — a deliberate failure mode.

CI sets dummy values at the job level for every required var:

| Var | Why needed at build time |
|---|---|
| `BACKEND_URL` | Server var; validated by `@t3-oss/env-nextjs` at build. |
| `NEXT_PUBLIC_SUPABASE_URL` | Client var; `@t3-oss/env-nextjs` validates and **bundles** client vars into the JS bundle at build time — missing = build failure. |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | Same as above. |

No request is made during the build, so dummy values only need to satisfy the Zod schema (e.g. `z.string().url()` requires a URL-shaped string). Do **not** remove any of these: each missing env reintroduces a silent-fail shape the validation was designed to prevent. When a new required var is added to `src/env.ts`, add a corresponding dummy to the workflow's `env:` block.

### Frontend Codecov upload

The frontend coverage pipeline mirrors the backend's three-piece pattern
(see § "Codecov upload must not be a silent failure") and is wired with
`flags: frontend` so Codecov reports backend and frontend separately.

Two frontend-specific notes:

- `working-directory: frontend` is required on `codecov-action` because
  Vitest writes lcov entries as `src/...`, relative to the frontend
  workspace. Without the working-directory hint, the entries land in
  Codecov without a `frontend/` prefix and silently fall outside the
  `flags.frontend.paths` filter declared in `codecov.yml`.
- `carryforward: true` per flag (in `codecov.yml`) is non-negotiable while
  CI is path-scoped: a backend-only PR never uploads frontend coverage,
  and Codecov would otherwise treat the missing upload as 0%, failing the
  frontend project status. See `docs/ci.md` § "Workflow scope and
  concurrency" for why path-scoping is the canonical pattern.

### Frontend deploy

Production deploys to Vercel are managed by Vercel's native Git integration: pushing to `main` triggers Vercel to build and deploy automatically, independently of GitHub Actions. The frontend workflow therefore runs only `lint-test-build` (lint, typecheck, build, test, coverage upload) — there is no `deploy` job in `.github/workflows/frontend.yml`, and no Vercel CLI authentication secrets are required on the GitHub side. Vercel reports deploy status back to GitHub via Commit Status / `deployment_status` events, which surface in the GitHub UI alongside the Actions checks.

#### Build environment mismatch risk

`vercel build` (run by Vercel, not by this workflow) is distinct from the repo's `pnpm build`. If the Node version configured in the Vercel project dashboard differs from the version pinned in `mise.toml` at the repo root (managed by mise), validation can pass in CI while the Vercel-side build fails — or, worse, silently produces a different output. Verify that the Vercel project's Node version setting matches the version in `mise.toml` under Project → Settings → General → Node.js Version.

#### Production env vars live only in the Vercel project

`lint-test-build` sets `BACKEND_URL`, `NEXT_PUBLIC_SUPABASE_URL`, and `NEXT_PUBLIC_SUPABASE_ANON_KEY` to dummy values so `next build` can validate the env schema without real credentials. The Vercel project's own production environment configuration is the only source of truth for the real values — never copy production secrets into the workflow's `env:` block, since `NEXT_PUBLIC_*` vars are baked into the client-side JavaScript bundle at build time and a stray dummy would ship to users.

## ER-chart workflow

`.github/workflows/er-chart.yml` generates an entity-relationship diagram and table documentation from the live database schema using SchemaSpy, then publishes the output to GitHub Pages at `https://yasuflatland-lf.github.io/flamingo-armond/er-chart/`.

### Why a live database, not the schema files

SchemaSpy reads a live PostgreSQL instance - it does not parse GraphQL schema files under `schema/` or migration SQL files directly. The workflow therefore stands up an ephemeral `postgres:15` service container, applies the schema to it, and points SchemaSpy at the running database. This produces accurate FK relationships, index annotations, and column nullability that static file parsing cannot derive reliably.

### The Supabase-coupling problem and the auth stub

The consolidated initial migration is written for Supabase and references objects that vanilla Postgres does not provide:

- a FK to `auth.users`
- a trigger on `auth.users`
- RLS policies that call `auth.uid()` in their `USING` expression

`CREATE POLICY` validates its `USING` expression at creation time, so `auth.uid()` must already resolve before any migration runs. `tools/schemaspy/auth-stub.sql` satisfies this requirement by creating the `auth` schema, the `auth.users` table, the `auth.uid()` function, and the `authenticated` role before the migrations are applied.

The bootstrap SQL in `tools/schemaspy/auth-stub.sql` is a faithful copy of the identical inline bootstrap embedded in four backend integration tests: `internal/database/pool_test.go`, `internal/repository/user_test.go`, `cmd/server/main_test.go`, and `cmd/seed/main_test.go`. The tests do not source from the shared file; deduplication is deliberately deferred. The bootstrap SQL therefore lives in five places. Changing it means updating all five copies - the tests do not pick up changes to `tools/schemaspy/auth-stub.sql` automatically.

### Pipeline shape

1. An ephemeral `postgres:15` service container starts with a known DSN.
2. The auth stub (`tools/schemaspy/auth-stub.sql`) runs against the service database.
3. All `backend/internal/database/migrations/*.up.sql` files are applied in lexical (chronological) order.
4. `schemaspy/schemaspy:7.0.2` runs in Docker with:
   - `-t pgsql11` - the `pgsql11` driver works for PG 11 and later, including PG 15.
   - `--network host` - lets the container reach the service Postgres on `localhost:5432` (Linux CI runner only; see "Local reproduction" below).
   - The output directory inside the image is always `/output`; the workflow bind-mounts a local `erout/` directory there and passes no `-o` flag.
   - `-s public` - restricts analysis to the `public` schema.
5. `JamesIves/github-pages-deploy-action@v4` publishes the `erout/` directory to the `gh-pages` branch under `er-chart/` with `clean: true` and `target-folder: er-chart`.

### Fail-fast guards before publish

`clean: true` replaces the entire `er-chart/` subtree on every publish. If generation produced nothing - for example because no migrations applied or the schema was empty - SchemaSpy can still exit 0, and an empty artifact would overwrite the previously-correct chart.

Three guards abort the workflow before publishing:

- No `*.up.sql` files found - the migration directory is missing or misconfigured.
- The `public` schema has zero base tables after migrations - the schema did not apply correctly.
- SchemaSpy did not produce `erout/index.html` - the generation run was empty.

**Generalizable rule:** any `clean: true` publish step needs a pre-publish content guard, because the deploy is destructive and the generator's exit code alone does not prove non-empty output.

### Trigger and permissions

- Triggers on `push` to `main`, filtered to paths under `backend/internal/database/migrations/`, `tools/schemaspy/`, and the workflow file itself.
- Also triggers on `workflow_dispatch` for manual reruns.
- There is deliberately no `pull_request` trigger. The job pushes to `gh-pages`; running on PRs would publish intermediate or unreviewed schema states.
- `permissions: contents: write` is required for the GitHub Pages deploy step.

### Image pinning

`schemaspy/schemaspy:7.0.2` is a pinned released tag. Never use `latest` or `snapshot`; those are moving tags that can change behavior silently between runs without any diff in the workflow file.

### One-time operator setup

After the first successful run creates the `gh-pages` branch, enable GitHub Pages in the repository settings: Settings -> Pages -> Source: "Deploy from a branch" -> Branch: `gh-pages`, folder `/ (root)`. The first run succeeds and creates the branch even before Pages is enabled; the site goes live only after this configuration step.

### Local reproduction caveat

The workflow passes `--network host` to the SchemaSpy Docker container, which is specific to Linux CI runners. On macOS with Docker Desktop, host networking is not available. To reproduce locally on macOS, create a user-defined Docker network, start the Postgres container on that network, and pass `-host <postgres-container-name>` to SchemaSpy instead of `--network host`. A host-networking failure on macOS is an environment limitation, not a workflow bug.
