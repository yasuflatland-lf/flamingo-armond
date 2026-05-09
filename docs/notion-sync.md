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

If Notion returns `429` or `5xx`, the backend follows the `Retry-After` header (with an exponential-backoff fallback capped at 5 seconds when the header is absent). If retry attempts exceed `NOTION_MAX_ATTEMPTS` or cumulative wait exceeds `NOTION_MAX_ELAPSED`, the endpoint returns `504`. Other Notion fetch failures return `502`.

Concurrent triggers from GitHub Actions (cron + workflow_dispatch) are serialized via the workflow's concurrency group, so only one sync runs at a time on that path.
