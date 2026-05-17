# Architecture Diagram

Source for the production architecture diagram of this repository. The script
`diagram.py` uses the [diagrams](https://diagrams.mingrammer.com/) Python
library (which renders via [Graphviz](https://graphviz.gitlab.io/)) and emits
`architecture.png` in this directory.

## Prerequisites

- Python 3.6+
- [Graphviz](https://graphviz.gitlab.io/download/)
- [Diagrams](https://diagrams.mingrammer.com/docs/getting-started/installation#quick-start)

On macOS:

```bash
brew install graphviz
pip3 install diagrams
```

## Generate Diagram

From this directory:

```bash
python diagram.py
```

The script writes `architecture.png` next to itself. Re-run after updating
`diagram.py`; the output is regenerable, so do not hand-edit the PNG.

## What the diagram shows

- **Vercel** — Next.js 16 (App Router): RSC pages call `gqlFetch` directly
  against `BACKEND_URL`; client components call `/api/graphql`, which the Next
  rewrite forwards to the backend so the browser stays same-origin (no CORS).
- **Render** — Go / Echo v5 backend: middleware chain (`RequestID` → `JWT`),
  gqlgen resolvers with DataLoader, and the `internal/` layers
  (`usecase` → `repository` over GORM → Postgres). `golang-migrate` runs on
  every boot.
- **Supabase** — Postgres (with RLS, `public` + `auth` schemas) and Auth (JWT
  issuer, JWKS endpoint, Google OAuth broker). Sole trust anchor between
  Vercel and Render — the backend verifies every incoming JWT against the
  Supabase JWKS.
- **schema/schema.graphql** — single source of truth for the wire contract;
  feeds `gqlgen` (backend) and `graphql-codegen` (frontend).
- **Notion** — upstream source for `Cards` content. A scheduled job
  (`/internal/notion-sync`, fired every 6 hours from GitHub Actions) pulls the
  configured Notion page and persists rows into Postgres via the backend. See
  `docs/notion-sync.md` for the env-var matrix and operational runbook.
- **GitHub Actions** — `backend.yml` triggers the Render deploy hook,
  `frontend.yml` is informational (Vercel's native git integration deploys),
  `e2e.yml` runs Playwright, and `readiness-ping.yml` keeps the stack warm
  every 15 minutes by hitting `/internal/ping`.

For the narrative version of the same picture, see `docs/deployment.md`
("Topology"), `docs/backend.md`, `docs/frontend.md`, and `docs/notion-sync.md`.
