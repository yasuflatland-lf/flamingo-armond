# Notion Page Sync

The backend exposes `POST /internal/notion-sync` for GitHub Actions or another trusted scheduler. The request body is ignored; all sync inputs come from backend environment variables.

## Backend env

Required:

- `NOTION_TOKEN`: Notion integration token.
- `NOTION_PAGE_IDS`: comma-separated page IDs.
- `NOTION_TARGET_OWNER_ID`: existing Supabase/auth user UUID that owns the destination cardgroup.
- `NOTION_TARGET_CARDGROUP_NAME`: destination cardgroup name. The backend creates it when absent.
- `NOTION_SYNC_TOKEN`: shared bearer token used by the scheduler.

Optional:

- `NOTION_MAX_ATTEMPTS`: default `5`.
- `NOTION_MAX_ELAPSED`: default `2m`.

`NOTION_TARGET_OWNER_ID` is required because the current `cardgroups` table is user-owned. Use the UUID from `auth.users.id` / `public.users.id` for the account that should see the synced cards.

## GitHub Actions

The workflow lives at [`.github/workflows/notion-sync.yml`](../.github/workflows/notion-sync.yml). It runs on a 6-hourly cron and supports manual `workflow_dispatch` triggers.

Required GitHub secrets (Settings → Secrets and variables → Actions):

- `NOTION_SYNC_URL`: full URL to the backend endpoint, e.g. `https://<render-service>/internal/notion-sync`.
- `NOTION_SYNC_TOKEN`: shared bearer token, must match the backend's `NOTION_SYNC_TOKEN` env var.

The workflow declares `concurrency: group: notion-sync, cancel-in-progress: false`, so overlapping triggers from cron and `workflow_dispatch` are serialized: a second trigger queues until the first completes rather than running in parallel.

## Behavior

The backend fetches every configured page, renders supported blocks to plain text, parses the existing text dictionary format, then upserts cards into the destination cardgroup. Existing FSRS state is preserved because updates only overwrite `back` and `updated_at`. Cards whose `front` no longer appears in Notion are deleted.

Lone front-only or back-only lines are skipped and reported as validation errors; they do not drop the rest of the batch. If every non-blank row is skipped, the sync returns the skip diagnostics without mutating cards. The skip-only short-circuit returns before `EnsureByName`, so no cardgroup is auto-created for a sync that would produce no cards — a deliberate "no persistence, no side effects" invariant. For the grammar-level rationale, see [`docs/backend/library-gotchas/goyacc-lexer-recovery-via-newline.md` § "What"](backend/library-gotchas/goyacc-lexer-recovery-via-newline.md#what).

If Notion returns `429` or `5xx`, the backend follows the `Retry-After` header (with an exponential-backoff fallback capped at 5 seconds when the header is absent).

The endpoint maps internal sentinels to HTTP statuses as follows:

- `422 Unprocessable Entity` — invalid input (`ErrNotionSyncInvalidInput`: missing config, empty page IDs, or parsed-row cap exceeded) and parse failures (`ErrNotionSyncParse`: malformed dictionary content from Notion).
- `502 Bad Gateway` — Notion API call failures (`ErrNotionSyncFetch`).
- `504 Gateway Timeout` — retry budget exhausted (`NOTION_MAX_ATTEMPTS`, `NOTION_MAX_ELAPSED`) or request context cancelled / timed out.
- `500 Internal Server Error` — persistence failures (`ErrNotionSyncPersist`) and any unmapped error.

Concurrent triggers from GitHub Actions (cron + workflow_dispatch) are serialized via the workflow's concurrency group, so only one sync runs at a time on that path.
