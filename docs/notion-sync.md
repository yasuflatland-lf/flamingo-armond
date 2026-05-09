# Notion Page Sync

The backend exposes `POST /internal/notion-sync` for GitHub Actions or another trusted scheduler. The request body is ignored; all sync inputs come from backend environment variables.

## Render Env

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

```yaml
name: notion-sync

on:
  schedule:
    - cron: "17 */6 * * *"
  workflow_dispatch:

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - name: Trigger backend sync
        run: |
          curl -fsS -X POST "$NOTION_SYNC_URL" \
            -H "Authorization: Bearer $NOTION_SYNC_TOKEN"
        env:
          NOTION_SYNC_URL: ${{ secrets.NOTION_SYNC_URL }}
          NOTION_SYNC_TOKEN: ${{ secrets.NOTION_SYNC_TOKEN }}
```

Set `NOTION_SYNC_URL` to `https://<render-service>/internal/notion-sync`.

## Behavior

The backend fetches every configured page, renders supported blocks to plain text, parses the existing text dictionary format, then upserts cards into the destination cardgroup. Existing FSRS state is preserved because updates only overwrite `back` and `updated_at`. Cards whose `front` no longer appears in Notion are deleted.

If Notion returns `429`, the backend follows `Retry-After`. If retry time exceeds `NOTION_MAX_ELAPSED`, the endpoint returns `504`. Non-retryable Notion fetch failures return `502`.
