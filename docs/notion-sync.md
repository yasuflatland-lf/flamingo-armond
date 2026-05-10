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

See [Operational setup](#operational-setup) below for the `make` targets that populate these secrets automatically.

The workflow declares `concurrency: group: notion-sync, cancel-in-progress: false`, so overlapping triggers from cron and `workflow_dispatch` are serialized: a second trigger queues until the first completes rather than running in parallel.

## Operational setup

All NOTION_* values are stored in the root `.env` file (gitignored). The Makefile provides three targets that read from that file and push values to the appropriate destinations.

### Required keys in root `.env`

| Key | Required | Notes |
|---|---|---|
| `NOTION_TOKEN` | yes | Notion integration token |
| `NOTION_PAGE_IDS` | yes | Comma-separated page IDs |
| `NOTION_TARGET_OWNER_ID` | yes | Supabase `auth.users.id` UUID for the destination owner |
| `NOTION_TARGET_CARDGROUP_NAME` | yes | Destination cardgroup name; created when absent |
| `NOTION_SYNC_TOKEN` | yes | Shared bearer token — written to both Render env and the GHA secret (see note below) |
| `NOTION_MAX_ATTEMPTS` | no | Defaults to `5` |
| `NOTION_MAX_ELAPSED` | no | Defaults to `2m` |

`NOTION_SYNC_TOKEN` is deliberately written to **two destinations with the same value**: the Render service env and the `NOTION_SYNC_TOKEN` GitHub Actions secret. This is what guarantees bearer-auth integrity — the backend validates the token in the incoming `Authorization: Bearer` header, and the GHA workflow supplies it as that same secret. If the two values drift, every sync request returns `401`.

### Make targets

**Verify only — no writes:**

```bash
make sync-notion-preflight
```

Checks that root `.env` contains every required NOTION_* key. Exits non-zero and prints the missing keys if any are absent. Run this before any push step to confirm your `.env` is complete.

**Push Notion secrets:**

```bash
make sync-notion-secrets
```

Pushes all seven NOTION_* keys to the Render service's environment variables, and writes `NOTION_SYNC_URL` (derived from the Render `backend_url` + `/internal/notion-sync`) and `NOTION_SYNC_TOKEN` to GitHub Actions secrets. The target is idempotent — safe to re-run after rotating a token or adding a new page ID.

**Full production sync (includes Notion):**

```bash
make setup-prod-postapply
```

Runs the complete post-`terraform apply` setup, which includes `sync-notion-secrets` automatically. Use this after provisioning or re-provisioning the production environment. The playbook lives at `playbooks/setup-prod/postapply.yml`.

### Verifying after setup

After running `sync-notion-secrets` or `setup-prod-postapply`, confirm the values landed:

1. **Render dashboard** — open the service's Environment tab and confirm the NOTION_* vars are present.
2. **GitHub secrets** — `gh secret list --repo <owner>/<repo>` should show `NOTION_SYNC_URL` and `NOTION_SYNC_TOKEN`.
3. **Trigger a test run** — `gh workflow run notion-sync.yml` (requires `workflow_dispatch` enabled) and check the Actions log for a `2xx` response from the backend.

## Behavior

The backend fetches every configured page, renders supported blocks to plain text, parses the existing text dictionary format, then upserts cards into the destination cardgroup. Existing FSRS state is preserved because updates only overwrite `back` and `updated_at`. Cards whose `front` no longer appears in Notion are deleted.

If Notion returns `429` or `5xx`, the backend follows the `Retry-After` header (with an exponential-backoff fallback capped at 5 seconds when the header is absent).

The endpoint maps internal sentinels to HTTP statuses as follows:

- `422 Unprocessable Entity` — invalid input (`ErrNotionSyncInvalidInput`: missing config, empty page IDs, or parsed-row cap exceeded) and parse failures (`ErrNotionSyncParse`: malformed dictionary content from Notion).
- `502 Bad Gateway` — Notion API call failures (`ErrNotionSyncFetch`).
- `504 Gateway Timeout` — retry budget exhausted (`NOTION_MAX_ATTEMPTS`, `NOTION_MAX_ELAPSED`) or request context cancelled / timed out.
- `500 Internal Server Error` — persistence failures (`ErrNotionSyncPersist`) and any unmapped error.

Concurrent triggers from GitHub Actions (cron + workflow_dispatch) are serialized via the workflow's concurrency group, so only one sync runs at a time on that path.
