# CI / CD notes

Operational decisions around GitHub Actions and external services that are not obvious from the workflow YAML alone.

## Workflow scope and concurrency

Two independent workflows: `.github/workflows/backend.yml` and `.github/workflows/frontend.yml`.

- **Triggers are `paths:`-scoped** per workflow — backend to `backend/**` + workflow file + `ops/terraform/modules/render/**`; frontend to `frontend/**` + `schema/**` + the root pnpm/workspace/tool-version manifests + the frontend workflow file. When adding a third service, **add its own workflow** — do not broaden an existing one. Mixing scopes breaks CI granularity and responsibility.
- **`concurrency` groups are per-workflow** (`backend-${{ github.ref }}`, `frontend-${{ github.ref }}`) with `cancel-in-progress: true` — rapid pushes on the same ref supersede in-flight runs per service (important for feature-branch iteration). The two workflows do not cancel each other.

## Deploy gating

- The `deploy` job is `needs: test` and `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` — doubly restricted.
- If `RENDER_DEPLOY_HOOK_URL` is missing, the step **explicitly exits 1** rather than silently skipping. Missing secrets are misconfiguration and should fail loudly. **Do not replace this with a silent skip.**
- `ops/terraform/modules/render/main.tf` is the source of truth on the Render side: `root_directory = "backend"`, build `./cmd/server` to `main`, `auto_deploy = false` (deploys are push-triggered via the hook, not Render's auto-deploy), `health_check_path = "/health"`.

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

## Frontend workflow

`.github/workflows/frontend.yml` mirrors the backend workflow's structure — per-service scope, major-tag pinning, per-ref concurrency — but has a different install/verify pipeline because the frontend is a pnpm workspace rooted at the repo root.

### `pnpm install --frozen-lockfile` is the gate

CI always runs `pnpm install --frozen-lockfile` (never plain `pnpm install`). With the `--frozen-lockfile` flag, pnpm refuses to mutate `pnpm-lock.yaml` and exits non-zero if the lockfile and the declared dependencies disagree. This is the only mechanism that catches "I edited `package.json` but forgot to re-run `pnpm install` locally" — without it, CI would silently regenerate the lockfile in-place and the drift would reach main.

For the same reason, the frontend workflow's `paths:` filter includes `pnpm-lock.yaml`, `pnpm-workspace.yaml`, root `package.json`, and `.tool-versions` alongside `frontend/**` and `schema/**`. A change to any of those can invalidate the frozen-lockfile invariant, so the workflow must run on those edits even when no file under `frontend/` changed.

### `--if-present` on the test step (revisit when Vitest lands)

Per "pnpm workspace filter exits 0 for missing scripts" above, a missing `test` script in `frontend/package.json` would silently pass with plain `pnpm --filter frontend test`. The current workflow runs `pnpm --filter frontend --if-present test` specifically so the step becomes a documented no-op today and **automatically activates** once PR 5 adds the `test` script plus a Vitest config — no workflow edit needed at that point. When Vitest lands, do not drop the `--if-present` flag: it stays as a guard against future script renames.

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

## Terraform CI

`.github/workflows/terraform.yml` validates HCL when a push to `main` or a PR touches `ops/terraform/**` or `.github/workflows/terraform.yml`. Both triggers are path-gated like the other workflows.

### What is checked

- **Format** (`fmt-check` job): `terraform fmt -check -recursive` across the entire `ops/terraform/` tree. Any unformatted file fails the job immediately.
- **Init + validate** (`validate` job): For each env stack under `ops/terraform/envs/prod/`, the job runs `terraform init -backend=false` (provider/module resolution, no real backend configured) followed by `terraform validate` (type-checks all HCL expressions and references). Reusable modules under `ops/terraform/modules/*/` are intentionally **not** listed in the matrix — every module is consumed by an env stack and is therefore validated transitively when that stack is initialized; listing them again would duplicate coverage and pay 5× the runner-setup cost. If a module is ever added without a consumer, add it to the matrix as a temporary entry until an env stack picks it up. A validate failure exits non-zero — `continue-on-error` is never used on these steps.

### What is NOT checked

- `terraform plan` and `terraform apply` — these require real credentials and a live backend. They are intentionally excluded from PR CI. Apply is a manual operator action.
- Drift detection — not run in CI. Operators check drift before applying.
- Security scanning (e.g. `tfsec`, `checkov`) — not yet wired in; add as a separate job if needed.
- Module locking (`terraform providers lock`) — `.terraform.lock.hcl` files are not yet committed; `terraform init` downloads providers fresh on each CI run. Commit them with `terraform providers lock -platform=linux_amd64 -platform=darwin_arm64` per stack/module if pinning by hash becomes a hard requirement.

### Fan-in job for matrix branch protection

The `validate` job is a `strategy: matrix`, so each matrix leg becomes its own GitHub status check (`Validate ops/terraform/envs/prod/initial`, `Validate ops/terraform/envs/prod/settings`, …). If branch protection requires a specific leg by name, every other leg is unprotected — and adding a new path to the matrix silently leaves it outside the gate.

`validate-all` is a trivial fan-in that depends on `[fmt-check, validate]`. Branch protection should require **`All Terraform validations passed`** (the `validate-all` job's display name), not the individual matrix legs. Adding a new path then automatically falls under the same gate. Apply the same pattern when introducing any new matrix workflow.

### Settings stack and `data.terraform_remote_state`

The `envs/prod/settings` stack reads the `initial` stack's outputs via `data.terraform_remote_state` with a local backend pointing at `../initial/terraform.tfstate`. That state file does not exist in CI, but `terraform validate` does **not** evaluate data sources — it only type-checks HCL expressions and references. The settings stack therefore validates cleanly without a state file, and the matrix job treats it the same as every other directory.

If the remote state reference is later replaced with a Terraform Cloud / S3 backend, `terraform init -backend=false` continues to work because the flag tells Terraform to skip backend initialization entirely.
