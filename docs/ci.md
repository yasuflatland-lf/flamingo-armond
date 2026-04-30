# CI / CD notes

Operational decisions around GitHub Actions and external services that are not obvious from the workflow YAML alone.

## Workflow scope and concurrency

Two independent workflows: `.github/workflows/backend.yml` and `.github/workflows/frontend.yml`.

- **Triggers are `paths:`-scoped** per workflow — backend to `backend/**` + workflow file; frontend to `frontend/**` + `schema/**` + the root pnpm/workspace/tool-version manifests + the frontend workflow file. When adding a third service, **add its own workflow** — do not broaden an existing one. Mixing scopes breaks CI granularity and responsibility.
- **`concurrency` groups are per-workflow** (`backend-${{ github.ref }}`, `frontend-${{ github.ref }}`) with `cancel-in-progress: true` — rapid pushes on the same ref supersede in-flight runs per service (important for feature-branch iteration). The two workflows do not cancel each other.
- **Deploy-job concurrency exception:** The `deploy` job in `frontend.yml` overrides the workflow-level cancellation policy with a job-scoped `concurrency:` group (`frontend-deploy-${{ github.ref }}`) that has `cancel-in-progress: false`. This ensures that once a `vercel deploy --prebuilt` begins, it cannot be cancelled mid-flight — Vercel may have already committed the deployment server-side, so cancelling the runner would leave an indeterminate state. Lint-test-build runs continue to cancel each other aggressively to save CI minutes on stale feature-branch commits.

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

### Frontend deploy job

The `deploy` job in `frontend.yml` follows the same gating pattern as the backend: `needs: lint-test-build` ensures the full quality gate must pass before any deploy is attempted, and the job condition restricts execution to direct pushes to `main` — pull-request events and branch pushes are excluded. See § "Deploy gating" for the backend equivalent.

If any of `VERCEL_TOKEN`, `VERCEL_ORG_ID`, or `VERCEL_PROJECT_ID` is empty or unset, a guard step **explicitly exits 1** rather than silently no-oping. Failing loud on a missing secret beats a silent skip — misconfiguration must be visible. **Do not replace this with a silent skip.**

#### Required GitHub secrets

| Secret | Purpose |
|---|---|
| `VERCEL_TOKEN` | Authenticates the Vercel CLI. Use a project-scoped token when the Vercel org plan supports it (limits blast radius to a single project); otherwise use an account-scoped token with a **quarterly rotation reminder**. Must be non-empty — the guard step checks all three Vercel secrets and exits 1 if any is absent. |
| `VERCEL_ORG_ID` | Identifies the Vercel organization. Exposed as a job-level env var; the Vercel CLI auto-reads it, so no explicit `--org` flag is needed. Must be non-empty — see `VERCEL_TOKEN` note above. |
| `VERCEL_PROJECT_ID` | Identifies the target Vercel project. Exposed as a job-level env var; the Vercel CLI resolves the project without a `vercel link` step. Must be non-empty — see `VERCEL_TOKEN` note above. |

Register all three in the repository's GitHub secrets before the workflow runs. `VERCEL_ORG_ID` and `VERCEL_PROJECT_ID` are visible in the Vercel dashboard under Project → Settings → General.

#### pnpm install in the deploy job

The `deploy` job runs `pnpm install --frozen-lockfile` (with a pnpm store cache that uses the same key as `lint-test-build`, so the deploy job benefits from the warm cache produced by the preceding job — both run on the same OS and the same `pnpm-lock.yaml` hash, so divergent keys would only waste cache space) before invoking `vercel build`, because `vercel build` executes `next build` locally on the runner and requires `node_modules` to be populated. See § "`pnpm install --frozen-lockfile` is the gate" for the repo-wide invariant this satisfies.

#### CLI deploy path vs. Vercel Git integration

**This is a transitional configuration.** Both the CLI deploy path (via this workflow) and Vercel's native Git integration are currently active — Vercel's integration fires on every push independently of the workflow. The CLI deploy is the *authoritative* path going forward: it is controlled by the same gating (`needs: lint-test-build`, main-only) that governs the rest of the release pipeline, and its output is observable in the Actions log alongside all other CI steps. The Git integration will be disabled in a follow-up change; see `docs/deployment.md` § 'Why CI deploy is now the authoritative path' for the resolution plan.

#### Build env mismatch risk

`vercel build` runs Vercel's own build pipeline, which is distinct from the repo's `pnpm build`. If the Node version configured in the Vercel project dashboard differs from the version pinned in `.tool-versions` at the repo root (managed by mise), validation can pass in CI while the Vercel-side build fails — or, worse, silently produces a different output. Verify that the Vercel project's Node version setting matches the Node version in `.tool-versions` in the Vercel dashboard under Project → Settings → General → Node.js Version.

#### Why the deploy job intentionally omits build-time env vars

The `lint-test-build` job sets `BACKEND_URL`, `NEXT_PUBLIC_SUPABASE_URL`, and `NEXT_PUBLIC_SUPABASE_ANON_KEY` to dummy values so `next build` can validate the env schema without real credentials. The `deploy` job **deliberately does not set any of these**. Instead, `vercel pull --environment=production` retrieves the real production values from the Vercel project's environment configuration and writes them to `.vercel/.env.production.local`, which `vercel build` then consumes.

**Trap**: if a maintainer copies the dummy-value `env:` block from `lint-test-build` into the `deploy` job, the production build will silently bake those dummy values (e.g. a `localhost` Supabase URL) into the client-side JavaScript bundle. The Vercel project's production environment configuration must be the only source of truth for these values — never duplicate them in the deploy job.

