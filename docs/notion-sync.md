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
- `NOTION_MAX_ELAPSED`: default `20s` (kept below the server's 30s `WriteTimeout`).

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
- The root `.env` file is populated with the keys listed below.

### Required keys in root `.env`

| Key | Purpose |
|---|---|
| `NOTION_TOKEN` | Notion integration token (shared with prod). Same workspace as production by default. |
| `NOTION_PAGE_IDS` | Comma-separated list of Notion page UUIDs to sync (shared with prod). |
| `NOTION_LOCAL_TARGET_CARDGROUP_NAME` | Name of the master cardgroup that holds locally-synced cards. Kept distinct from prod by convention so a misconfigured local connection cannot touch prod data. |
| `NOTION_LOCAL_SYNC_TOKEN` | Bearer token used by the local backend to authenticate the `/internal/notion-sync` POST. Local-only; not pushed to Render. |

#### Hybrid env model: shared vs. local-only keys

The local-testing keys split into two groups by design:

- **Shared with prod** — `NOTION_TOKEN` and `NOTION_PAGE_IDS` are read from the same root `.env` keys that the production flow uses. There is no `NOTION_LOCAL_TOKEN`; the same Notion integration token authenticates against the same Notion workspace in both flows.
- **Local-only** — `NOTION_LOCAL_TARGET_CARDGROUP_NAME` and `NOTION_LOCAL_SYNC_TOKEN` exist solely to point the backend at a local master cardgroup and a local bearer token. The production sync push (`make sync-notion-secrets`) does not read these — it pushes the prod-specific keys (`NOTION_MASTER_CARDGROUP_NAME`, `NOTION_SYNC_TOKEN`) listed in [§ "Required keys in root `.env`"](#required-keys-in-root-env-1) below, and ignores any `NOTION_LOCAL_*` entry.

The `NOTION_LOCAL_*` prefix is the boundary marker: anything under it is consumed only by `make notion-local-setup` and is rewritten into the prod-equivalent key name (e.g. `NOTION_LOCAL_SYNC_TOKEN` → `NOTION_SYNC_TOKEN`, `NOTION_LOCAL_TARGET_CARDGROUP_NAME` → `NOTION_MASTER_CARDGROUP_NAME`) inside `backend/.env.local`. Keeping the master cardgroup name distinct from prod by convention also means a misconfigured local connection cannot touch prod data.

### Quickstart

```bash
make notion-local-setup    # one-time
make dev-backend           # in another terminal
make notion-local-run      # repeat as needed
```

`make notion-local-setup` validates that the required keys are present in root `.env`, then writes the `NOTION_*` keys configured in the Makefile target into `backend/.env.local`. `make dev-backend` starts the backend on `:1323` with those env vars loaded. `make notion-local-run` fires a single authenticated POST to `/internal/notion-sync` and prints the HTTP response body. Backend progress logs appear in the terminal where `make dev-backend` is running.

### Troubleshooting

| Symptom | Fix |
|---|---|
| `make notion-local-run` prints "Backend not reachable on :1323" | Start the backend in another terminal with `make dev-backend`, then re-run. |
| Backend logs show "notion sync: disabled" | Check `backend/.env.local` for the four required `NOTION_*` keys (`NOTION_TOKEN`, `NOTION_PAGE_IDS`, `NOTION_MASTER_CARDGROUP_NAME`, `NOTION_SYNC_TOKEN`). Re-run `make notion-local-setup` if missing, then restart the backend. |

For the production sync flow, see [Operational setup](#operational-setup) below.

## Operational setup

> **Deploy action required.** `render.yaml`, the Ansible playbook, and the `make sync-notion-*` targets now use `NOTION_MASTER_CARDGROUP_NAME` (the backend's variable). The **deployed Render service env still carries the old `NOTION_TARGET_OWNER_ID` / `NOTION_TARGET_CARDGROUP_NAME` keys** until an operator runs Blueprint Manual Sync (which creates the new `NOTION_MASTER_CARDGROUP_NAME` placeholder from `render.yaml`) followed by `make sync-notion-secrets` (which pushes its value from root `.env`). The old `NOTION_TARGET_*` keys are inert once the new key is set and can be deleted from the service manually.

All NOTION_* values are stored in the root `.env` file (gitignored). The Makefile provides three targets that read from that file and push values to the appropriate destinations.

### Required keys in root `.env`

| Key | Required | Notes |
|---|---|---|
| `NOTION_TOKEN` | yes | Notion integration token |
| `NOTION_PAGE_IDS` | yes | Comma-separated page IDs |
| `NOTION_MASTER_CARDGROUP_NAME` | yes | Destination master cardgroup name; created when absent. Owner-less — no owner email/UUID is required. |
| `NOTION_SYNC_TOKEN` | yes | Shared bearer token — written to both Render env and the GHA secret (see note below) |
| `NOTION_MAX_ATTEMPTS` | no | Defaults to `5` |
| `NOTION_MAX_ELAPSED` | no | Defaults to `20s` |

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

The upsert carries no equality predicate — [`backend/internal/repository/bulk_card_tx.go`](../backend/internal/repository/bulk_card_tx.go) emits `DO UPDATE SET back = EXCLUDED.back, updated_at = now(), position = EXCLUDED.position` — so every conflicting row is rewritten even when nothing about it changed. Two consequences are easy to misread. First, the response's `inserted` / `updated` tallies (also carried by the `notion sync complete` log line) are **not** change detection: they partition rows by whether the row already existed, not by whether anything differs, so re-syncing a byte-identical Notion page reports every surviving row as updated. Second, `updated_at` is refreshed on every row of every sync, so `UPDATED_AT` ordering (`MasterCardOrderBy.UPDATED_AT`) carries no information for synced master decks — order by `POSITION` instead.

`MasterCardgroupRepository.EnsureByName` commits in its own transaction, before the transaction that upserts and prunes the cards is opened. A first sync for a deck name that fails anywhere after that call therefore leaves a master cardgroup row behind with zero cards — including the `422` raised when every parsed row fails domain construction, which returns before the card transaction is ever opened. The row is created unpublished, so it is `DRAFT` and reachable only through the admin catalog — no learner sees it — and the next successful sync for the same name heals it by upserting the cards into that same row. An empty deck appearing after a failed run is this side effect, not a separate defect.

Rows are dropped through two independent channels: **grammar-level skips** (lone front-only or back-only lines, recovered by the parser) and **domain-constructor-level skips** (a parsed row whose `domain.NewMasterCard` call fails — empty or over-`CardTextMax` front/back). Neither channel drops the rest of the batch, but the two report differently, and the reporting also differs between the partial-failure and the all-rows-dropped path:

- **Partial failure (some rows survive).** The sync persists the survivors and returns `200` with the grammar-level skips in the response's `parseErrors` array. Domain-constructor-level skips are **not** in `parseErrors` on this path — they are recorded server-side as structured warns (`notion sync: skipping invalid master card row`), so a row silently disappears from the published deck unless the operator inspects the sync logs. Such a row is also pruned from the master cardgroup, because the diff-prune keep-set is built from the surviving cards; that per-row prune is deliberate.
- **Every non-blank row dropped.** No cards are mutated either way, but the two channels surface it through different mechanisms. A grammar-only payload short-circuits before `EnsureByName` — so no master cardgroup is auto-created, a deliberate "no persistence, no side effects" invariant — and returns `200` with every skip diagnostic in `parseErrors`. A payload whose every row fails domain construction is rejected as `ErrNotionSyncInvalidInput` (`422`) before the persistence transaction opens — but `EnsureByName` has already committed by then, so on a first sync it still leaves the empty `DRAFT` row described above; the error message carries the dropped-row count and the first offending line, and the per-row detail is in the warn logs rather than in `parseErrors`. This second guard exists because an all-dropped batch leaves an empty keep-set, which would otherwise delete every card in the target master cardgroup.

For the grammar-level rationale, see [`docs/backend/library-gotchas/goyacc-lexer-recovery-via-newline.md` § "What"](backend/library-gotchas/goyacc-lexer-recovery-via-newline.md#what).

If Notion returns `429` or `5xx`, the backend retries, and `retryDelay` in [`backend/internal/notion/client.go`](../backend/internal/notion/client.go) picks the wait through three distinct branches:

- **A usable `Retry-After` header is honoured**, whatever the status — either a non-negative integer count of seconds, or an HTTP-date still in the future, in which case the wait is the time remaining until it. A header that parses as neither, or names a date already past, falls through to the branches below.
- **`429` with no usable header waits a fixed 1 second.** It is neither exponential nor attempt-dependent, so a burst of rate-limited responses is spaced one second apart however long the burst runs.
- **Anything else waits `2^(attempt-1)` seconds, capped at 5 seconds** — in practice a `5xx` without a usable header.

Only a response can be retried. A transport failure that produced no response at all — a connection reset, a DNS failure, a TLS error — is not retried at any backoff: `RoundTrip` returns it immediately, and it surfaces as `ErrNotionSyncFetch` (`502`).

The endpoint maps internal sentinels to HTTP statuses as follows:

- `422 Unprocessable Entity` — invalid input (`ErrNotionSyncInvalidInput`: missing config, empty page IDs, parsed-row cap exceeded, or every parsed row rejected by domain construction) and parse failures (`ErrNotionSyncParse`: malformed dictionary content from Notion).
- `502 Bad Gateway` — Notion API call failures (`ErrNotionSyncFetch`).
- `504 Gateway Timeout` — retry budget exhausted (`NOTION_MAX_ATTEMPTS`, `NOTION_MAX_ELAPSED`) or request context cancelled / timed out.
- `500 Internal Server Error` — persistence failures (`ErrNotionSyncPersist`) and any unmapped error.

The handler wraps the whole sync in a request-context deadline (25s) that sits below the server's 30s `WriteTimeout`. This guarantees a slow sync is cancelled and returns a deliverable `504` before the connection write deadline expires — otherwise the sync commits on the backend but the response write fails against the expired deadline, so the caller sees a connection reset for work that actually succeeded and a retry re-runs the entire sync.

The operator-visible tradeoff is that a legitimate large sync whose work lands in the 25–30s band now surfaces as a deliverable `504` even when the persistence step already committed. Confirm a run's true outcome from the server-side `notion sync complete` log line rather than trusting the HTTP status alone: a `504` accompanied by that log record means the sync succeeded and the timeout only truncated the response.

Concurrent triggers from GitHub Actions (cron + workflow_dispatch) are serialized via the workflow's concurrency group, so only one sync runs at a time on that path.
