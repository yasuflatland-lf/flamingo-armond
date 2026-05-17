# docs/

L2 architecture and topic docs. Tier conventions: see [`doc-organization.md`](doc-organization.md).

## Before adding or splitting a doc

1. Apply the 5-axis classification in `doc-organization.md` to decide L2 vs L3.
2. Run `wc -l <file>` — soft cap 600 lines per file.
3. Don't split preemptively; see [`.claude/rules/scope-discipline.md`](../.claude/rules/scope-discipline.md).

## When editing existing docs

- Verify cross-doc symbols against the source of truth, not sibling docs ([`.claude/rules/language-policy.md`](../.claude/rules/language-policy.md)).
- Prefer Markdown anchor links over bare-text references — heading-rename rot becomes loud.
- Re-check inbound references after deletion or rename ([`.claude/rules/scope-discipline.md`](../.claude/rules/scope-discipline.md)).
- All committed text is English (`language-policy.md`); chat replies in Japanese.

## Layout

- `docs/frontend/` — frontend topic docs (indexed by `frontend/CLAUDE.md`).
- `docs/backend/` — backend topic docs.
- `docs/pagination/` — pagination-pattern L3 chapter docs.
- Top-level `docs/*.md` — cross-area or area-overview docs.

## Cross-cutting rules already in context

Language policy, scope discipline, PR-update conventions are auto-loaded via `.claude/rules/`. Do not duplicate them here.
