# Production deployment

Manual bring-up guide for the production stack. Walk through this once when standing up a new environment from scratch; everyday redeploys (push to `main`) do not require these steps.

The stack splits across three providers, each owning a distinct concern:

| Provider | Owns |
|---|---|
| Supabase | Postgres + Auth (JWT issuer, JWKS) |
| Render | Go / Echo backend (`backend/`) |
| Vercel | Next.js frontend (`frontend/`) |

## Guided bring-up via `make setup-prod`

A thin Ansible-driven layer wraps this manual runbook with prerequisite checks, value-derivation, GitHub Secret registration, deploy-trigger, and smoke tests. It does **not** replace the dashboard work below — operators still create the Supabase project, the Render web service, and the Vercel project by hand. The remainder of this document is the authoritative manual procedure; refer to it for what each phase is doing under the hood.

```bash
make setup-prod              # full guided run: preflight + 4 dashboard steps + smoke
make setup-prod-preflight    # ~10s scanner: tokens + auth + manual-prereq reminders
make setup-prod-postapply    # re-runnable: kick first Render deploy + smoke tests
```

### Why guided, not fully automated

The earlier Terraform implementation was deleted **as an intentional IaC-abandonment decision**, not as tech-debt cleanup — the bring-up runs once per environment per lifetime, so the ROI of full API automation is low and the maintenance cost of mirroring three providers' APIs in a stateful tool is high. The playbook re-introduces the layer where humans actually make mistakes (token expiry, missed GitHub App installs, copy-paste of derived URLs, forgetting to register the deploy-hook secret) without re-introducing IaC. Reach for `--tags <phase>` or hand-runs of the manual procedure when you need to deviate; the playbook is a convenience, not a contract.

### Three Make targets, six phases

The Makefile exposes only `setup-prod`, `setup-prod-preflight`, and `setup-prod-postapply` because the four dashboard phases (`supabase`, `render`, `vercel`, `loopback`) must run in order and giving each its own target invites order-of-operation mistakes. When intentional partial runs are needed, drive them via `--tags` directly:

```bash
ansible-playbook playbooks/setup-prod.yml --tags <phase>
```

| Tag | Phase | Runbook step |
|---|---|---|
| `preflight` | 1 | (pre-bring-up scanner) |
| `supabase` | 2 | Step 1 — Supabase project create |
| `render` | 3 | Step 2 — Render web service |
| `vercel` | 4 | Step 3 — Vercel project import |
| `loopback` | 5 | Step 4 — Supabase loopback |
| `postapply` | 6 | Post-bring-up + smoke |

`supabase.yml` is imported twice from the entry playbook with different `step` vars (`create` vs `loopback`), gated by `when: step == ...` blocks inside the file. This keeps the tag → step mapping 1:1 (`--tags supabase` runs only Step 1, `--tags loopback` only Step 4) while avoiding two near-duplicate task files.

### State file (`.setup-prod.state.yml`)

The playbook persists collected values across phases in a YAML file at the repo root. It is **gitignored**, written with mode `0600`, and treated as a single-operator local artifact:

| Property | Value |
|---|---|
| Path | `<repo-root>/.setup-prod.state.yml` |
| Permissions | `0600` (re-asserted on every write) |
| Backup | `<file>.<pid>.<timestamp>~` siblings created on every write (`copy: backup: yes`). Mode bits are preserved from the source (already `0600`). |
| Vault | Not encrypted; gitignore + `0600` is the baseline |
| Tier 1 secrets | `supabase_db_url` (DB password embedded). Do not share, copy across machines, or print on screen-share. |
| Tier 2 publishable | `supabase_anon_key`. Safe to display on the operator's own screen. |
| Tier 3 IDs / URLs | `supabase_project_ref`, `*_url`, `render_service_id`, `vercel_project_id`, `production_url`, `backend_url`. Public values, used as `--tags <phase>` re-run inputs. |
| Out-of-state | API tokens (read from env each run) and the Render deploy-hook URL (consumed once via `gh secret set`, never persisted). |

Every phase that needs prior values follows a three-step idiom: `stat:` to detect first-run vs existing file → `include_vars` only when the file exists (so YAML parse errors fail loudly instead of being masked as "missing keys") → `assert:` to enforce the keys this phase actually needs → merge new values via `combine` and re-write with `mode: '0600'`. If the file is corrupted or hand-edited, restore from the most recent backup with `cp "$(ls -1t .setup-prod.state.yml.*~ 2>/dev/null | head -1)" .setup-prod.state.yml` from the repo root.

### Why most phases reject `confirm=true`

`confirm=true` is the unattended-mode flag (intended for CI / scripted re-runs). Phases 2, 3, 4, and 5 all `fail` immediately when `confirm=true` is set, because each requires the operator to paste back values from a dashboard the playbook cannot read (project ref / anon key / DSN / service ID / deploy-hook / project ID / production URL / OAuth redirect URI). Failing fast with a clear message beats hanging on a `pause:` prompt that no one will answer. Phases 1 (preflight) and 6 (postapply) are the only fully unattended phases: postapply re-runs are how operators recover from a transient Render or Vercel cold-start smoke failure.

### Three external side effects

The playbook only writes to three places outside the operator's machine: (1) `gh secret set RENDER_DEPLOY_HOOK_URL` on the GitHub repo (idempotent overwrite), (2) `POST /v1/services/<id>/deploys` against the Render API (each call enqueues a deploy, which is the operator's intent), and (3) the local state file. Re-running any phase is safe — none of the three writes accumulate state in a way that corrupts the next run.

### Two security patterns worth knowing

- **`gh secret set` via `shell:` with `stdin:`, not `--body "<URL>"`.** `--body` puts the secret on argv, where it leaks to `ps`, audit logs, and shell history. Ansible's `no_log: true` masks playbook output but does not affect argv, so piping the value through stdin is the only way to keep the deploy-hook URL out of process listings.
- **Tier-1 values are never `debug:`-printed inline with non-secret values.** `SUPABASE_DB_URL` (which carries the DB password) is shown in its own task with a "leave screen-share before reading this" warning banner, so the operator can pause sharing for that one paste.

### Render deploy polling: terminal failure states

Phase 6 polls the Render deploy with `until:` over a single condition list: HTTP `[401, 403, 404]` (auth / wrong service id) plus the Render deploy state set `[live, build_failed, update_failed, canceled, deactivated, pre_deploy_failed]`. The `until` predicate (not `failed_when`) is what actually aborts the loop — `failed_when` only sets the final task status after retries exhaust, so terminal conditions must live inside `until` itself. Without all six failure states in the predicate, a doomed deploy burns the full 15-minute retry budget before the playbook gives up. If you ever change the polling logic, keep this set complete — Render's API can return any of the above as a final state, and only `live` is success.

### Failure recovery

| Failure | Surfaces in | Recovery |
|---|---|---|
| Token missing or expired | Phase 1 `assert:` or `uri:` 401 | Update `.env`, re-run `make setup-prod-preflight` |
| GitHub App not installed | Phase 3 (Render) or Phase 4 (Vercel) API failure | Install via https://github.com/apps/render or /apps/vercel, re-run the failing phase |
| Wrong Supabase pooler tab (transaction vs session) | Phase 2 DSN `assert:` | Re-copy from **Connect → Session pooler** |
| Render deploy hits a terminal failure state | Phase 6 polling abort | Fix the underlying issue (logs in Render dashboard), re-run `make setup-prod-postapply` |
| Vercel HEAD never reaches 200 | Phase 6 retry exhaustion | Most likely cause: `main` is empty so Vercel produced no build. Push a commit, then re-run `make setup-prod-postapply` |
| State file corrupted | `include_vars` parse error | Restore from the most recent `.setup-prod.state.yml.<timestamp>~` backup (e.g. `cp "$(ls -1t .setup-prod.state.yml.*~ \| head -1)" .setup-prod.state.yml`) |
| Need to redo a single phase | — | `ansible-playbook playbooks/setup-prod.yml --tags <phase>` (state file carries forward) |

## Topology

```mermaid
flowchart LR
    Vercel["Vercel<br/>(Next.js)"]
    Render["Render (Go BE)<br/>POST /query"]
    Supabase["Supabase<br/>Auth + Postgres"]

    Vercel -- "HTTPS /api/graphql" --> Render
    Vercel -- "OAuth" --> Supabase
    Render -- "Postgres + JWKS" --> Supabase
```

The browser only ever talks to its own Vercel origin. `frontend/next.config.ts` rewrites `/api/graphql` → `${BACKEND_URL}/query`, so requests reach Render through Next's rewrite — there is no CORS layer on the backend (see `docs/frontend.md` § "Backend rewrite contract").

The backend trusts Supabase as the JWT issuer: it fetches the JWKS at boot from `SUPABASE_JWKS_URL` and validates `aud` / `iss` against `SUPABASE_JWT_AUDIENCE` / `SUPABASE_JWT_ISSUER` (see `docs/backend.md` § "Authentication"). Supabase is therefore the single trust anchor between Vercel and Render.

## Manual prerequisites

A few things must be set up out-of-band before standing up the providers.

### 1. Render GitHub App installation

Render needs read access to the repository:

1. `https://dashboard.render.com/select-repo` → **Configure account**.
2. Choose the GitHub account / org owning `yasuflatland-lf/flamingo-armond`.
3. Either grant access to all repos or just this one.
4. Confirm.

If you skip this, creating the Render web service fails with a 4xx because the GitHub App cannot reach the repo.

### 2. Vercel GitHub App installation

Same idea on Vercel:

1. `https://vercel.com/new` → **Import Git Repository**.
2. If `yasuflatland-lf` is not listed, click **Adjust GitHub App Permissions** and grant access.
3. Cancel the import wizard if you only need to install the app — you will trigger the actual import in Step 3 below.

### 3. Google Cloud Console OAuth Client

Supabase delegates Google sign-in to a Google OAuth client. It must be created on the Google Cloud side. This client is **separate from the local-dev OAuth client** in `docs/dev-setup.md` § "Supabase CLI": local credentials live in the root `./.env` (the single hand-edited file; see `README.md` § env layout), production credentials live on the Google client itself, so a leaked local secret never affects production.

1. Open `https://console.cloud.google.com/apis/credentials`. Pick or create a project (this is the Google Cloud project, distinct from the Supabase project).
2. **Configure OAuth consent screen** if not yet done. Type **External**. Set product name and support email.
3. **Create Credentials → OAuth client ID** → Application type **Web application**. Name something like `flamingo-armond-prod`.
4. **Authorized redirect URIs** — add a placeholder for now:

   ```
   https://placeholder.supabase.co/auth/v1/callback
   ```

   The real Supabase project ref is unknown until the Supabase project is created in Step 1 below. After that, replace the placeholder with the actual ref (see "Post-bring-up tasks").
5. **Create**. Copy the Client ID and secret — you will paste them into Supabase Auth in the next section.

## Bring-up procedure

Four steps across the three providers. Allow about 30 minutes total; Supabase project provisioning is the slow leg (~5–15 minutes).

### Step 1 — Supabase

1. Create a new Supabase project (record region and DB password). Region matters for latency — keep it close to the Render region (the local default elsewhere in this repo is `ap-northeast-1`).
2. **Authentication → Sign In / Up**: enable Google OAuth. Paste the Client ID and secret from Manual prerequisites § 3.
3. Capture the values that other providers need:

   | Used as | Value | Where to find it in the dashboard |
   |---|---|---|
   | Frontend `NEXT_PUBLIC_SUPABASE_URL` | `https://<project-ref>.supabase.co` | **Data API** page, or assemble from **General → Project ID**. |
   | Frontend `NEXT_PUBLIC_SUPABASE_ANON_KEY` | Publishable key (preferred) or legacy anon key | **API Keys → Publishable and secret API keys** tab → **Publishable key** `default` row. **Never** copy from **Secret keys** or the legacy `service_role` row. |
   | Backend `SUPABASE_JWKS_URL` | `https://<project-ref>.supabase.co/auth/v1/.well-known/jwks.json` | **JWT Keys → Discovery URL**. |
   | Backend `SUPABASE_JWT_ISSUER` | `https://<project-ref>.supabase.co/auth/v1` | Assemble from project ref. |
   | Backend `SUPABASE_JWT_AUDIENCE` | `authenticated` | Supabase default. |

4. **Connect** (top of any page) → **Session pooler** tab → copy the `postgres://...?sslmode=require` DSN → backend `SUPABASE_DB_URL`. Use session pooler, not transaction pooler — `golang-migrate` issues advisory locks that need a real session.

### Step 2 — Render

Create a web service against `yasuflatland-lf/flamingo-armond`:

| Setting | Value |
|---|---|
| Root directory | `backend` |
| Build command | `go mod download && go build -o main ./cmd/server` |
| Start command | `./main` |
| Health check path | `/health` |
| Auto deploy | **off** — deploys are push-triggered via the deploy hook (see `.github/workflows/backend.yml`), not Render's auto-deploy. Schema migrations run on boot, so we tie deploys to explicit pushes. |

Set the env vars listed below.

| Variable | Value |
|---|---|
| `SUPABASE_DB_URL` | Session-mode pooler DSN from Step 1.4. |
| `SUPABASE_JWKS_URL` | `https://<project-ref>.supabase.co/auth/v1/.well-known/jwks.json` |
| `SUPABASE_JWT_AUDIENCE` | `authenticated` |
| `SUPABASE_JWT_ISSUER` | `https://<project-ref>.supabase.co/auth/v1` |
| `APP_ENV` | `production` |
| `GRAPHQL_INTROSPECTION` | `off` |
| `OTEL_TRACES_SAMPLER_ARG` | `0.1` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP endpoint, or empty for no-op tracing. |

After the service is created, copy the deploy hook URL from **Settings → Deploy Hook** and store it as the GitHub Actions secret `RENDER_DEPLOY_HOOK_URL` (used by `.github/workflows/backend.yml`). When using `make setup-prod`, this registration is automated via `gh secret set` with the value piped through stdin — see the "Two security patterns" subsection above.

### Step 3 — Vercel

Import the repo with **Root Directory: `frontend`**, framework **Next.js**. Add env vars:

| Variable | Scope | Value |
|---|---|---|
| `BACKEND_URL` | Production / Preview | Render service URL from Step 2. |
| `NEXT_PUBLIC_SUPABASE_URL` | All | `https://<project-ref>.supabase.co` |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | All | Publishable key from Step 1.3. |

### Step 4 — Loop back to Supabase

1. **Authentication → URL Configuration** → set **Site URL** to the Vercel production URL and add `<vercel-domain>/auth/callback` to **Redirect URLs**.
2. In Google Cloud Console, replace the redirect URI placeholder from Manual prerequisites § 3 with the real `https://<project-ref>.supabase.co/auth/v1/callback`.

If you skip step 2, Google sign-in completes but redirects to the placeholder URL and fails.

## Post-bring-up tasks

### Trigger the first Render deploy

Auto-deploy is off so schema migrations stay tied to explicit deploys. There are two paths:

- **`make setup-prod` users**: Phase 6 (postapply) already triggered the first deploy via the Render API and polled it to `live`. Skip this section.
- **Manual operators (no `make setup-prod`)**: trigger the first deploy yourself via Render dashboard → service → **Manual Deploy → Deploy latest commit**, or push any commit to `main` (CI fires the deploy hook).

`golang-migrate` runs the schema migrations during the first boot.

### Trigger the first Vercel deploy

Push to `main`, or use the Vercel dashboard's **Deploy** button on the project page.

## Smoke tests after the initial setup

Replace `<backend-url>` and `<frontend-url>` with the URLs printed by Render and Vercel.

```bash
# Backend health
curl "<backend-url>/health"                              # → 200

# Backend GraphQL (auth-free query)
curl -X POST "<backend-url>/query" \
  -H 'content-type: application/json' \
  -d '{"query":"{ health }"}'                            # → {"data":{"health":"ok"}}

# Frontend reachability
curl -I "<frontend-url>"                                 # → 200
```

For an end-to-end check, sign in via Google on the Vercel domain and load `/profile`. The page issues an authenticated `me` query through the rewrite, which exercises the full Vercel → Render → Supabase chain (see `docs/frontend.md` § "Profile page").

## Operational gotchas

- **Migrations run on every Render boot.** A failing migration sets `schema_migrations.dirty=true` and requires manual `migrate force <version>` recovery (`docs/backend.md` § "Migrations").
- **Use `127.0.0.1`, not `localhost`, for any local OAuth setup.** Google's redirect URI validation treats them as distinct origins. This applies to local development only; production uses real domains (`docs/dev-setup.md` § "Gotchas").
- **Render free tier sleeps idle services.** The first request after idleness incurs a cold start. Health checks on `/health` keep the service warm only while traffic flows.
- **Custom domains.** When adding a Vercel custom domain, also update the Supabase Auth Site URL and add the new origin to the Redirect URLs allow list.
