# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

## Repo layout

Monorepo for a swiping flashcard app (`🦩 flamingo-armond`):

- `backend/` — Go / Echo v5. The only area wired end-to-end (build, tests, CI, deploy).
- `frontend/` — TypeScript / GraphQL client.
- `schema/` — Shared GraphQL schema. Source of truth for both `gqlgen` (backend) and `codegen` (frontend); outputs land in `backend/graph/{generated,model}/` and `frontend/src/generated/`.

Each top-level area has its own `CLAUDE.md` that auto-loads when working under that directory (Hierarchical CLAUDE.md scheme): [`backend/CLAUDE.md`](backend/CLAUDE.md), [`frontend/CLAUDE.md`](frontend/CLAUDE.md), [`schema/CLAUDE.md`](schema/CLAUDE.md), [`docs/CLAUDE.md`](docs/CLAUDE.md).

## PR Updates

When rewriting a PR title/body, always write the body to a tempfile and pass `gh pr edit <N> --title "..." --body-file <tempfile>`. Never use HEREDOC or inline `--body "..."` for multiline content. Full rule: [`.claude/rules/pr-updates.md`](.claude/rules/pr-updates.md). Backend layer-dependency rules: [`.claude/rules/backend-layering.md`](.claude/rules/backend-layering.md) (CI-enforced via [`go-arch-lint`](backend/.go-arch-lint.yml)).

## Doc tiers

This file is **L1**: keep it ≤35 lines, top-level orientation only. Overflow goes into:

- **L2 — `docs/`** — architecture & topic docs (source of truth for technical detail). Soft cap ~600 lines per file; if a top-level section exceeds that, split by topic.
- **L3 — `.claude/rules/`** — cross-cutting rules and conventions (apply across multiple L2 docs, or to all of `backend/` / `frontend/`).

Prefer updating an L2 or L3 doc over expanding this file. Hierarchical CLAUDE.md: see `<area>/CLAUDE.md` for area orientation. `README.md` § "Further reading" remains as the human-onboarding index. When L1 grows past 35 lines, move the newest section down a tier and leave a one-line pointer here.
