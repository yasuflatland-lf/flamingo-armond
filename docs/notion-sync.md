# Notion Page Sync

The backend exposes `POST /internal/notion-sync` for GitHub Actions or another trusted scheduler. The request body is ignored; all sync inputs come from backend environment variables. The sync writes into the admin-only `master_*` catalog tables (`master_cardgroups` / `master_cards`), not the user-owned `cardgroups` table.

## Backend env

Required:

- `NOTION_TOKEN`: Notion integration token.
- `NOTION_PAGE_IDS`: comma-separated page IDs.
- `NOTION_MASTER_CARDGROUP_NAME`: destination master cardgroup name. The backend creates it when absent.
- `NOTION_SYNC_TOKEN`: shared bearer token used by the scheduler.

Optional:

- `NOTION_MAX_ATTEMPTS`: default `5`.
- `NOTION_MAX_ELAPSED`: default `2m`.

Master cardgroups are owner-less: the destination deck is identified by `NOTION_MASTER_CARDGROUP_NAME` alone. The backend resolves the name to a master cardgroup id via `MasterCardgroupRepository.EnsureByName`, which serializes lookup-then-insert under a `pg_advisory_xact_lock(hashtext('master'), hashtext(name))` so concurrent runs cannot create duplicate-name rows. No owner UUID is required.

## GitHub Actions

The workflow lives at [`.github/workflows/notion-sync.yml`](../.github/workflows/notion-sync.yml). It runs on a 6-hourly cron and supports manual `workflow_dispatch` triggers.

Required GitHub secrets (Settings → Secrets and variables → Actions):

- `NOTION_SYNC_URL`: full URL to the backend endpoint, e.g. `https://<render-service>/internal/notion-sync`.
- `NOTION_SYNC_TOKEN`: shared bearer token, must match the backend's `NOTION_SYNC_TOKEN` env var.

See [Operational setup](#operational-setup) below for the `make` targets that populate these secrets automatically.

The workflow declares `concurrency: group: notion-sync, cancel-in-progress: false`, so overlapping triggers from cron and `workflow_dispatch` are serialized: a second trigger queues until the first completes rather than running in parallel.

## Local testing

Use this section to run the Notion sync against your local Supabase instance without touching any production environment.

### Prerequisites

- Local Supabase is running (`make supabase-start`).
- You have logged in to the local app at least once via `make dev-frontend` so your developer account's row exists in `auth.users` locally.
- The root `.env` file is populated with the keys listed below.

### Required keys in root `.env`

> **Transitional state.** The backend now reads `NOTION_MASTER_CARDGROUP_NAME` to identify the master cardgroup, but `make notion-local-setup` still writes the old `NOTION_LOCAL_TARGET_OWNER_EMAIL` / `NOTION_LOCAL_TARGET_CARDGROUP_NAME` keys into `backend/.env.local`. Until the operator follow-up in the [pending operator step callout](#operational-setup) lands, you must also set `NOTION_MASTER_CARDGROUP_NAME` manually in `backend/.env.local` (or in root `.env` and re-run `make notion-local-setup`) for local master-sync testing to work.

| Key | Purpose |
|---|---|
| `NOTION_TOKEN` | Notion integration token (shared with prod). Same workspace as production by default. |
| `NOTION_PAGE_IDS` | Comma-separated list of Notion page UUIDs to sync (shared with prod). |
| `NOTION_LOCAL_TARGET_OWNER_EMAIL` | Email of the local Supabase user who will own the synced cardgroup. Resolved to UUID at setup. |
| `SUPABASE_DB_URL` | Local Supabase Postgres connection string (e.g. `postgresql://postgres:postgres@127.0.0.1:54322/postgres`). Run `supabase status` to confirm. Used to resolve the owner email to its UUID. |
| `NOTION_LOCAL_TARGET_CARDGROUP_NAME` | Name of the cardgroup that holds locally-synced cards. Kept distinct from prod by convention so a misconfigured `SUPABASE_DB_URL` cannot delete prod data. |
| `NOTION_LOCAL_SYNC_TOKEN` | Bearer token used by the local backend to authenticate the `/internal/notion-sync` POST. Local-only; not pushed to Render. |

#### Hybrid env model: shared vs. local-only keys

The local-testing keys split into two groups by design:

- **Shared with prod** — `NOTION_TOKEN` and `NOTION_PAGE_IDS` are read from the same root `.env` keys that the production flow uses. There is no `NOTION_LOCAL_TOKEN`; the same Notion integration token authenticates against the same Notion workspace in both flows.
- **Local-only** — `NOTION_LOCAL_TARGET_OWNER_EMAIL`, `NOTION_LOCAL_TARGET_CARDGROUP_NAME`, `NOTION_LOCAL_SYNC_TOKEN`, and `SUPABASE_DB_URL` exist solely to point the backend at the local Supabase instance and a local owner/cardgroup. The production sync push (`make sync-notion-secrets`) does not read these — it pushes the prod-specific keys (`NOTION_TARGET_OWNER_ID`, `NOTION_TARGET_CARDGROUP_NAME`, `NOTION_SYNC_TOKEN`) listed in [§ "Required keys in root `.env`"](#required-keys-in-root-env-1) below, and ignores any `NOTION_LOCAL_*` entry.

The `NOTION_LOCAL_*` prefix is the boundary marker: anything under it is consumed only by `make notion-local-setup` and is rewritten into the prod-equivalent key name (e.g. `NOTION_LOCAL_SYNC_TOKEN` → `NOTION_SYNC_TOKEN`) inside `backend/.env.local`. Keeping the cardgroup name distinct from prod by convention also means a misconfigured `SUPABASE_DB_URL` cannot delete prod data.

### Quickstart

```bash
make notion-local-setup    # one-time
make dev-backend           # in another terminal
make notion-local-run      # repeat as needed
```

`make notion-local-setup` validates that the required keys are present in root `.env`, probes local Supabase connectivity, resolves `NOTION_LOCAL_TARGET_OWNER_EMAIL` to a UUID, then writes the `NOTION_*` keys configured in the Makefile target into `backend/.env.local`. `make dev-backend` starts the backend on `:1323` with those env vars loaded. `make notion-local-run` fires a single authenticated POST to `/internal/notion-sync` and prints the HTTP response body. Backend progress logs appear in the terminal where `make dev-backend` is running.

### Troubleshooting

| Symptom | Fix |
|---|---|
| `make notion-local-setup` fails with "log into local app first" | Run `make dev-frontend`, sign in via Google OAuth, then re-run `make notion-local-setup`. |
| `make notion-local-run` prints "Backend not reachable on :1323" | Start the backend in another terminal with `make dev-backend`, then re-run. |
| Backend logs show "notion sync: disabled" | Check `backend/.env.local` for the required `NOTION_*` keys (see note above about `NOTION_MASTER_CARDGROUP_NAME`). Re-run `make notion-local-setup` if missing, then restart the backend. |
| psql probe fails with connection refused | Run `make supabase-start` and wait until the local Supabase stack reports healthy. |

For the production sync flow, see [Operational setup](#operational-setup) below.

## Operational setup

> **Pending operator step (master repoint).** The backend now reads `NOTION_MASTER_CARDGROUP_NAME` and no longer reads `NOTION_TARGET_OWNER_ID` / `NOTION_TARGET_CARDGROUP_NAME`. The deployed Render service env, `render.yaml`, `cloudbuild.yaml`, and the `make sync-notion-*` targets below still reference the old variable names; they are intentionally left unchanged in the code change that repointed the sync because rotating a deployed service's env is a deploy-affecting operation. Before the next production sync run, set `NOTION_MASTER_CARDGROUP_NAME` on the Render service (and update the Makefile push targets) so the scheduler keeps working. The `NOTION_TARGET_*` keys become inert once the new key is set.

All NOTION_* values are stored in the root `.env` file (gitignored). The Makefile provides three targets that read from that file and push values to the appropriate destinations.

### Required keys in root `.env`

> **Transitional state.** The backend now reads `NOTION_MASTER_CARDGROUP_NAME` but the Makefile targets and playbook below still push/write the old `NOTION_TARGET_*` keys until the operator follow-up in the [pending operator step callout](#operational-setup) lands. The table below reflects the keys the Makefile currently reads; `NOTION_MASTER_CARDGROUP_NAME` must be set manually on the Render service until those targets are updated.

| Key | Required | Notes |
|---|---|---|
| `NOTION_TOKEN` | yes | Notion integration token |
| `NOTION_PAGE_IDS` | yes | Comma-separated page IDs |
| `NOTION_TARGET_OWNER_EMAIL` | one of these two | Email of the destination account. `make sync-notion-secrets` resolves it to a UUID automatically via the production Supabase `auth.users` table. The account must have signed in to the production app at least once. |
| `NOTION_TARGET_OWNER_ID` | one of these two | Manual fallback: UUID from `auth.users.id`. Set only when `NOTION_TARGET_OWNER_EMAIL` is blank. |
| `NOTION_TARGET_CARDGROUP_NAME` | yes | Destination cardgroup name; created when absent (Makefile only — backend reads `NOTION_MASTER_CARDGROUP_NAME` instead) |
| `NOTION_SYNC_TOKEN` | yes | Shared bearer token — written to both Render env and the GHA secret (see note below) |
| `NOTION_MAX_ATTEMPTS` | no | Defaults to `5` |
| `NOTION_MAX_ELAPSED` | no | Defaults to `2m` |

`NOTION_TARGET_OWNER_EMAIL` is preferred over `NOTION_TARGET_OWNER_ID` because it eliminates the manual UUID lookup step. When both are set, `NOTION_TARGET_OWNER_EMAIL` takes precedence and the UUID is resolved fresh each run.

`NOTION_SYNC_TOKEN` is deliberately written to **two destinations with the same value**: the Render service env and the `NOTION_SYNC_TOKEN` GitHub Actions secret. This is what guarantees bearer-auth integrity — the backend validates the token in the incoming `Authorization: Bearer` header, and the GHA workflow supplies it as that same secret. If the two values drift, every sync request returns `401`.

### Make targets

**Verify only — no writes:**

```bash
make sync-notion-preflight
```

Checks that root `.env` contains the NOTION_* keys configured in the Makefile target. Exits non-zero and prints the missing keys if any are absent. Run this before any push step to confirm your `.env` is complete.

**Push Notion secrets:**

```bash
make sync-notion-secrets
```

Pushes the NOTION_* keys configured in the Makefile target to the Render service's environment variables, and writes `NOTION_SYNC_URL` (derived from the Render `backend_url` + `/internal/notion-sync`) and `NOTION_SYNC_TOKEN` to GitHub Actions secrets. The target is idempotent — safe to re-run after rotating a token or adding a new page ID.

**Full production sync (includes Notion):**

```bash
make setup-prod-postapply
```

Runs the complete production post-provisioning Ansible phase, which includes `sync-notion-secrets` automatically. Use this after provisioning or re-provisioning the production environment. The playbook lives at `playbooks/setup-prod/postapply.yml`.

### Verifying after setup

After running `sync-notion-secrets` or `setup-prod-postapply`, confirm the values landed:

1. **Render dashboard** — open the service's Environment tab and confirm the NOTION_* vars are present.
2. **GitHub secrets** — `gh secret list --repo <owner>/<repo>` should show `NOTION_SYNC_URL` and `NOTION_SYNC_TOKEN`.
3. **Trigger a test run** — `gh workflow run notion-sync.yml` (requires `workflow_dispatch` enabled) and check the Actions log for a `2xx` response from the backend.

## Manual card write-back to Notion

When a user creates or updates a card via GraphQL and the save succeeds, the backend appends a plain-text paragraph of the form `front back` (space-separated) to the bottom of the first page listed in `NOTION_PAGE_IDS`.

**Trigger conditions:**

- `createCard` mutation completes with a new card — not a duplicate.
- `updateCard` mutation completes with an updated card.
- The duplicate path (`outcome.Duplicate != nil`) skips the write-back entirely because the card already exists in the database.

**Configuration:** No new environment variables. The write-back reuses `NOTION_TOKEN` (for API authentication) and the first entry of `NOTION_PAGE_IDS` as the target page. When any of the four `NOTION_*` variables is absent, `notionSyncDisabled` is true, the Notion `CardWritebacker` adapter is not wired, and write-back is silently disabled. This covers local development and CI environments without Notion credentials.

**Failure handling:** Any Notion API error (non-2xx response, network timeout, context expiry) emits `slog.Warn` with `card_id`, `cardgroup_id`, and `page_id` as structured fields. Card create/update is unaffected — the mutation returns successfully regardless of the write-back outcome.

**Lifecycle:** The write-back runs in a detached goroutine using `context.Background()` with a 15-second `WithTimeout`. The request context is cancelled the moment the GraphQL handler returns; deriving the goroutine context from it would abort any in-flight Notion API call immediately. See [`docs/backend/library-gotchas/fire-and-forget-goroutine-detached-context.md`](backend/library-gotchas/fire-and-forget-goroutine-detached-context.md) for the general pattern.

**Acknowledged edge case:** A card that exists in Notion but has not yet been synced to the database will produce a duplicate paragraph in Notion when manually created. The duplicate paragraph persists after the next `notion-sync` run because that run upserts the database row (a no-op) but does not deduplicate Notion page content. This is accepted as a low-frequency, low-severity situation.

## Behavior

The backend fetches every configured page, renders supported blocks to plain text, parses the existing text dictionary format, then upserts master cards into the destination master cardgroup (resolved by name via `MasterCardgroupRepository.EnsureByName`). Each card is assigned a `position` equal to its zero-based index in the deduped, document-order row list (page order, then line order within a page). `position` is an internal sync-metadata field with no GraphQL field. On upsert conflict, `back`, `updated_at`, and `position` are overwritten, so re-syncing reflects the latest Notion document order for surviving master cards. Cards whose `front` no longer appears in Notion are deleted from the master cardgroup. The master catalog is admin-only and never directly owned by an end user; per-user copies are created downstream from the published master deck.

Lone front-only or back-only lines are skipped and reported as validation errors; they do not drop the rest of the batch. If every non-blank row is skipped, the sync returns the skip diagnostics without mutating cards. The skip-only short-circuit returns before `EnsureByName`, so no master cardgroup is auto-created for a sync that would produce no cards — a deliberate "no persistence, no side effects" invariant. For the grammar-level rationale, see [`docs/backend/library-gotchas/goyacc-lexer-recovery-via-newline.md` § "What"](backend/library-gotchas/goyacc-lexer-recovery-via-newline.md#what).

If Notion returns `429` or `5xx`, the backend follows the `Retry-After` header (with an exponential-backoff fallback capped at 5 seconds when the header is absent).

The endpoint maps internal sentinels to HTTP statuses as follows:

- `422 Unprocessable Entity` — invalid input (`ErrNotionSyncInvalidInput`: missing config, empty page IDs, or parsed-row cap exceeded) and parse failures (`ErrNotionSyncParse`: malformed dictionary content from Notion).
- `502 Bad Gateway` — Notion API call failures (`ErrNotionSyncFetch`).
- `504 Gateway Timeout` — retry budget exhausted (`NOTION_MAX_ATTEMPTS`, `NOTION_MAX_ELAPSED`) or request context cancelled / timed out.
- `500 Internal Server Error` — persistence failures (`ErrNotionSyncPersist`) and any unmapped error.

Concurrent triggers from GitHub Actions (cron + workflow_dispatch) are serialized via the workflow's concurrency group, so only one sync runs at a time on that path.
