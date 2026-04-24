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

## GitHub Actions versioning

Actions are pinned to **major tags (`@vN`)**, not SHAs:

- `@vN` auto-follows patch / minor — security fixes land without intervention.
- Major bumps are reviewed manually — breaking changes stay visible.

When skipping majors (e.g. `upload-artifact@v4 → @v7`), verify the breaking changes of each skipped version:

- `actions/upload-artifact@v5+` — implicit merging of same-named artifacts was removed; uploading the same name from a matrix job now errors immediately.
- `actions/checkout@v5+` — runtime Node.js version was bumped (may not run on older self-hosted runners).
- `codecov-action@v6` — switched internals to the Codecov CLI (auth flow changed).

Pin to commit SHAs when you need stronger supply-chain guarantees, at the cost of maintenance burden. The project uses major tags for now.
