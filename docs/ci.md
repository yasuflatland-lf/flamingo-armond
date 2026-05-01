# CI / CD notes

Operational decisions around GitHub Actions and external services that are not obvious from the workflow YAML alone.

## Workflow scope and concurrency

Three independent workflows: `.github/workflows/backend.yml`, `.github/workflows/frontend.yml`, and `.github/workflows/e2e.yml`.

- **Triggers are `paths:`-scoped** per workflow — backend to `backend/**` + workflow file; frontend to `frontend/**` + `schema/**` + the root pnpm/workspace/tool-version manifests + the frontend workflow file. When adding a third service, **add its own workflow** — do not broaden an existing one. Mixing scopes breaks CI granularity and responsibility.
- **`concurrency` groups are per-workflow** (`backend-${{ github.ref }}`, `frontend-${{ github.ref }}`, `e2e-${{ github.ref }}`) with `cancel-in-progress: true` — rapid pushes on the same ref supersede in-flight runs per service (important for feature-branch iteration). The three workflows do not cancel each other, except that E2E uses a PR-only `cancel-in-progress` split; see § "E2E workflow".
- **General rule for `cancel-in-progress`:** jobs whose effects are confined to the runner (lint, test, build artifacts in transit) are safe for `cancel-in-progress: true`. Jobs that have already committed external state (deploys, releases, side-effecting API calls) must override with `cancel-in-progress: false` to avoid leaving external systems in an indeterminate state. The backend `deploy` job in `backend.yml` follows this rule; see § "Deploy gating".

## E2E workflow

`.github/workflows/e2e.yml` runs Playwright against a local Supabase-backed stack. It triggers on frontend/schema/Supabase-config changes, pushes to `main`, and a nightly `0 4 * * *` UTC cron.

The workflow uses split concurrency: pull-request runs use `cancel-in-progress: true`, while push and cron runs do not. Post-merge and scheduled runs must produce a definitive pass/fail signal for `main` and the nightly cadence; cancellation by a later push would mask regressions on the integration boundary. PR runs do not have this constraint and prefer `cancel-in-progress: true` to free the queue for newer pushes.

The job exports Supabase local keys from `supabase status -o env`; no repository secret is required for the service-role key. On failure it uploads the Playwright report, raw test results, and backend log with 14-day retention.

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

## Frontend codegen step positioning

`frontend/src/generated/` is gitignored (see `.gitignore`). On a fresh CI checkout the directory does not exist. The `@/generated` import used in `src/app/page.tsx` and `src/lib/apollo/server.test.ts` must resolve before `TypeScript typecheck`, `Build`, or `Vitest` run. Therefore the workflow places a dedicated `Codegen (graphql-codegen)` step immediately after `Install dependencies` and before `Biome check` / `TypeScript typecheck` / `Build` / `Vitest`.

This mirrors the backend rule (§"Codegen must run before Vet and Build"). `pnpm codegen` takes under 5s on a warm pnpm store, so an independent step adds negligible overhead. A `prebuild` lifecycle hook also runs codegen locally (`pnpm --filter frontend build` triggers it automatically); the two mechanisms coexist because CI runs `TypeScript typecheck` before `Build`, and typecheck alone does not trigger `prebuild`. We deliberately do NOT rely on `git diff --exit-code` for validation — `src/generated/` is gitignored so the diff is always empty; the real signal is `pnpm codegen` exiting 0.

## Shell command paths under `working-directory:` are cwd-relative

This applies to any shell command in a step, not only `git` pathspecs. When a step runs a `grep` or similar command and the path argument was copied from a repo-root perspective (e.g. `backend/internal`), the shell resolves it relative to the step cwd, producing `backend/backend/internal` (nonexistent). `grep` then prints an error to stderr but exits 0 on an empty match — the `if grep ...` condition evaluates false without any visible failure. Always write paths relative to the declared `working-directory`.

**Chained `grep` exit-code masking.** `if grep -X | grep -Y` only checks the exit code of the last `grep`. A missing-path error on the first `grep` is masked — the pipeline returns 0 and the lint step appears to pass. Prefer running each `grep` independently or use `pipefail` (`set -o pipefail`) when piping.

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

For the same reason, the frontend workflow's `paths:` filter includes `pnpm-lock.yaml`, `pnpm-workspace.yaml`, root `package.json`, and `.tool-versions` alongside `frontend/**` and `schema/**`. A change to any of those can invalidate the frozen-lockfile invariant, so the workflow must run on those edits even when no file under `frontend/` changed.

### `--if-present` on the test step (revisit when Vitest lands)

Per "pnpm workspace filter exits 0 for missing scripts" above, a missing `test` script in `frontend/package.json` would silently pass with plain `pnpm --filter frontend test`. The current workflow runs `pnpm --filter frontend --if-present test` specifically so the step becomes a documented no-op today and **automatically activates** once the `test` script plus a Vitest config are added — no workflow edit needed at that point. When Vitest lands, do not drop the `--if-present` flag: it stays as a guard against future script renames.

### Node/pnpm provisioning via mise

The workflow uses the same `jdx/mise-action@v4` step that `backend.yml` uses, relying on the repo-root `.tool-versions` to pin both Node (`nodejs 24`) and pnpm (`pnpm 10.33.2`). mise installs both directly, so for local dev and GitHub Actions the pnpm version is pinned by the repo — not by the CI runner's preinstalled toolchain and not by Corepack. The `packageManager` field in root `package.json` is kept aligned for two reasons that are NOT informational: (1) Vercel does not run mise, so it reads `packageManager` to choose which pnpm version to install on its build image, and (2) pnpm 10 itself uses the field as a self-consistency check and refuses to run when the executing binary disagrees with the declared version. Together these keep local, CI, and Vercel pnpm versions in lockstep with a single source of truth.

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

`vercel build` (run by Vercel, not by this workflow) is distinct from the repo's `pnpm build`. If the Node version configured in the Vercel project dashboard differs from the version pinned in `.tool-versions` at the repo root (managed by mise), validation can pass in CI while the Vercel-side build fails — or, worse, silently produces a different output. Verify that the Vercel project's Node version setting matches the version in `.tool-versions` under Project → Settings → General → Node.js Version.

#### Production env vars live only in the Vercel project

`lint-test-build` sets `BACKEND_URL`, `NEXT_PUBLIC_SUPABASE_URL`, and `NEXT_PUBLIC_SUPABASE_ANON_KEY` to dummy values so `next build` can validate the env schema without real credentials. The Vercel project's own production environment configuration is the only source of truth for the real values — never copy production secrets into the workflow's `env:` block, since `NEXT_PUBLIC_*` vars are baked into the client-side JavaScript bundle at build time and a stray dummy would ship to users.
