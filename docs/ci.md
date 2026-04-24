# CI / CD notes

Operational decisions around GitHub Actions and external services that are not obvious from the workflow YAML alone.

## Workflow scope and concurrency

`.github/workflows/backend.yml` is the only active workflow.

- **Triggers are `paths:`-scoped** to `backend/**`, the workflow file, and `render.yaml`. When adding a second service (e.g. `frontend/`), **add its own workflow** — do not broaden this one. Mixing scopes breaks CI granularity and responsibility.
- **`concurrency: backend-${{ github.ref }}` with `cancel-in-progress: true`** — rapid pushes on the same ref supersede in-flight runs (important for feature-branch iteration).

## Deploy gating

- The `deploy` job is `needs: test` and `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` — doubly restricted.
- If `RENDER_DEPLOY_HOOK_URL` is missing, the step **explicitly exits 1** rather than silently skipping. Missing secrets are misconfiguration and should fail loudly. **Do not replace this with a silent skip.**
- `render.yaml` is the source of truth on the Render side: `rootDir: backend`, build `./cmd/server` to `main`, `autoDeploy: false` (deploys are push-triggered via the hook, not Render's auto-deploy), `healthCheckPath: /health`.

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

## Git pathspec under `working-directory:` is cwd-relative

The `test` job declares `defaults.run.working-directory: backend`, so every `run:` step starts with cwd in `backend/`. When a step invokes `git diff -- <pathspec>`, the pathspec is resolved relative to the shell cwd, **not** the repository root. Writing `git diff -- backend/graph/resolver/*.resolvers.go` would be interpreted as `backend/backend/graph/...`, which matches nothing; `git diff --exit-code` then returns 0 and the check silently passes regardless of actual drift.

Two safe forms:

- cwd-relative: `git diff --exit-code -- graph/resolver/*.resolvers.go` (current form).
- Repo-root-anchored: `git diff --exit-code -- :/backend/graph/resolver/*.resolvers.go` (the `:/` magic signature).

Do **not** mix the two by keeping the full `backend/...` path when `working-directory` is already `backend/`.
